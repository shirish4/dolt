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

package doltdb

import (
	"context"
	"errors"
	"sort"
	"strings"

	"github.com/dolthub/dolt/go/libraries/doltcore/schema"
	"github.com/dolthub/dolt/go/libraries/utils/funcitr"
	"github.com/dolthub/dolt/go/libraries/utils/set"
	"github.com/dolthub/dolt/go/store/types"
)

const (
	// DoltNamespace is the name prefix of dolt system tables. We reserve all tables that begin with dolt_ for system use.
	DoltNamespace = "dolt"
)

var ErrSystemTableCannotBeModified = errors.New("system tables cannot be dropped or altered")

// HasDoltPrefix returns a boolean whether or not the provided string is prefixed with the DoltNamespace. Users should
// not be able to create tables in this reserved namespace.
func HasDoltPrefix(s string) bool {
	return strings.HasPrefix(strings.ToLower(s), DoltNamespace)
}

// IsReadOnlySystemTable returns whether the table name given is a system table that should not be included in command line
// output (e.g. dolt status) by default.
func IsReadOnlySystemTable(name string) bool {
	return HasDoltPrefix(name) && !set.NewStrSet(writeableSystemTables).Contains(name)
}

// GetNonSystemTableNames gets non-system table names
func GetNonSystemTableNames(ctx context.Context, root *RootValue) ([]string, error) {
	tn, err := root.GetTableNames(ctx)
	if err != nil {
		return nil, err
	}
	tn = funcitr.FilterStrings(tn, func(n string) bool {
		return !HasDoltPrefix(n)
	})
	sort.Strings(tn)
	return tn, nil
}

// GetSystemTableNames gets system table names
func GetSystemTableNames(ctx context.Context, root *RootValue) ([]string, error) {
	p, err := GetPersistedSystemTables(ctx, root)
	if err != nil {
		return nil, err
	}

	g, err := GetGeneratedSystemTables(ctx, root)
	if err != nil {

	}

	s := append(p, g...)
	sort.Strings(s)

	return s, nil
}

// GetPersistedSystemTables returns table names of all persisted system tables.
func GetPersistedSystemTables(ctx context.Context, root *RootValue) ([]string, error) {
	tn, err := root.GetTableNames(ctx)
	if err != nil {
		return nil, err
	}
	sort.Strings(tn)
	return funcitr.FilterStrings(tn, HasDoltPrefix), nil
}

// GetGeneratedSystemTables returns table names of all generated system tables.
func GetGeneratedSystemTables(ctx context.Context, root *RootValue) ([]string, error) {
	s := set.NewStrSet(generatedSystemTables)

	tn, err := root.GetTableNames(ctx)
	if err != nil {
		return nil, err
	}

	for _, pre := range generatedSystemTablePrefixes {
		s.Add(funcitr.MapStrings(tn, func(s string) string { return pre + s })...)
	}

	return s.AsSlice(), nil
}

// GetAllTableNames returns table names for all persisted and generated tables.
func GetAllTableNames(ctx context.Context, root *RootValue) ([]string, error) {
	n, err := GetNonSystemTableNames(ctx, root)
	if err != nil {
		return nil, err
	}
	s, err := GetSystemTableNames(ctx, root)
	if err != nil {
		return nil, err
	}
	return append(n, s...), nil
}

func MaybeCreateDoltDocsTable(ctx context.Context, root *RootValue) (*RootValue, error) {
	_, ok, err := root.GetTable(ctx, DocTableName)
	if err != nil {
		return nil, err
	}
	if ok {
		return root, nil
	}
	return root.CreateEmptyTable(ctx, DocTableName, DocsSchema)
}

// The set of reserved dolt_ tables that should be considered part of user space, like any other user-created table,
// for the purposes of the dolt command line. These tables cannot be created or altered explicitly, but can be updated
// like normal SQL tables.
var writeableSystemTables = []string{
	DoltQueryCatalogTableName,
	SchemasTableName,
	ProceduresTableName,
	DocTableName,
}

var persistedSystemTables = []string{
	DocTableName,
	DoltQueryCatalogTableName,
	SchemasTableName,
	ProceduresTableName,
}

var generatedSystemTables = []string{
	BranchesTableName,
	LogTableName,
	TableOfTablesInConflictName,
	TableOfTablesWithViolationsName,
	CommitsTableName,
	CommitAncestorsTableName,
	StatusTableName,
	RemotesTableName,
}

var generatedSystemViewPrefixes = []string{
	DoltBlameViewPrefix,
}

var generatedSystemTablePrefixes = []string{
	DoltDiffTablePrefix,
	DoltCommitDiffTablePrefix,
	DoltHistoryTablePrefix,
	DoltConfTablePrefix,
	DoltConstViolTablePrefix,
}

const (
	// LicenseDoc is the key for accessing the license within the docs table
	LicenseDoc = "LICENSE.md"
	// ReadmeDoc is the key for accessing the readme within the docs table
	ReadmeDoc = "README.md"
)

// todo(andy)
var doltDocsColumns = schema.NewColCollection(
	schema.NewColumn(DocPkColumnName, schema.DocNameTag, types.StringKind, true, schema.NotNullConstraint{}),
	schema.NewColumn(DocTextColumnName, schema.DocTextTag, types.StringKind, false),
)
var DocsSchema = schema.MustSchemaFromCols(doltDocsColumns)

const (
	// DocTableName is the name of the dolt table containing documents such as the license and readme
	DocTableName = "dolt_docs"
	// DocPkColumnName is the name of the pk column in the docs table
	DocPkColumnName = "doc_name"
	//DocTextColumnName is the name of the column containing the document contents in the docs table
	DocTextColumnName = "doc_text"
)

const (
	// DoltQueryCatalogTableName is the name of the query catalog table
	DoltQueryCatalogTableName = "dolt_query_catalog"

	// QueryCatalogIdCol is the name of the primary key column of the query catalog table
	QueryCatalogIdCol = "id"

	// QueryCatalogOrderCol is the column containing the order of the queries in the catalog
	QueryCatalogOrderCol = "display_order"

	// QueryCatalogNameCol is the name of the column containing the name of a query in the catalog
	QueryCatalogNameCol = "name"

	// QueryCatalogQueryCol is the name of the column containing the query of a catalog entry
	QueryCatalogQueryCol = "query"

	// QueryCatalogDescriptionCol is the name of the column containing the description of a query in the catalog
	QueryCatalogDescriptionCol = "description"
)

const (
	// SchemasTableName is the name of the dolt schema fragment table
	SchemasTableName = "dolt_schemas"
	// SchemasTablesIdCol is an incrementing integer that represents the insertion index.
	SchemasTablesIdCol = "id"
	// Currently: `view` or `trigger`.
	SchemasTablesTypeCol = "type"
	// The name of the database entity.
	SchemasTablesNameCol = "name"
	// The schema fragment associated with the database entity.
	// For example, the SELECT statement for a CREATE VIEW.
	SchemasTablesFragmentCol = "fragment"
	// The extra information for schema; currently contains creation time for triggers and views
	SchemasTablesExtraCol = "extra"
	// The name of the index that is on the table.
	SchemasTablesIndexName = "fragment_name"
)

const (
	// DoltBlameViewPrefix is the prefix assigned to all the generated blame tables
	DoltBlameViewPrefix = "dolt_blame_"
	// DoltHistoryTablePrefix is the prefix assigned to all the generated history tables
	DoltHistoryTablePrefix = "dolt_history_"
	// DoltDiffTablePrefix is the prefix assigned to all the generated diff tables
	DoltDiffTablePrefix = "dolt_diff_"
	// DoltCommitDiffTablePrefix is the prefix assigned to all the generated commit diff tables
	DoltCommitDiffTablePrefix = "dolt_commit_diff_"
	// DoltConfTablePrefix is the prefix assigned to all the generated conflict tables
	DoltConfTablePrefix = "dolt_conflicts_"
	// DoltConstViolTablePrefix is the prefix assigned to all the generated constraint violation tables
	DoltConstViolTablePrefix = "dolt_constraint_violations_"
)

const (
	// LogTableName is the log system table name
	LogTableName = "dolt_log"

	// DiffTableName is the name of the table with a map of commits to tables changed
	DiffTableName = "dolt_diff"

	// TableOfTablesInConflictName is the conflicts system table name
	TableOfTablesInConflictName = "dolt_conflicts"

	// TableOfTablesWithViolationsName is the constraint violations system table name
	TableOfTablesWithViolationsName = "dolt_constraint_violations"

	// BranchesTableName is the branches system table name
	BranchesTableName = "dolt_branches"

	// RemotesTableName is the remotes system table name
	RemotesTableName = "dolt_remotes"

	// CommitsTableName is the commits system table name
	CommitsTableName = "dolt_commits"

	// CommitAncestorsTableName is the commit_ancestors system table name
	CommitAncestorsTableName = "dolt_commit_ancestors"

	// StatusTableName is the status system table name.
	StatusTableName = "dolt_status"

	// TagsTableName is the tags table name
	TagsTableName = "dolt_tags"
)

const (
	// ProceduresTableName is the name of the dolt stored procedures table.
	ProceduresTableName = "dolt_procedures"
	// ProceduresTableNameCol is the name of the stored procedure. Using CREATE PROCEDURE, will always be lowercase.
	ProceduresTableNameCol = "name"
	// ProceduresTableCreateStmtCol is the CREATE PROCEDURE statement for this stored procedure.
	ProceduresTableCreateStmtCol = "create_stmt"
	// ProceduresTableCreatedAtCol is the time that the stored procedure was created at, in UTC.
	ProceduresTableCreatedAtCol = "created_at"
	// ProceduresTableModifiedAtCol is the time that the stored procedure was last modified, in UTC.
	ProceduresTableModifiedAtCol = "modified_at"
)
