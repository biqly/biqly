package ai

import (
	promptpkg "github.com/biqly/biqly/internal/ai/prompt"
	providerpkg "github.com/biqly/biqly/internal/ai/provider"
	"github.com/biqly/biqly/internal/ai/routing"
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
type AIRequest = Request

// Response is the output from the AI query endpoint.
type Response struct {
	LogicalQuery          *query.LogicalQuery `json:"logical_query,omitempty"`
	SQL                   string              `json:"sql,omitempty"`
	Args                  []any               `json:"args,omitempty"`
	Warnings              []string            `json:"warnings,omitempty"`
	Result                *query.QueryResult  `json:"result,omitempty"`
	Confidence            float64             `json:"confidence"`
	TableRouting          *routing.TableRoutingResult `json:"table_routing,omitempty"`
	NeedsClarification    bool                `json:"needs_clarification,omitempty"`
	ClarificationQuestion string              `json:"clarification_question,omitempty"`
	ClarificationOptions  []string            `json:"clarification_options,omitempty"`
	// Clarification is the structured form of a needs-clarification response.
	// Populated alongside ClarificationQuestion/ClarificationOptions when the
	// router or validator cannot proceed without user input. Frontend can
	// render the options as selectable chips.
	Clarification *Clarification `json:"clarification,omitempty"`
	Prompt        string         `json:"-"`
	RawResponse   string         `json:"-"`
	// Multi-candidate generation
	Candidates      []CandidateEntry `json:"candidates,omitempty"`
	CandidatesCount int              `json:"candidates_count,omitempty"`
	// Retry / validation
	RetryCount       int                      `json:"retry_count,omitempty"`
	ValidationResult *ValidationExplainResult `json:"validation_result,omitempty"`
	// Model / cost tracking
	ModelUsed   string       `json:"model_used,omitempty"`
	PromptStats *promptpkg.PromptStats `json:"prompt_stats,omitempty"`
	TokenUsage  *providerpkg.TokenUsage  `json:"token_usage,omitempty"`
	CostUSD     float64      `json:"cost_usd,omitempty"`
	LatencyMs   int          `json:"latency_ms,omitempty"`
	// Prompt template traceability for prompt-version A/B comparison and evals.
	PromptTemplateLocale        string         `json:"prompt_template_locale,omitempty"`
	PromptTemplateVersions      map[string]int `json:"prompt_template_versions,omitempty"`
	PromptTemplateBundleVersion int            `json:"prompt_template_bundle_version,omitempty"`
	// Visualization hint for frontend chart auto-selection
	VisualizationHint *VisualizationHint `json:"visualization_hint,omitempty"`
}

// ClarificationStatus enumerates the structured clarification states.
const (
	ClarificationStatusNeeded = "needs_clarification"
)

// Clarification surfaces a structured ask-the-user response from the AI or the
// table router. The frontend renders Question with Options as selectable
// answers; selecting an option re-issues the original request with the chosen
// key appended (the backend wires it through Request.Tables or a dedicated
// clarification continuation depending on the source).
type Clarification struct {
	Status     string                 `json:"status"`               // ClarificationStatusNeeded
	Question   string                 `json:"question"`             // user-facing prompt
	Reason     string                 `json:"reason,omitempty"`     // short explanation, e.g. "Multiple revenue metrics matched"
	Options    []ClarificationOption  `json:"options,omitempty"`    // discrete choices
	Candidates []ClarificationContext `json:"candidates,omitempty"` // alternative semantic contexts when the router was unsure
	Source     string                 `json:"source,omitempty"`     // "router" | "validator" | "ai"
}

// ClarificationOption is a single discrete answer a user can pick.
type ClarificationOption struct {
	Key   string `json:"key"`            // machine-readable answer key
	Label string `json:"label"`          // human-readable label
	Hint  string `json:"hint,omitempty"` // optional explanation
}

// ClarificationContext describes one of several candidate semantic contexts
// the router was deciding between. Surfaces to the user as "did you mean this
// model?" when scores were close.
type ClarificationContext struct {
	Type   string  `json:"type"`             // "semantic_model" | "table"
	Name   string  `json:"name"`             // model or table identifier
	Score  float64 `json:"score,omitempty"`  // routing score [0,1]
	Reason string  `json:"reason,omitempty"` // short why this candidate
}

// ClarificationFromRouting wraps an ambiguous table-routing decision into the
// structured Clarification envelope so the frontend can render the router's
// top candidates as selectable options. Returns nil when the routing did not
// flag NeedsClarification or has no candidates to surface.
func ClarificationFromRouting(result *routing.TableRoutingResult, question string) *Clarification {
	if result == nil || !result.NeedsClarification || len(result.Candidates) == 0 {
		return nil
	}
	if question == "" {
		question = "Which table set should I use to answer this question?"
	}
	candidates := make([]ClarificationContext, 0, len(result.Candidates))
	options := make([]ClarificationOption, 0, len(result.Candidates))
	for _, c := range result.Candidates {
		label := c.Table
		if c.Description != "" {
			label = c.Table + " — " + c.Description
		}
		candidates = append(candidates, ClarificationContext{
			Type:   "table",
			Name:   c.Table,
			Score:  c.Score,
			Reason: c.RejectedReason,
		})
		options = append(options, ClarificationOption{
			Key:   c.Table,
			Label: label,
		})
	}
	return &Clarification{
		Status:     ClarificationStatusNeeded,
		Question:   question,
		Reason:     "Multiple table sets scored similarly for this question.",
		Options:    options,
		Candidates: candidates,
		Source:     "router",
	}
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

// VisualizationHint suggests a chart type for the result data.
type VisualizationHint struct {
	ChartType string `json:"chart_type"` // bar, line, pie, table
	Reason    string `json:"reason"`
}

// AIResponse is a deprecated alias for Response.
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
      "items": {
        "type": "object",
        "required": ["field"],
        "properties": {
          "field": {"type": "string"},
          "time_grain": {"type": "string", "enum": ["day","week","month","quarter","year"]}
        }
      }
    },
    "order_by": {
      "type": "array",
      "items": {"type": "object", "required": ["field"], "properties": {"field": {"type": "string"}, "direction": {"type": "string", "enum": ["asc", "desc"]}}}
    },
    "limit": {"type": "integer", "minimum": 0},
    "offset": {"type": "integer", "minimum": 0},
    "default_schema": {"type": "string"},
    "table_schemas": {
      "type": "object",
      "additionalProperties": {"type": "string"}
    }
  },
  "additionalProperties": false
}`
