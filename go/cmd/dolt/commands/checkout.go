// Copyright 2019 Dolthub, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package commands

import (
	"context"
	"fmt"
	"github.com/dolthub/dolt/go/cmd/dolt/cli"
	"github.com/dolthub/dolt/go/cmd/dolt/errhand"
	eventsapi "github.com/dolthub/dolt/go/gen/proto/dolt/services/eventsapi/v1alpha1"
	"github.com/dolthub/dolt/go/libraries/doltcore/doltdb"
	"github.com/dolthub/dolt/go/libraries/doltcore/doltdocs"
	"github.com/dolthub/dolt/go/libraries/doltcore/env"
	"github.com/dolthub/dolt/go/libraries/doltcore/env/actions"
	"github.com/dolthub/dolt/go/libraries/doltcore/ref"
	"github.com/dolthub/dolt/go/libraries/utils/argparser"
	"strings"
)

var checkoutDocs = cli.CommandDocumentationContent{
	ShortDesc: `Switch branches or restore working tree tables`,
	LongDesc: `
Updates tables in the working set to match the staged versions. If no paths are given, dolt checkout will also update HEAD to set the specified branch as the current branch.

dolt checkout {{.LessThan}}branch{{.GreaterThan}}
   To prepare for working on {{.LessThan}}branch{{.GreaterThan}}, switch to it by updating the index and the tables in the working tree, and by pointing HEAD at the branch. Local modifications to the tables in the working
   tree are kept, so that they can be committed to the {{.LessThan}}branch{{.GreaterThan}}.

dolt checkout -b {{.LessThan}}new_branch{{.GreaterThan}} [{{.LessThan}}start_point{{.GreaterThan}}]
   Specifying -b causes a new branch to be created as if dolt branch were called and then checked out.

dolt checkout {{.LessThan}}table{{.GreaterThan}}...
  To update table(s) with their values in HEAD `,
	Synopsis: []string{
		`{{.LessThan}}branch{{.GreaterThan}}`,
		`{{.LessThan}}table{{.GreaterThan}}...`,
		`-b {{.LessThan}}new-branch{{.GreaterThan}} [{{.LessThan}}start-point{{.GreaterThan}}]`,
		`-b {{.LessThan}}new-branch{{.GreaterThan}} --track {{.LessThan}}remote{{.GreaterThan}}/{{.LessThan}}branch{{.GreaterThan}}`,
	},
}

type CheckoutCmd struct{}

// Name is returns the name of the Dolt cli command. This is what is used on the command line to invoke the command
func (cmd CheckoutCmd) Name() string {
	return "checkout"
}

// Description returns a description of the command
func (cmd CheckoutCmd) Description() string {
	return "Checkout a branch or overwrite a table from HEAD."
}

func (cmd CheckoutCmd) Docs() *cli.CommandDocumentation {
	ap := cli.CreateCheckoutArgParser()
	return cli.NewCommandDocumentation(checkoutDocs, ap)
}

func (cmd CheckoutCmd) ArgParser() *argparser.ArgParser {
	return cli.CreateCheckoutArgParser()
}

// EventType returns the type of the event to log
func (cmd CheckoutCmd) EventType() eventsapi.ClientEventType {
	return eventsapi.ClientEventType_CHECKOUT
}

// Exec executes the command
func (cmd CheckoutCmd) Exec(ctx context.Context, commandStr string, args []string, dEnv *env.DoltEnv) int {
	ap := cli.CreateCheckoutArgParser()
	helpPrt, usagePrt := cli.HelpAndUsagePrinters(cli.CommandDocsForCommandString(commandStr, checkoutDocs, ap))
	apr := cli.ParseArgsOrDie(ap, args, helpPrt)

	if ((apr.Contains(cli.CheckoutCoBranch) || apr.Contains(cli.TrackFlag)) && apr.NArg() > 1) || (!apr.Contains(cli.CheckoutCoBranch) && apr.NArg() == 0) {
		usagePrt()
		return 1
	}

	if apr.ContainsArg(doltdb.DocTableName) {
		verr := errhand.BuildDError("Use dolt checkout <filename> to check out individual docs.").Build()
		return HandleVErrAndExitCode(verr, usagePrt)
	}

	if apr.Contains(cli.CheckoutCoBranch) || apr.Contains(cli.TrackFlag) {
		verr := checkoutNewBranch(ctx, dEnv, apr)
		return HandleVErrAndExitCode(verr, usagePrt)
	}

	name := apr.Arg(0)
	force := apr.Contains(cli.ForceFlag)

	if len(name) == 0 {
		verr := errhand.BuildDError("error: cannot checkout empty string").Build()
		return HandleVErrAndExitCode(verr, usagePrt)
	}

	if isBranch, err := actions.IsBranch(ctx, dEnv.DoltDB, name); err != nil {
		verr := errhand.BuildDError("error: unable to determine type of checkout").AddCause(err).Build()
		return HandleVErrAndExitCode(verr, usagePrt)
	} else if isBranch {
		verr := checkoutBranch(ctx, dEnv, name, force)
		return HandleVErrAndExitCode(verr, usagePrt)
	}

	// Check if the user executed `dolt checkout .`
	if apr.NArg() == 1 && name == "." {
		roots, err := dEnv.Roots(ctx)
		if err != nil {
			return HandleVErrAndExitCode(errhand.BuildDError(err.Error()).Build(), usagePrt)
		}
		verr := actions.ResetHard(ctx, dEnv, "HEAD", roots)
		return handleResetError(verr, usagePrt)
	}

	tbls, docs, err := actions.GetTablesOrDocs(dEnv.DocsReadWriter(), args)
	if err != nil {
		verr := errhand.BuildDError("error: unable to parse arguments.").AddCause(err).Build()
		return HandleVErrAndExitCode(verr, usagePrt)
	}

	verr := checkoutTablesAndDocs(ctx, dEnv, tbls, docs)

	if verr != nil && apr.NArg() == 1 {
		verr = checkoutRemoteBranchOrSuggestNew(ctx, dEnv, name)
	}

	return HandleVErrAndExitCode(verr, usagePrt)
}

func checkoutRemoteBranchOrSuggestNew(ctx context.Context, dEnv *env.DoltEnv, name string) errhand.VerboseError {
	if ref, refExists, err := actions.GetRemoteBranchRef(ctx, dEnv.DoltDB, name); err != nil {
		return errhand.BuildDError("fatal: unable to read from data repository.").AddCause(err).Build()
	} else if refExists {
		return checkoutNewBranchFromStartPt(ctx, dEnv, name, ref.String())
	} else {
		// Check if the user is trying to enter a detached head state
		commit, _ := actions.MaybeGetCommit(ctx, dEnv, name)
		if commit != nil {
			// User tried to enter a detached head state, which we don't support.
			// Inform and suggest that they check-out a new branch at this commit instead.

			str := "dolt does not support a detached head state. To create a branch at this commit instead, run:\n\n" +
				"\tdolt checkout %s -b {new_branch_name}\n"

			return errhand.BuildDError(str, name).Build()
		}
		return errhand.BuildDError("error: could not find %s", name).Build()
	}
}

func checkoutNewBranchFromStartPt(ctx context.Context, dEnv *env.DoltEnv, newBranch, startPt string) errhand.VerboseError {
	err := actions.CreateBranchWithStartPt(ctx, dEnv.DbData(), newBranch, startPt, false)

	if err != nil {
		return errhand.BuildDError(err.Error()).Build()
	}

	return checkoutBranch(ctx, dEnv, newBranch, false)
}

func checkoutNewBranch(ctx context.Context, dEnv *env.DoltEnv, apr *argparser.ArgParseResults) errhand.VerboseError {
	var remote env.Remote
	var err error
	var newBranchName string
	var startPt = "head"

	if apr.NArg() == 1 {
		startPt = apr.Arg(0)
	}

	trackVal, setTrack := apr.GetValue(cli.TrackFlag)
	if setTrack {
		if trackVal != "direct" && trackVal != "inherit" {
			startPt = trackVal
		} else if trackVal == "inherit" {
			return errhand.VerboseErrorFromError(fmt.Errorf("--track='inherit' is not supported yet"))
		}
	}

	if newBranch, ok := apr.GetValue(cli.CheckoutCoBranch); ok {
		if len(newBranch) == 0 {
			return errhand.BuildDError("error: cannot checkout empty string").Build()
		}
		newBranchName = newBranch
	} else {
		remote, newBranchName, err = getRemoteTrackingBranch(ctx, dEnv, startPt)
		if err != nil {
			return errhand.BuildDError(err.Error()).Build()
		}
	}

	err = actions.CreateBranchWithStartPt(ctx, dEnv.DbData(), newBranchName, startPt, false)
	if err != nil {
		return errhand.BuildDError(err.Error()).Build()
	}

	verr := checkoutBranch(ctx, dEnv, newBranchName, false)
	if verr != nil {
		return verr
	}

	if setTrack {
		// the new branch is checked out at this point
		err = setRemoteTrackingForABranch(ctx, dEnv, remote, newBranchName, startPt)
		if err != nil {
			return errhand.BuildDError(err.Error()).Build()
		}
		err = dEnv.RepoState.Save(dEnv.FS)
		if err != nil {
			err = fmt.Errorf("%w; %s", actions.ErrFailedToSaveRepoState, err.Error())
		}
	}

	return nil
}

func getRemoteTrackingBranch(ctx context.Context, dEnv *env.DoltEnv, startPt string) (env.Remote, string, error) {
	remotes, err := dEnv.RepoStateReader().GetRemotes()
	if err != nil {
		return env.Remote{}, "", err
	}

	var remote env.Remote
	refSpecStr := startPt
	names := strings.Split(startPt, "/")
	if r, remoteOK := remotes[names[0]]; remoteOK {
		remote = r
		refSpecStr = strings.Join(names[1:], "/")
	} else if r, remoteOK = remotes["origin"]; remoteOK {
		remote = r
	} else {
		return env.Remote{}, "", fmt.Errorf("invalid remote-tracking path")
	}

	refSpecStr, err = env.DisambiguateRefSpecStr(ctx, dEnv.DoltDB, refSpecStr)
	if err != nil {
		return env.Remote{}, "", err
	}

	return remote, refSpecStr, nil
}

func setRemoteTrackingForABranch(ctx context.Context, dEnv *env.DoltEnv, remote env.Remote, branchName, refSpecStr string) error {
	refSpec, err := ref.ParseRefSpec(refSpecStr)
	if err != nil {
		return fmt.Errorf("%w: '%s'", err, refSpecStr)
	}

	// TODO : get ref.DoltRef for branchName
	branch := dEnv.RepoStateReader().CWBHeadRef()
	hasRef, err := dEnv.DoltDB.HasRef(ctx, branch)
	if err != nil {
		return err
	}
	if !hasRef {
		return doltdb.ErrBranchNotFound
	}

	src := refSpec.SrcRef(branch)
	dest := refSpec.DestRef(src)

	if branch.GetPath() != dest.GetPath() {
		return env.ErrBranchDoesNotMatchUpstream
	}

	return dEnv.RepoStateWriter().UpdateBranch(branch.GetPath(), env.BranchConfig{
		Merge: ref.MarshalableRef{
			Ref: dest,
		},
		Remote: remote.Name,
	})
}

func checkoutTablesAndDocs(ctx context.Context, dEnv *env.DoltEnv, tables []string, docs doltdocs.Docs) errhand.VerboseError {
	roots, err := dEnv.Roots(ctx)
	if err != nil {
		return errhand.VerboseErrorFromError(err)
	}

	err = actions.CheckoutTablesAndDocs(ctx, roots, dEnv.DbData(), tables, docs)

	if err != nil {
		if doltdb.IsRootValUnreachable(err) {
			return unreadableRootToVErr(err)
		} else if actions.IsTblNotExist(err) {
			badTbls := actions.GetTablesForError(err)
			bdr := errhand.BuildDError("")
			for _, tbl := range badTbls {
				bdr.AddDetails("error: table '%s' did not match any table(s) known to dolt.", tbl)
			}
			return bdr.Build()
		} else {
			bdr := errhand.BuildDError("fatal: Unexpected error checking out tables")
			bdr.AddCause(err)
			return bdr.Build()
		}
	}

	return nil
}

func checkoutBranch(ctx context.Context, dEnv *env.DoltEnv, name string, force bool) errhand.VerboseError {
	err := actions.CheckoutBranch(ctx, dEnv, name, force)

	if err != nil {
		if err == doltdb.ErrBranchNotFound {
			return errhand.BuildDError("fatal: Branch '%s' not found.", name).Build()
		} else if doltdb.IsRootValUnreachable(err) {
			return unreadableRootToVErr(err)
		} else if actions.IsCheckoutWouldOverwrite(err) {
			tbls := actions.CheckoutWouldOverwriteTables(err)
			bdr := errhand.BuildDError("error: Your local changes to the following tables would be overwritten by checkout:")
			for _, tbl := range tbls {
				bdr.AddDetails("\t" + tbl)
			}

			bdr.AddDetails("Please commit your changes or stash them before you switch branches.")
			bdr.AddDetails("Aborting")
			return bdr.Build()
		} else if err == doltdb.ErrAlreadyOnBranch {
			// Being on the same branch shouldn't be an error
			cli.Printf("Already on branch '%s'", name)
			return nil
		} else {
			bdr := errhand.BuildDError("fatal: Unexpected error checking out branch '%s'", name)
			bdr.AddCause(err)
			return bdr.Build()
		}
	}

	cli.Printf("Switched to branch '%s'\n", name)

	return nil
}

func unreadableRootToVErr(err error) errhand.VerboseError {
	rt := doltdb.GetUnreachableRootType(err)
	bdr := errhand.BuildDError("error: unable to read the %s", rt.String())
	return bdr.AddCause(doltdb.GetUnreachableRootCause(err)).Build()
}
