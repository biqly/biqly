package ai

import (
	"github.com/biqly/biqly/internal/query"
)

// Request is the input for the AI query endpoint.
type Request struct {
	DatasourceID string   `json:"datasource_id"`
	ModelID      string   `json:"model_id,omitempty"`
	Question     string   `json:"question"`
	Tables       []string `json:"tables,omitempty"`
}

// AIRequest is a deprecated alias for Request.
//
//nolint:revive // alias for backward compatibility
type AIRequest = Request

// Response is the output from the AI query endpoint.
type Response struct {
	LogicalQuery          *query.LogicalQuery `json:"logical_query,omitempty"`
	SQL                   string              `json:"sql,omitempty"`
	Args                  []any               `json:"args,omitempty"`
	Warnings              []string            `json:"warnings,omitempty"`
	Result                *query.QueryResult  `json:"result,omitempty"`
	Confidence            float64             `json:"confidence"`
	TableRouting          *TableRoutingResult `json:"table_routing,omitempty"`
	NeedsClarification    bool                `json:"needs_clarification,omitempty"`
	ClarificationQuestion string              `json:"clarification_question,omitempty"`
	ClarificationOptions  []string            `json:"clarification_options,omitempty"`
	Prompt                string              `json:"-"`
	RawResponse           string              `json:"-"`
	// Multi-candidate generation
	Candidates      []CandidateEntry `json:"candidates,omitempty"`
	CandidatesCount int              `json:"candidates_count,omitempty"`
	// Retry / validation
	RetryCount       int                    `json:"retry_count,omitempty"`
	ValidationResult *ValidationExplainResult `json:"validation_result,omitempty"`
	// Model / cost tracking
	ModelUsed  string      `json:"model_used,omitempty"`
	TokenUsage *TokenUsage `json:"token_usage,omitempty"`
	CostUSD    float64     `json:"cost_usd,omitempty"`
	LatencyMs  int         `json:"latency_ms,omitempty"`
	// Visualization hint for frontend chart auto-selection
	VisualizationHint *VisualizationHint `json:"visualization_hint,omitempty"`
}

// CandidateEntry represents one LogicalQuery candidate from self-consistency.
type CandidateEntry struct {
	LogicalQuery *query.LogicalQuery `json:"logical_query"`
	Confidence   float64             `json:"confidence"`
	Reasoning    string              `json:"reasoning,omitempty"`
}

// ValidationExplainResult holds the EXPLAIN output and whether the plan is OK.
type ValidationExplainResult struct {
	ExplainOutput string `json:"explain_output,omitempty"`
	PlanOK        bool   `json:"plan_ok"`
}

// TokenUsage tracks LLM token consumption.
type TokenUsage struct {
	Prompt     int `json:"prompt"`
	Completion int `json:"completion"`
	Total      int `json:"total"`
}

// VisualizationHint suggests a chart type for the result data.
type VisualizationHint struct {
	ChartType string `json:"chart_type"` // bar, line, pie, table
	Reason    string `json:"reason"`
}

// AIResponse is a deprecated alias for Response.
//
//nolint:revive // alias for backward compatibility
type AIResponse = Response

// LogicalQuerySchema defines the JSON schema the AI must output.
const LogicalQuerySchema = `{
  "type": "object",
  "required": ["select"],
  "properties": {
    "datasource_id": {"type": "string"},
    "model_id": {"type": "string"},
    "select": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["type", "name"],
        "properties": {
          "type": {"type": "string", "enum": ["dimension", "metric"]},
          "name": {"type": "string"},
          "alias": {"type": "string"}
        }
      }
    },
    "filters": {
      "type": "array",
      "items": {
        "type": "object",
        "required": ["field", "operator"],
        "properties": {
          "field": {"type": "string"},
          "operator": {"type": "string", "enum": ["eq","neq","gt","gte","lt","lte","in","not_in","contains","starts_with","ends_with","between","is_null","is_not_null"]},
          "value": {}
        }
      }
    },
    "group_by": {
      "type": "array",
      "items": {"type": "object", "required": ["field"], "properties": {"field": {"type": "string"}}}
    },
    "order_by": {
      "type": "array",
      "items": {"type": "object", "required": ["field"], "properties": {"field": {"type": "string"}, "direction": {"type": "string", "enum": ["asc", "desc"]}}}
    },
    "limit": {"type": "integer", "minimum": 0},
    "offset": {"type": "integer", "minimum": 0}
  },
  "additionalProperties": false
}`
