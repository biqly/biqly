// Package ai defines the AI query endpoint schema and response types.
package ai

import (
	"fmt"

	ambiguitypkg "github.com/biqly/biqly/internal/ai/ambiguity"
	promptpkg "github.com/biqly/biqly/internal/ai/prompt"
	providerpkg "github.com/biqly/biqly/internal/ai/provider"
	"github.com/biqly/biqly/internal/ai/routing"
	"github.com/biqly/biqly/internal/i18n"
	"github.com/biqly/biqly/internal/query"
)

// Request is the input for the AI query endpoint.
type Request struct {
	DatasourceID        string   `json:"datasource_id"`
	ModelID             string   `json:"model_id,omitempty"`
	Question            string   `json:"question"`
	Tables              []string `json:"tables,omitempty"`
	ClarificationChoice string   `json:"clarification_choice,omitempty"`
}

// AIRequest is a deprecated alias for Request.
type AIRequest = Request

type AIResult struct {
	LogicalQuery      *query.LogicalQuery `json:"logical_query,omitempty"`
	SQL               string              `json:"sql,omitempty"`
	Args              []any               `json:"args,omitempty"`
	Warnings          []string            `json:"warnings,omitempty"`
	Result            *query.Result       `json:"result,omitempty"`
	Confidence        float64             `json:"confidence"`
	VisualizationHint *VisualizationHint  `json:"visualization_hint,omitempty"`
}

type AIMetadata struct {
	ModelUsed                   string                      `json:"model_used,omitempty"`
	PromptStats                 *promptpkg.Stats            `json:"prompt_stats,omitempty"`
	TokenUsage                  *providerpkg.TokenUsage     `json:"token_usage,omitempty"`
	CostUSD                     float64                     `json:"cost_usd,omitempty"`
	LatencyMs                   int                         `json:"latency_ms,omitempty"`
	LLMGenerateDurationMs       int                         `json:"llm_generate_duration_ms,omitempty"`
	RetryCount                  int                         `json:"retry_count,omitempty"`
	TableRouting                *routing.TableRoutingResult `json:"table_routing,omitempty"`
	ValidationResult            *ValidationExplainResult    `json:"validation_result,omitempty"`
	Prompt                      string                      `json:"-"`
	RawResponse                 string                      `json:"-"`
	PromptTemplateLocale        string                      `json:"prompt_template_locale,omitempty"`
	PromptTemplateVersions      map[string]int              `json:"prompt_template_versions,omitempty"`
	PromptTemplateBundleVersion int                         `json:"prompt_template_bundle_version,omitempty"`
	ABExperimentID              string                      `json:"ab_experiment_id,omitempty"`
	ABVariantID                 string                      `json:"ab_variant_id,omitempty"`
	Candidates                  []CandidateEntry            `json:"candidates,omitempty"`
	CandidatesCount             int                         `json:"candidates_count,omitempty"`
	RepairDetails               []RepairDetail              `json:"repair_details,omitempty"`
	GenerationTrace             *GenerationTrace            `json:"generation_trace,omitempty"`
}

type RepairDetail struct {
	Attempt    int      `json:"attempt"`
	ErrorCodes []string `json:"error_codes"`
	ErrorsJSON string   `json:"errors_json,omitempty"`
	Strategy   string   `json:"strategy"`
}

type ClarificationResponse struct {
	NeedsClarification    bool           `json:"needs_clarification,omitempty"`
	ClarificationQuestion string         `json:"clarification_question,omitempty"`
	ClarificationOptions  []string       `json:"clarification_options,omitempty"`
	ClarificationRound    int            `json:"clarification_round,omitempty"`
	Clarification         *Clarification `json:"clarification,omitempty"`
	// ResolvedQuestion is the question text after applying earlier
	// clarification choices. Follow-up rounds MUST send this (not the
	// original question) so option keys resolve against the same analysis.
	ResolvedQuestion string `json:"resolved_question,omitempty"`
}

// Response is the output from the AI query endpoint.
type Response struct {
	Result        *AIResult              `json:"result,omitempty"`
	Metadata      *AIMetadata            `json:"metadata,omitempty"`
	Clarification *ClarificationResponse `json:"clarification,omitempty"`
}

// AIResponse is a deprecated alias for Response.
type AIResponse = Response

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
	Status          string                 `json:"status"`               // ClarificationStatusNeeded
	Question        string                 `json:"question"`             // user-facing prompt
	Reason          string                 `json:"reason,omitempty"`     // short explanation, e.g. "Multiple revenue metrics matched"
	Options         []ClarificationOption  `json:"options,omitempty"`    // discrete choices
	Candidates      []ClarificationContext `json:"candidates,omitempty"` // alternative semantic contexts when the router was unsure
	Source          string                 `json:"source,omitempty"`     // "router" | "validator" | "ai" | "ambiguity_analyzer"
	AmbiguityDetail *AmbiguityDetail       `json:"ambiguity_detail,omitempty"`
}

// AmbiguityDetail carries the analyzer evidence needed to render semantic choices.
type AmbiguityDetail struct {
	Ambiguities []ambiguitypkg.Item `json:"ambiguities"`
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

// ClarificationFromAmbiguity wraps semantic ambiguities into selectable options.
func ClarificationFromAmbiguity(result ambiguitypkg.Result) *Clarification {
	return ClarificationFromAmbiguityWithMaxOptions(i18n.DefaultLocale, result, 0)
}

// ClarificationFromAmbiguityWithMaxOptions wraps semantic ambiguities into a
// bounded list of selectable options. A non-positive maximum leaves the list
// uncapped for backward compatibility.
func ClarificationFromAmbiguityWithMaxOptions(locale i18n.Locale, result ambiguitypkg.Result, maxOptions int) *Clarification {
	if !result.IsAmbiguous || len(result.Ambiguities) == 0 {
		return nil
	}

	var options []ClarificationOption
	for ambiguityIndex, item := range result.Ambiguities {
		for interpretationIndex, interpretation := range item.Interpretations {
			options = append(options, ClarificationOption{
				Key:   fmt.Sprintf("ambiguity:%d:%d", ambiguityIndex, interpretationIndex),
				Label: interpretation.Label,
				Hint:  interpretation.Description,
			})
			if maxOptions > 0 && len(options) >= maxOptions {
				break
			}
		}
		if maxOptions > 0 && len(options) >= maxOptions {
			break
		}
	}
	if len(options) == 0 {
		return nil
	}

	question := i18n.T(locale, "clarification.ambiguity_question_multiple")
	if len(result.Ambiguities) == 1 {
		question = i18n.Tf(locale, "clarification.ambiguity_question_single", map[string]any{"Term": result.Ambiguities[0].Term})
	}
	return &Clarification{
		Status:   ClarificationStatusNeeded,
		Question: question,
		Reason:   i18n.T(locale, "clarification.ambiguity_reason"),
		Options:  options,
		Source:   "ambiguity_analyzer",
		AmbiguityDetail: &AmbiguityDetail{
			Ambiguities: result.Ambiguities,
		},
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
          "operator": {"type": "string", "enum": ["eq","neq","gt","gte","lt","lte","in","not_in","contains","starts_with","ends_with","between","is_null","is_not_null","is_empty","is_not_empty"]},
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
