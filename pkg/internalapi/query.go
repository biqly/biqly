package internalapi

import (
	"github.com/biqly/biqly/pkg/logicalquery"
	pkgquery "github.com/biqly/biqly/pkg/query"
	"github.com/biqly/biqly/pkg/semantic"
)

// CompileRequest is the body of POST /internal/query/compile.
//
// The caller submits a fully-formed LogicalQuery; the service compiles it
// against the published semantic model identified by LogicalQuery.ModelID
// and the datasource identified by LogicalQuery.DatasourceID.
type CompileRequest struct {
	LogicalQuery logicalquery.LogicalQuery `json:"logical_query"`
	// Model optionally inlines the semantic model to compile against. Callers
	// set it when the model is synthetic (auto table routing produces models
	// with an "auto:" ID prefix that exist only in the caller's memory) and
	// therefore cannot be loaded from the catalog via LogicalQuery.ModelID.
	// Leave nil for published models.
	Model *semantic.SemanticModel `json:"model,omitempty"`
	// DryRun toggles EXPLAIN-only compilation. When true, the service MAY
	// run the result through the dialect's EXPLAIN wrapper without executing.
	DryRun bool `json:"dry_run,omitempty"`
}

// CompileResponse is the body of POST /internal/query/compile.
type CompileResponse struct {
	SQL         string `json:"sql"`
	Args        []any  `json:"args,omitempty"`
	Fingerprint string `json:"fingerprint"`
	// Warnings are non-fatal compilation messages (e.g. fanout risk on a
	// many_to_many join). Empty when the query compiled cleanly.
	Warnings []string `json:"warnings,omitempty"`
}

// RunRequest is the body of POST /internal/query/run.
type RunRequest struct {
	LogicalQuery logicalquery.LogicalQuery `json:"logical_query"`
	// Model optionally inlines a synthetic semantic model; see CompileRequest.Model.
	Model *semantic.SemanticModel `json:"model,omitempty"`
	// MaxRows overrides BI_QUERY_MAX_ROWS for this single request when > 0.
	// Servers SHOULD cap the value at the global BI_QUERY_MAX_ROWS to prevent
	// callers from raising the ceiling.
	MaxRows int `json:"max_rows,omitempty"`
	// TimeoutMs overrides BI_QUERY_TIMEOUT_SECONDS in milliseconds when > 0.
	// Servers SHOULD cap at the global timeout for the same reason as MaxRows.
	TimeoutMs int `json:"timeout_ms,omitempty"`
}

// RunResponse is the body of POST /internal/query/run.
type RunResponse struct {
	Columns     []pkgquery.ResultColumn `json:"columns"`
	Rows        [][]any                 `json:"rows"`
	RowCount    int                     `json:"row_count"`
	DurationMs  int64                   `json:"duration_ms"`
	Fingerprint string                  `json:"fingerprint"`
	SQL         string                  `json:"sql,omitempty"`
	// Truncated is true when the executor stopped at MaxRows.
	Truncated bool `json:"truncated,omitempty"`
}

// DryRunRequest is the body of POST /internal/query/dry-run.
//
// Servers MUST NOT execute the query against the user database; they only
// compile and (optionally) wrap with EXPLAIN to validate syntax.
type DryRunRequest struct {
	LogicalQuery logicalquery.LogicalQuery `json:"logical_query"`
	// Model optionally inlines a synthetic semantic model; see CompileRequest.Model.
	Model *semantic.SemanticModel `json:"model,omitempty"`
}

// DryRunResponse is the body of POST /internal/query/dry-run.
type DryRunResponse struct {
	SQL         string   `json:"sql"`
	Args        []any    `json:"args,omitempty"`
	Fingerprint string   `json:"fingerprint"`
	Warnings    []string `json:"warnings,omitempty"`
}
