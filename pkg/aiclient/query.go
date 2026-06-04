package aiclient

import (
	"context"
)

// Query turns natural language into a LogicalQuery via POST /api/ai/query.
func (c *Client) Query(ctx context.Context, req QueryRequest) (*QueryResponse, error) {
	var resp QueryResponse
	if err := c.post(ctx, "/query", req, &resp); err != nil {
		return nil, err
	}
	if err := clarificationFromResponse(&resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Preview generates a LogicalQuery and compiled SQL via POST /api/ai/query/preview.
func (c *Client) Preview(ctx context.Context, req QueryRequest) (*PreviewResponse, error) {
	var resp PreviewResponse
	if err := c.post(ctx, "/query/preview", req, &resp); err != nil {
		return nil, err
	}
	if err := clarificationFromResponse(&resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

// Run generates, compiles, and executes a query via POST /api/ai/query/run.
func (c *Client) Run(ctx context.Context, req QueryRequest) (*RunResponse, error) {
	var resp RunResponse
	if err := c.post(ctx, "/query/run", req, &resp); err != nil {
		return nil, err
	}
	if err := clarificationFromResponse(&resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
