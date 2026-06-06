package internalapi

import (
	"time"

	"github.com/biqly/biqly/pkg/logicalquery"
	"github.com/biqly/biqly/pkg/metadata"
	"github.com/biqly/biqly/pkg/query"
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

func (r *AIHistoryRequest) HistoryDatasourceID() string { return r.Entry.DatasourceID }
func (r *AIHistoryRequest) HistoryID() string           { return r.Entry.ID }

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

func (r *QueryHistoryRequest) HistoryDatasourceID() string { return r.Entry.DatasourceID }
func (r *QueryHistoryRequest) HistoryID() string           { return r.Entry.ID }

// QueryHistoryResponse is the body of POST /internal/query-history.
type QueryHistoryResponse struct {
	ID string `json:"id"`
}

// EvalResultsRequest is the body of POST /internal/eval-results.
type EvalResultsRequest struct {
	RunID            string              `json:"run_id"`
	Provider         string              `json:"provider"`
	Model            string              `json:"model"`
	ContextVersion   int                 `json:"context_version"`
	ContextUpdatedAt time.Time           `json:"context_updated_at"`
	Results          []EvalResultMetrics `json:"results"`
}

// EvalResultsResponse acknowledges a persisted eval result batch.
type EvalResultsResponse struct {
	RunID      string `json:"run_id"`
	TotalCases int    `json:"total_cases"`
}

// EvalResultMetrics is the wire shape for one persisted eval case result.
type EvalResultMetrics struct {
	Case                        EvalGoldenCase             `json:"case"`
	Got                         *logicalquery.LogicalQuery `json:"got,omitempty"`
	Match                       bool                       `json:"match"`
	Reason                      string                     `json:"reason"`
	Confidence                  float64                    `json:"confidence"`
	LatencyMs                   int64                      `json:"latency_ms"`
	TokenCount                  int                        `json:"token_count"`
	PromptTemplateVersions      map[string]int             `json:"prompt_template_versions,omitempty"`
	PromptTemplateBundleVersion int                        `json:"prompt_template_bundle_version,omitempty"`
}

// EvalGoldenCase is the public wire shape for an eval benchmark case.
type EvalGoldenCase struct {
	ID          string                    `json:"id"`
	Question    string                    `json:"question"`
	Expected    logicalquery.LogicalQuery `json:"expected"`
	Tags        []string                  `json:"tags,omitempty"`
	Description string                    `json:"description,omitempty"`
}
