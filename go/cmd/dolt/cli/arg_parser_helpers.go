// Copyright 2020 Dolthub, Inc.
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

package cli

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/dolthub/dolt/go/libraries/doltcore/dbfactory"
	"github.com/dolthub/dolt/go/libraries/utils/argparser"
)

const VerboseFlag = "verbose"

// we are more permissive than what is documented.
var SupportedLayouts = []string{
	"2006/01/02",
	"2006/01/02T15:04:05",
	"2006/01/02T15:04:05Z07:00",

	"2006.01.02",
	"2006.01.02T15:04:05",
	"2006.01.02T15:04:05Z07:00",

	"2006-01-02",
	"2006-01-02T15:04:05",
	"2006-01-02T15:04:05Z07:00",
}

// Parses a date string. Used by multiple commands.
func ParseDate(dateStr string) (time.Time, error) {
	for _, layout := range SupportedLayouts {
		t, err := time.Parse(layout, dateStr)

		if err == nil {
			return t, nil
		}
	}

	return time.Time{}, errors.New("error: '" + dateStr + "' is not in a supported format.")
}

// Parses the author flag for the commit method.
func ParseAuthor(authorStr string) (string, string, error) {
	if len(authorStr) == 0 {
		return "", "", errors.New("Option 'author' requires a value")
	}

	reg := regexp.MustCompile("(?m)([^)]+) \\<([^)]+)") // Regex matches Name <email
	matches := reg.FindStringSubmatch(authorStr)        // This function places the original string at the beginning of matches

	// If name and email are provided
	if len(matches) != 3 {
		return "", "", errors.New("Author not formatted correctly. Use 'Name <author@example.com>' format")
	}

	name := matches[1]
	email := strings.ReplaceAll(matches[2], ">", "")

	return name, email, nil
}

const (
	AllowEmptyFlag   = "allow-empty"
	DateParam        = "date"
	MessageArg       = "message"
	AuthorParam      = "author"
	ForceFlag        = "force"
	DryRunFlag       = "dry-run"
	SetUpstreamFlag  = "set-upstream"
	AllFlag          = "all"
	HardResetParam   = "hard"
	SoftResetParam   = "soft"
	CheckoutCoBranch = "b"
	NoFFParam        = "no-ff"
	SquashParam      = "squash"
	AbortParam       = "abort"
	CopyFlag         = "copy"
	MoveFlag         = "move"
	DeleteFlag       = "delete"
	DeleteForceFlag  = "D"
	OutputOnlyFlag   = "output-only"
	RemoteParam      = "remote"
	BranchParam      = "branch"
	TrackFlag        = "track"
)

const (
	SyncBackupId        = "sync"
	SyncBackupUrlId     = "sync-url"
	RestoreBackupId     = "restore"
	AddBackupId         = "add"
	RemoveBackupId      = "remove"
	RemoveBackupShortId = "rm"
)

var mergeAbortDetails = `Abort the current conflict resolution process, and try to reconstruct the pre-merge state.

If there were uncommitted working set changes present when the merge started, {{.EmphasisLeft}}dolt merge --abort{{.EmphasisRight}} will be unable to reconstruct these changes. It is therefore recommended to always commit or stash your changes before running dolt merge.
`

// Creates the argparser shared dolt commit cli and DOLT_COMMIT.
func CreateCommitArgParser() *argparser.ArgParser {
	ap := argparser.NewArgParser()
	ap.SupportsString(MessageArg, "m", "msg", "Use the given {{.LessThan}}msg{{.GreaterThan}} as the commit message.")
	ap.SupportsFlag(AllowEmptyFlag, "", "Allow recording a commit that has the exact same data as its sole parent. This is usually a mistake, so it is disabled by default. This option bypasses that safety.")
	ap.SupportsString(DateParam, "", "date", "Specify the date used in the commit. If not specified the current system time is used.")
	ap.SupportsFlag(ForceFlag, "f", "Ignores any foreign key warnings and proceeds with the commit.")
	ap.SupportsString(AuthorParam, "", "author", "Specify an explicit author using the standard A U Thor {{.LessThan}}author@example.com{{.GreaterThan}} format.")
	ap.SupportsFlag(AllFlag, "a", "Adds all edited files in working to staged.")
	return ap
}

func CreateMergeArgParser() *argparser.ArgParser {
	ap := argparser.NewArgParser()
	ap.SupportsFlag(NoFFParam, "", "Create a merge commit even when the merge resolves as a fast-forward.")
	ap.SupportsFlag(SquashParam, "", "Merges changes to the working set without updating the commit history")
	ap.SupportsString(MessageArg, "m", "msg", "Use the given {{.LessThan}}msg{{.GreaterThan}} as the commit message.")
	ap.SupportsFlag(AbortParam, "", mergeAbortDetails)
	return ap
}

func CreatePushArgParser() *argparser.ArgParser {
	ap := argparser.NewArgParser()
	ap.SupportsFlag(SetUpstreamFlag, "u", "For every branch that is up to date or successfully pushed, add upstream (tracking) reference, used by argument-less {{.EmphasisLeft}}dolt pull{{.EmphasisRight}} and other commands.")
	ap.SupportsFlag(ForceFlag, "f", "Update the remote with local history, overwriting any conflicting history in the remote.")
	return ap
}

func CreateAddArgParser() *argparser.ArgParser {
	ap := argparser.NewArgParser()
	ap.ArgListHelp = append(ap.ArgListHelp, [2]string{"table", "Working table(s) to add to the list tables staged to be committed. The abbreviation '.' can be used to add all tables."})
	ap.SupportsFlag("all", "A", "Stages any and all changes (adds, deletes, and modifications).")
	return ap
}

func CreateCloneArgParser() *argparser.ArgParser {
	ap := argparser.NewArgParser()
	ap.SupportsString(RemoteParam, "", "name", "Name of the remote to be added to the cloned database. The default is 'origin'.")
	ap.SupportsString(BranchParam, "b", "branch", "The branch to be cloned. If not specified all branches will be cloned.")
	ap.SupportsString(dbfactory.AWSRegionParam, "", "region", "")
	ap.SupportsValidatedString(dbfactory.AWSCredsTypeParam, "", "creds-type", "", argparser.ValidatorFromStrList(dbfactory.AWSCredsTypeParam, dbfactory.AWSCredTypes))
	ap.SupportsString(dbfactory.AWSCredsFileParam, "", "file", "AWS credentials file.")
	ap.SupportsString(dbfactory.AWSCredsProfile, "", "profile", "AWS profile to use.")
	return ap
}

func CreateResetArgParser() *argparser.ArgParser {
	ap := argparser.NewArgParser()
	ap.SupportsFlag(HardResetParam, "", "Resets the working tables and staged tables. Any changes to tracked tables in the working tree since {{.LessThan}}commit{{.GreaterThan}} are discarded.")
	ap.SupportsFlag(SoftResetParam, "", "Does not touch the working tables, but removes all tables staged to be committed.")
	return ap
}

func CreateRemoteArgParser() *argparser.ArgParser {
	ap := argparser.NewArgParser()
	ap.SupportsString(dbfactory.AWSRegionParam, "", "region", "")
	ap.SupportsValidatedString(dbfactory.AWSCredsTypeParam, "", "creds-type", "", argparser.ValidatorFromStrList(dbfactory.AWSCredsTypeParam, dbfactory.AWSCredTypes))
	ap.SupportsString(dbfactory.AWSCredsFileParam, "", "file", "AWS credentials file")
	ap.SupportsString(dbfactory.AWSCredsProfile, "", "profile", "AWS profile to use")
	return ap
}

func CreateCleanArgParser() *argparser.ArgParser {
	ap := argparser.NewArgParser()
	ap.SupportsFlag(DryRunFlag, "", "Tests removing untracked tables without modifying the working set.")
	return ap
}

func CreateCheckoutArgParser() *argparser.ArgParser {
	ap := argparser.NewArgParser()
	ap.SupportsString(CheckoutCoBranch, "", "branch", "Create a new branch named {{.LessThan}}new_branch{{.GreaterThan}} and start it at {{.LessThan}}start_point{{.GreaterThan}}.")
	ap.SupportsFlag(ForceFlag, "f", "If there is any changes in working set, the force flag will wipe out the current changes and checkout the new branch.")
	ap.SupportsString(TrackFlag, "t", "", "When creating a new branch, set up 'upstream' configuration.")
	return ap
}

func CreateCherryPickArgParser() *argparser.ArgParser {
	ap := argparser.NewArgParser()
	return ap
}

func CreateFetchArgParser() *argparser.ArgParser {
	ap := argparser.NewArgParser()
	ap.SupportsFlag(ForceFlag, "f", "Update refs to remote branches with the current state of the remote, overwriting any conflicting history.")
	return ap
}

func CreateRevertArgParser() *argparser.ArgParser {
	ap := argparser.NewArgParser()
	ap.SupportsString(AuthorParam, "", "author", "Specify an explicit author using the standard A U Thor {{.LessThan}}author@example.com{{.GreaterThan}} format.")
	ap.ArgListHelp = append(ap.ArgListHelp, [2]string{"revision",
		"The commit revisions. If multiple revisions are given, they're applied in the order given."})

	return ap
}

func CreatePullArgParser() *argparser.ArgParser {
	ap := argparser.NewArgParser()
	ap.SupportsFlag(SquashParam, "", "Merges changes to the working set without updating the commit history")
	ap.SupportsFlag(NoFFParam, "", "Create a merge commit even when the merge resolves as a fast-forward.")
	ap.SupportsFlag(ForceFlag, "f", "Ignores any foreign key warnings and proceeds with the commit.")

	return ap
}

func CreateBranchArgParser() *argparser.ArgParser {
	ap := argparser.NewArgParser()
	ap.SupportsFlag(ForceFlag, "f", "Ignores any foreign key warnings and proceeds with the commit.")
	ap.SupportsFlag(CopyFlag, "c", "Create a copy of a branch.")
	ap.SupportsFlag(MoveFlag, "m", "Move/rename a branch")
	ap.SupportsFlag(DeleteFlag, "d", "Delete a branch. The branch must be fully merged in its upstream branch.")
	ap.SupportsFlag(DeleteForceFlag, "", "Shortcut for {{.EmphasisLeft}}--delete --force{{.EmphasisRight}}.")

	return ap
}

func CreateTagArgParser() *argparser.ArgParser {
	ap := argparser.NewArgParser()
	ap.ArgListHelp = append(ap.ArgListHelp, [2]string{"ref", "A commit ref that the tag should point at."})
	ap.SupportsString(MessageArg, "m", "msg", "Use the given {{.LessThan}}msg{{.GreaterThan}} as the tag message.")
	ap.SupportsFlag(VerboseFlag, "v", "list tags along with their metadata.")
	ap.SupportsFlag(DeleteFlag, "d", "Delete a tag.")
	return ap
}

func CreateBackupArgParser() *argparser.ArgParser {
	ap := argparser.NewArgParser()
	ap.ArgListHelp = append(ap.ArgListHelp, [2]string{"region", "cloud provider region associated with this backup."})
	ap.ArgListHelp = append(ap.ArgListHelp, [2]string{"creds-type", "credential type.  Valid options are role, env, and file.  See the help section for additional details."})
	ap.ArgListHelp = append(ap.ArgListHelp, [2]string{"profile", "AWS profile to use."})
	ap.SupportsFlag(VerboseFlag, "v", "When printing the list of backups adds additional details.")
	ap.SupportsString(dbfactory.AWSRegionParam, "", "region", "")
	ap.SupportsValidatedString(dbfactory.AWSCredsTypeParam, "", "creds-type", "", argparser.ValidatorFromStrList(dbfactory.AWSCredsTypeParam, dbfactory.AWSCredTypes))
	ap.SupportsString(dbfactory.AWSCredsFileParam, "", "file", "AWS credentials file")
	ap.SupportsString(dbfactory.AWSCredsProfile, "", "profile", "AWS profile to use")
	return ap
}

func CreateVerifyConstraintsArgParser() *argparser.ArgParser {
	ap := argparser.NewArgParser()
	ap.SupportsFlag(AllFlag, "a", "Verifies that all rows in the database do not violate constraints instead of just rows modified or inserted in the working set.")
	ap.SupportsFlag(OutputOnlyFlag, "o", "Disables writing violated constraints to the constraint violations table.")
	ap.ArgListHelp = append(ap.ArgListHelp, [2]string{"table", "The table(s) to check constraints on. If omitted, checks all tables."})
	return ap
}

var awsParams = []string{dbfactory.AWSRegionParam, dbfactory.AWSCredsTypeParam, dbfactory.AWSCredsFileParam, dbfactory.AWSCredsProfile}

func ProcessBackupArgs(apr *argparser.ArgParseResults, scheme, backupUrl string) (map[string]string, error) {
	params := map[string]string{}

	var err error
	if scheme == dbfactory.AWSScheme {
		err = AddAWSParams(backupUrl, apr, params)
	} else {
		err = VerifyNoAwsParams(apr)
	}

	return params, err
}

func AddAWSParams(remoteUrl string, apr *argparser.ArgParseResults, params map[string]string) error {
	isAWS := strings.HasPrefix(remoteUrl, "aws")

	if !isAWS {
		for _, p := range awsParams {
			if _, ok := apr.GetValue(p); ok {
				return fmt.Errorf("%s param is only valid for aws cloud remotes in the format aws://dynamo-table:s3-bucket/database", p)
			}
		}
	}

	for _, p := range awsParams {
		if val, ok := apr.GetValue(p); ok {
			params[p] = val
		}
	}

	return nil
}

func VerifyNoAwsParams(apr *argparser.ArgParseResults) error {
	if awsParams := apr.GetValues(awsParams...); len(awsParams) > 0 {
		awsParamKeys := make([]string, 0, len(awsParams))
		for k := range awsParams {
			awsParamKeys = append(awsParamKeys, k)
		}

		keysStr := strings.Join(awsParamKeys, ",")
		return fmt.Errorf("The parameters %s, are only valid for aws remotes", keysStr)
	}

	return nil
}
