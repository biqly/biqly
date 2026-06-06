package aiclient

import (
	"encoding/json"

	"github.com/biqly/biqly/pkg/logicalquery"
	pkgquery "github.com/biqly/biqly/pkg/query"
)

// PriorTurn is one conversational turn sent so follow-up questions stay in context.
type PriorTurn struct {
	Question     string          `json:"question"`
	LogicalQuery json.RawMessage `json:"logical_query,omitempty"`
	Note         string          `json:"note,omitempty"`
}

// QueryRequest is the JSON body for POST /api/ai/query, /preview, and /run.
type QueryRequest struct {
	DatasourceID       string      `json:"datasource_id"`
	ModelID            string      `json:"model_id,omitempty"`
	Question           string      `json:"question"`
	Tables             []string    `json:"tables,omitempty"`
	IncludeBaseTables  *bool       `json:"include_base_tables,omitempty"`
	IncludeViews       *bool       `json:"include_views,omitempty"`
	PriorTurns         []PriorTurn `json:"prior_turns,omitempty"`
	ExampleIDs         []string    `json:"example_ids,omitempty"`
	IncludePastQueries bool        `json:"include_past_queries,omitempty"`
}

// QueryResponse is the wire shape returned by POST /api/ai/query.
type QueryResponse = Response

// PreviewResponse is the wire shape returned by POST /api/ai/query/preview
// (includes compiled sql/args when compilation succeeds).
type PreviewResponse = Response

// RunResponse is the wire shape returned by POST /api/ai/query/run
// (includes result rows when execution succeeds).
type RunResponse = Response

// Response is the output from the AI query endpoint.
type Response struct {
	Result        *AIResult              `json:"result,omitempty"`
	Metadata      *AIMetadata            `json:"metadata,omitempty"`
	Clarification *ClarificationResponse `json:"clarification,omitempty"`
}

type AIResult struct {
	LogicalQuery      *logicalquery.LogicalQuery `json:"logical_query,omitempty"`
	SQL               string                     `json:"sql,omitempty"`
	Args              []any                      `json:"args,omitempty"`
	Warnings          []string                   `json:"warnings,omitempty"`
	Result            *pkgquery.Result           `json:"result,omitempty"`
	Confidence        float64                    `json:"confidence"`
	VisualizationHint any                        `json:"visualization_hint,omitempty"`
}

type AIMetadata struct {
	ModelUsed                   string         `json:"model_used,omitempty"`
	PromptStats                 any            `json:"prompt_stats,omitempty"`
	TokenUsage                  any            `json:"token_usage,omitempty"`
	CostUSD                     float64        `json:"cost_usd,omitempty"`
	LatencyMs                   int            `json:"latency_ms,omitempty"`
	RetryCount                  int            `json:"retry_count,omitempty"`
	TableRouting                any            `json:"table_routing,omitempty"`
	ValidationResult            any            `json:"validation_result,omitempty"`
	PromptTemplateLocale        string         `json:"prompt_template_locale,omitempty"`
	PromptTemplateVersions      map[string]int `json:"prompt_template_versions,omitempty"`
	PromptTemplateBundleVersion int            `json:"prompt_template_bundle_version,omitempty"`
	ABExperimentID              string         `json:"ab_experiment_id,omitempty"`
	ABVariantID                 string         `json:"ab_variant_id,omitempty"`
	Candidates                  []any          `json:"candidates,omitempty"`
	CandidatesCount             int            `json:"candidates_count,omitempty"`
	RepairDetails               []RepairDetail `json:"repair_details,omitempty"`
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
	Clarification         *Clarification `json:"clarification,omitempty"`
}

type Clarification struct {
	Status          string                 `json:"status"`
	Question        string                 `json:"question"`
	Reason          string                 `json:"reason,omitempty"`
	Options         []ClarificationOption  `json:"options,omitempty"`
	Candidates      []ClarificationContext `json:"candidates,omitempty"`
	Source          string                 `json:"source,omitempty"`
	AmbiguityDetail any                    `json:"ambiguity_detail,omitempty"`
}

type ClarificationOption struct {
	Key   string `json:"key"`
	Label string `json:"label"`
	Hint  string `json:"hint,omitempty"`
}

type ClarificationContext struct {
	Type   string  `json:"type"`
	Name   string  `json:"name"`
	Score  float64 `json:"score,omitempty"`
	Reason string  `json:"reason,omitempty"`
}

// DescribeRequest is the JSON body for POST /api/ai/metadata/describe.
type DescribeRequest struct {
	DatasourceID string `json:"datasource_id"`
	Schema       string `json:"schema"`
	Table        string `json:"table"`
	SampleSize   int    `json:"sample_size,omitempty"`
	AutoApply    bool   `json:"auto_apply,omitempty"`
}

// DescribeResponse is returned by POST /api/ai/metadata/describe.
type DescribeResponse struct {
	Table              string              `json:"table"`
	Schema             string              `json:"schema"`
	Description        string              `json:"description"`
	Columns            []ColumnDescription `json:"columns"`
	Applied            bool                `json:"applied"`
	SampleRows         int                 `json:"sample_rows"`
	Model              string              `json:"model,omitempty"`
	TranslationApplied bool                `json:"translation_applied,omitempty"`
	TranslationModel   string              `json:"translation_model,omitempty"`
	TranslationError   string              `json:"translation_error,omitempty"`
}

// ColumnDescription is one column's AI-generated description.
type ColumnDescription struct {
	Name        string `json:"name"`
	Description string `json:"description"`
}

// EmbedRequest is the JSON body for POST /api/ai/metadata/embed.
type EmbedRequest struct {
	DatasourceID string `json:"datasource_id"`
	ModelID      string `json:"model_id,omitempty"`
}

// EmbedResponse summarizes an embedding refresh run.
type EmbedResponse struct {
	DatasourceID string             `json:"datasource_id"`
	ModelID      string             `json:"model_id,omitempty"`
	Model        string             `json:"model"`
	Embedded     int                `json:"embedded"`
	Skipped      int                `json:"skipped"`
	Results      []EmbedTableResult `json:"results,omitempty"`
}

// EmbedTableResult is one table or column embedding outcome.
type EmbedTableResult struct {
	Schema  string `json:"schema"`
	Table   string `json:"table"`
	Column  string `json:"column,omitempty"`
	Kind    string `json:"kind"`
	Skipped bool   `json:"skipped,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// SettingsResponse is the non-secret runtime AI configuration from GET /api/ai/settings.
type SettingsResponse struct {
	Provider         string `json:"provider"`
	LLMModel         string `json:"llm_model"`
	BaseURL          string `json:"base_url"`
	BaseURLEffective string `json:"base_url_effective"`
	APIKeyConfigured bool   `json:"api_key_configured"`

	QueryModelOverride      bool   `json:"query_model_override"`
	QueryProvider           string `json:"query_provider,omitempty"`
	QueryModel              string `json:"query_model,omitempty"`
	QueryBaseURL            string `json:"query_base_url,omitempty"`
	QueryBaseURLEffective   string `json:"query_base_url_effective,omitempty"`
	QueryAPIKeyConfigured   bool   `json:"query_api_key_configured,omitempty"`
	QueryAPIKeyDedicated    bool   `json:"query_api_key_dedicated,omitempty"`
	QueryHTTPTimeoutSeconds int    `json:"query_http_timeout_seconds,omitempty"`

	EmbeddingsEnabled         bool   `json:"embeddings_enabled"`
	EmbeddingModel            string `json:"embedding_model,omitempty"`
	EmbeddingBaseURL          string `json:"embedding_base_url,omitempty"`
	EmbeddingBaseURLEffective string `json:"embedding_base_url_effective,omitempty"`
	EmbeddingAPIKeyConfigured bool   `json:"embedding_api_key_configured,omitempty"`
	EmbeddingAPIKeyDedicated  bool   `json:"embedding_api_key_dedicated,omitempty"`

	TranslationEnabled          bool   `json:"translation_enabled"`
	TranslationModel            string `json:"translation_model,omitempty"`
	TranslationBaseURL          string `json:"translation_base_url,omitempty"`
	TranslationBaseURLEffective string `json:"translation_base_url_effective,omitempty"`
	TranslationAPIKeyConfigured bool   `json:"translation_api_key_configured,omitempty"`
	TranslationAPIKeyDedicated  bool   `json:"translation_api_key_dedicated,omitempty"`
	TranslationTargetLanguage   string `json:"translation_target_language,omitempty"`
	TranslationTargetCode       string `json:"translation_target_code,omitempty"`

	MaxPromptInputRunes     int    `json:"max_prompt_input_runes,omitempty"`
	EffectiveMaxPromptRunes int    `json:"effective_max_prompt_runes,omitempty"`
	ContextWindowTokens     int    `json:"context_window_tokens,omitempty"`
	ContextWindowSource     string `json:"context_window_source,omitempty"`
}
