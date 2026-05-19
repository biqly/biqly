package aiclient

import (
	"encoding/json"

	"github.com/biqly/biqly/internal/ai"
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
//
// TODO(decomposition): move shared shapes into pkg/aitypes after Phase 4.
type QueryResponse = ai.Response

// PreviewResponse is the wire shape returned by POST /api/ai/query/preview
// (includes compiled sql/args when compilation succeeds).
type PreviewResponse = ai.Response

// RunResponse is the wire shape returned by POST /api/ai/query/run
// (includes result rows when execution succeeds).
type RunResponse = ai.Response

// DescribeRequest is the JSON body for POST /api/ai/metadata/describe.
type DescribeRequest = ai.DescribeRequest

// DescribeResponse is returned by POST /api/ai/metadata/describe.
type DescribeResponse = ai.DescribeResult

// EmbedRequest is the JSON body for POST /api/ai/metadata/embed.
type EmbedRequest struct {
	DatasourceID string `json:"datasource_id"`
	ModelID      string `json:"model_id,omitempty"`
}

// EmbedResponse summarizes an embedding refresh run.
type EmbedResponse struct {
	DatasourceID string                `json:"datasource_id"`
	ModelID      string                `json:"model_id,omitempty"`
	Model        string                `json:"model"`
	Embedded     int                   `json:"embedded"`
	Skipped      int                   `json:"skipped"`
	Results      []ai.EmbedTableResult `json:"results,omitempty"`
}

// SettingsResponse is the non-secret runtime AI configuration from GET /api/ai/settings.
type SettingsResponse struct {
	Provider         string `json:"provider"`
	LLMModel         string `json:"llm_model"`
	BaseURL          string `json:"base_url"`
	BaseURLEffective string `json:"base_url_effective"`
	APIKeyConfigured bool   `json:"api_key_configured"`

	QueryModelOverride        bool   `json:"query_model_override"`
	QueryProvider             string `json:"query_provider,omitempty"`
	QueryModel                string `json:"query_model,omitempty"`
	QueryBaseURL              string `json:"query_base_url,omitempty"`
	QueryBaseURLEffective     string `json:"query_base_url_effective,omitempty"`
	QueryAPIKeyConfigured     bool   `json:"query_api_key_configured,omitempty"`
	QueryAPIKeyDedicated      bool   `json:"query_api_key_dedicated,omitempty"`
	QueryHTTPTimeoutSeconds   int    `json:"query_http_timeout_seconds,omitempty"`

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
