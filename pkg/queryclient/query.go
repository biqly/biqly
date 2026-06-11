package queryclient

import (
	"context"

	"github.com/biqly/biqly/pkg/internalapi"
	"github.com/biqly/biqly/pkg/logicalquery"
	"github.com/biqly/biqly/pkg/semantic"
)

// Compile validates a LogicalQuery and returns the parameterized SQL plus the
// canonical fingerprint. It does NOT execute the query.
func (c *Client) Compile(ctx context.Context, lq *logicalquery.LogicalQuery) (*internalapi.CompileResponse, error) {
	return c.CompileWithModel(ctx, lq, nil)
}

// CompileWithModel is Compile with an optional inline semantic model. Pass a
// non-nil model only when it is synthetic (auto table routing) and cannot be
// loaded from the catalog via LogicalQuery.ModelID; pass nil otherwise.
func (c *Client) CompileWithModel(ctx context.Context, lq *logicalquery.LogicalQuery, model *semantic.SemanticModel) (*internalapi.CompileResponse, error) {
	req := internalapi.CompileRequest{LogicalQuery: *lq, Model: model}
	var resp internalapi.CompileResponse
	if err := c.do(ctx, "/compile", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Run validates, compiles and executes a LogicalQuery. The optional
// per-request maxRows and timeoutMs are advisory; the server caps them at
// BI_QUERY_MAX_ROWS and BI_QUERY_TIMEOUT_SECONDS respectively.
//
// Pass 0 for either to use the server default.
func (c *Client) Run(ctx context.Context, lq *logicalquery.LogicalQuery, maxRows, timeoutMs int) (*internalapi.RunResponse, error) {
	return c.RunWithModel(ctx, lq, nil, maxRows, timeoutMs)
}

// RunWithModel is Run with an optional inline semantic model; see
// CompileWithModel for when to pass a non-nil model.
func (c *Client) RunWithModel(ctx context.Context, lq *logicalquery.LogicalQuery, model *semantic.SemanticModel, maxRows, timeoutMs int) (*internalapi.RunResponse, error) {
	req := internalapi.RunRequest{
		LogicalQuery: *lq,
		Model:        model,
		MaxRows:      maxRows,
		TimeoutMs:    timeoutMs,
	}
	var resp internalapi.RunResponse
	if err := c.do(ctx, "/run", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// DryRun validates and compiles a LogicalQuery without executing it. Use this
// when you only need the SQL/fingerprint for audit, caching, or preview.
func (c *Client) DryRun(ctx context.Context, lq *logicalquery.LogicalQuery) (*internalapi.DryRunResponse, error) {
	return c.DryRunWithModel(ctx, lq, nil)
}

// DryRunWithModel is DryRun with an optional inline semantic model; see
// CompileWithModel for when to pass a non-nil model.
func (c *Client) DryRunWithModel(ctx context.Context, lq *logicalquery.LogicalQuery, model *semantic.SemanticModel) (*internalapi.DryRunResponse, error) {
	req := internalapi.DryRunRequest{LogicalQuery: *lq, Model: model}
	var resp internalapi.DryRunResponse
	if err := c.do(ctx, "/dry-run", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
