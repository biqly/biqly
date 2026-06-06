package catalogclient

import (
	"context"
	"time"

	ai "github.com/biqly/biqly/internal/ai/eval"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/pkg/internalapi"
)

// EvalResultsInput is the batch persisted by POST /internal/eval-results.
type EvalResultsInput struct {
	RunID            string
	Provider         string
	Model            string
	ContextVersion   int
	ContextUpdatedAt time.Time
	Results          []ai.ResultWithMetrics
}

// CreateAIHistory persists an AI query history row. The returned id is the
// canonical identifier the caller should attach to user-visible responses so
// feedback / follow-up calls correlate cleanly. If entry.ID is empty the
// server allocates one.
func (c *Client) CreateAIHistory(ctx context.Context, entry *metadata.AIQueryHistoryEntry) (string, error) {
	req := internalapi.AIHistoryRequest{Entry: *entry}
	var resp internalapi.AIHistoryResponse
	if err := c.post(ctx, "/history/ai", req, &resp); err != nil {
		return "", err
	}
	return resp.ID, nil
}

// CreateQueryHistory persists a query execution history row. Symmetric to
// CreateAIHistory; the returned id may be attached to downstream telemetry.
func (c *Client) CreateQueryHistory(ctx context.Context, entry *query.HistoryEntry) (string, error) {
	req := internalapi.QueryHistoryRequest{Entry: *entry}
	var resp internalapi.QueryHistoryResponse
	if err := c.post(ctx, "/history/query", req, &resp); err != nil {
		return "", err
	}
	return resp.ID, nil
}

// CreateEvalResults persists a completed eval run's per-case results and
// summary. Catalog owns this storage so AI Service can stay stateless.
func (c *Client) CreateEvalResults(ctx context.Context, in *EvalResultsInput) (*internalapi.EvalResultsResponse, error) {
	req := internalapi.EvalResultsRequest{
		RunID:            in.RunID,
		Provider:         in.Provider,
		Model:            in.Model,
		ContextVersion:   in.ContextVersion,
		ContextUpdatedAt: in.ContextUpdatedAt,
		Results:          in.Results,
	}
	var resp internalapi.EvalResultsResponse
	if err := c.post(ctx, "/eval-results", req, &resp); err != nil {
		return nil, err
	}
	return &resp, nil
}
