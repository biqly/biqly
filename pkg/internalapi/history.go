package internalapi

import (
	"time"

	ai "github.com/biqly/biqly/internal/ai/eval"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/query"
)

// AIHistoryRequest is the body of POST /internal/ai-history.
//
// Wraps metadata.AIQueryHistoryEntry so the wire shape can grow independently
// of the persistence struct (e.g. when adding correlation IDs, tracing
// metadata). The Entry field is the persisted shape; everything else is
// transport-layer metadata.
type AIHistoryRequest struct {
	Entry metadata.AIQueryHistoryEntry `json:"entry"`
}

func (r AIHistoryRequest) HistoryDatasourceID() string { return r.Entry.DatasourceID }
func (r AIHistoryRequest) HistoryID() string           { return r.Entry.ID }

// AIHistoryResponse is the body of POST /internal/ai-history. ID is the
// row identifier assigned by the catalog service, which the AI service
// SHOULD persist alongside the user-visible response so feedback /
// follow-up calls can reference the same history row.
type AIHistoryResponse struct {
	ID string `json:"id"`
}

// QueryHistoryRequest is the body of POST /internal/query-history.
type QueryHistoryRequest struct {
	Entry query.HistoryEntry `json:"entry"`
}

func (r QueryHistoryRequest) HistoryDatasourceID() string { return r.Entry.DatasourceID }
func (r QueryHistoryRequest) HistoryID() string             { return r.Entry.ID }

// QueryHistoryResponse is the body of POST /internal/query-history.
type QueryHistoryResponse struct {
	ID string `json:"id"`
}

// EvalResultsRequest is the body of POST /internal/eval-results.
type EvalResultsRequest struct {
	RunID            string                     `json:"run_id"`
	Provider         string                     `json:"provider"`
	Model            string                     `json:"model"`
	ContextVersion   int                        `json:"context_version"`
	ContextUpdatedAt time.Time                  `json:"context_updated_at"`
	Results          []ai.EvalResultWithMetrics `json:"results"`
}

// EvalResultsResponse acknowledges a persisted eval result batch.
type EvalResultsResponse struct {
	RunID      string `json:"run_id"`
	TotalCases int    `json:"total_cases"`
}
