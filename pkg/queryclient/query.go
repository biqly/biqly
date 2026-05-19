package queryclient

import (
	"context"

	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/pkg/internalapi"
)

// Compile validates a LogicalQuery and returns the parameterized SQL plus the
// canonical fingerprint. It does NOT execute the query.
func (c *Client) Compile(ctx context.Context, lq query.LogicalQuery) (*internalapi.CompileResponse, error) {
	req := internalapi.CompileRequest{LogicalQuery: lq}
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
func (c *Client) Run(ctx context.Context, lq query.LogicalQuery, maxRows, timeoutMs int) (*internalapi.RunResponse, error) {
	req := internalapi.RunRequest{
		LogicalQuery: lq,
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
func (c *Client) DryRun(ctx context.Context, lq query.LogicalQuery) (*internalapi.DryRunResponse, error) {
	req := internalapi.DryRunRequest{LogicalQuery: lq}
	var resp internalapi.DryRunResponse
	if err := c.do(ctx, "/dry-run", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
