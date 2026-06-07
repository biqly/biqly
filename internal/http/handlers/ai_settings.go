package handlers

import (
	"net/http"
	"strings"

	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/ai/prompt"
	"github.com/biqly/biqly/internal/config"
)

// aiRuntimeSettingsResponse is safe to expose in the browser: no secrets.
type aiRuntimeSettingsResponse struct {
	Provider         string `json:"provider"`
	LLMModel         string `json:"llm_model"`
	BaseURL          string `json:"base_url"`
	BaseURLEffective string `json:"base_url_effective"`
	APIKeyConfigured bool   `json:"api_key_configured"`

	// QueryModelOverride is true when BI_AI_QUERY_* knobs split the
	// NL→LogicalQuery model away from the base. QueryModel / QueryBaseURL /
	// QueryProvider always report the effective values so the UI can show
	// "AI Sorgu modeli: X" alongside "Describe modeli: Y" — frontend hides
	// the dedicated row when override is false to avoid duplicate badges.
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

	// DBManaged reports whether provider/model selection is sourced from the
	// ai_providers / ai_models tables (admin-managed) rather than env vars.
	// ActiveModels lists the current default model per purpose when DB-managed.
	DBManaged    bool                 `json:"db_managed"`
	ActiveModels []activeModelSummary `json:"active_models,omitempty"`
}

// activeModelSummary is the non-secret view of a default model per purpose.
type activeModelSummary struct {
	Purpose      string `json:"purpose"`
	ModelID      string `json:"model_id"`
	DisplayName  string `json:"display_name"`
	ProviderName string `json:"provider_name"`
	ProviderType string `json:"provider_type"`
}

func effectiveAIBaseURL(cfg config.AIConfig) string {
	conn := cfg.Connection
	if strings.TrimSpace(conn.BaseURL) != "" {
		return conn.BaseURL
	}
	switch strings.ToLower(strings.TrimSpace(conn.Provider)) {
	case "", "openai", "openai-compatible":
		return "https://api.openai.com/v1 (default when BI_AI_BASE_URL is empty)"
	case "anthropic":
		return "https://api.anthropic.com/v1 (default when BI_AI_BASE_URL is empty)"
	default:
		return "(provider default — set BI_AI_BASE_URL to override)"
	}
}

// embeddingBaseURLEffectiveLabel explains which URL embeddings HTTP calls use.
func embeddingBaseURLEffectiveLabel(cfg config.AIConfig) string {
	eff := cfg.EffectiveEmbeddingBaseURL()
	if eff == "" {
		return "— (set BI_AI_EMBEDDING_BASE_URL or BI_AI_BASE_URL for OpenAI-compatible providers)"
	}
	if strings.TrimSpace(cfg.Embedding.BaseURL) != "" {
		return eff
	}
	if strings.TrimSpace(cfg.Connection.BaseURL) != "" {
		return eff + " (from BI_AI_BASE_URL; override with BI_AI_EMBEDDING_BASE_URL)"
	}
	return eff + " (default when embedding URL env vars are empty)"
}

func translationBaseURLEffectiveLabel(cfg config.AIConfig) string {
	eff := cfg.EffectiveTranslationBaseURL()
	if eff == "" {
		return "— (set BI_AI_TRANSLATION_BASE_URL or BI_AI_BASE_URL)"
	}
	if strings.TrimSpace(cfg.Translation.BaseURL) != "" {
		return eff
	}
	return eff + " (from BI_AI_BASE_URL; override with BI_AI_TRANSLATION_BASE_URL)"
}

// RuntimeSettings returns non-secret AI configuration for the UI. Operational
// knobs come from the environment; provider/model selection is overlaid from
// the database (the sole source of truth for connections and models).
func (h *AIHandler) RuntimeSettings(w http.ResponseWriter, r *http.Request) {
	cfg := h.deps.Config.AI
	if h.deps.AIProviderStore != nil {
		// Overlay DB-resolved embedding/translation, then the base (describe) and
		// query connections, so the UI reflects what actually runs.
		cfg = h.deps.AIProviderStore.EffectiveConfig()
		if dc, ok := h.deps.AIProviderStore.ChatConfigForPurpose(ai.PurposeDescribe); ok {
			cfg.Connection.Provider = dc.Connection.Provider
			cfg.Connection.Model = dc.Connection.Model
			cfg.Connection.BaseURL = dc.Connection.BaseURL
			cfg.Connection.APIKey = dc.Connection.APIKey
		}
		if qc, ok := h.deps.AIProviderStore.ChatConfigForPurpose(ai.PurposeQuery); ok {
			cfg.Query.Provider = qc.Connection.Provider
			cfg.Query.Model = qc.Connection.Model
			cfg.Query.BaseURL = qc.Connection.BaseURL
			cfg.Query.APIKey = qc.Connection.APIKey
		}
	}
	queryCfg := cfg.EffectiveQueryConfig()
	profile := prompt.LookupModelContextProfile(queryCfg.Connection.Model, queryCfg.Generation.NumCtx)
	out := aiRuntimeSettingsResponse{
		Provider:         cfg.Connection.Provider,
		LLMModel:         cfg.Connection.Model,
		BaseURL:          cfg.Connection.BaseURL,
		BaseURLEffective: effectiveAIBaseURL(cfg),
		APIKeyConfigured: strings.TrimSpace(cfg.Connection.APIKey) != "",
	}
	out.QueryModelOverride = cfg.HasQueryOverride()
	out.QueryProvider = queryCfg.Connection.Provider
	out.QueryModel = queryCfg.Connection.Model
	out.QueryBaseURL = queryCfg.Connection.BaseURL
	out.QueryBaseURLEffective = effectiveAIBaseURL(queryCfg)
	out.QueryAPIKeyConfigured = strings.TrimSpace(queryCfg.Connection.APIKey) != ""
	out.QueryAPIKeyDedicated = strings.TrimSpace(cfg.Query.APIKey) != ""
	if cfg.Query.HTTPTimeoutSeconds > 0 {
		out.QueryHTTPTimeoutSeconds = cfg.Query.HTTPTimeoutSeconds
	}
	out.MaxPromptInputRunes = queryCfg.Generation.MaxPromptInputRunes
	out.EffectiveMaxPromptRunes = prompt.EffectiveMaxPromptRunes(queryCfg, queryCfg.Connection.Model)
	out.ContextWindowTokens = profile.ContextWindowTokens
	out.ContextWindowSource = profile.Source
	// cfg already carries the DB-resolved embedding/translation overlay.
	embedCfg := cfg
	if embedCfg.EmbeddingsConfigured() {
		out.EmbeddingsEnabled = true
		out.EmbeddingModel = strings.TrimSpace(embedCfg.Embedding.Model)
		out.EmbeddingBaseURL = embedCfg.Embedding.BaseURL
		out.EmbeddingBaseURLEffective = embeddingBaseURLEffectiveLabel(embedCfg)
		out.EmbeddingAPIKeyConfigured = strings.TrimSpace(embedCfg.EffectiveEmbeddingAPIKey()) != ""
		out.EmbeddingAPIKeyDedicated = strings.TrimSpace(embedCfg.Embedding.APIKey) != ""
	}
	if cfg.TranslationConfigured() {
		out.TranslationEnabled = true
		out.TranslationModel = strings.TrimSpace(cfg.Translation.Model)
		out.TranslationBaseURL = cfg.Translation.BaseURL
		out.TranslationBaseURLEffective = translationBaseURLEffectiveLabel(cfg)
		out.TranslationAPIKeyConfigured = strings.TrimSpace(cfg.EffectiveTranslationAPIKey()) != ""
		out.TranslationAPIKeyDedicated = strings.TrimSpace(cfg.Translation.APIKey) != ""
		out.TranslationTargetLanguage = cfg.Translation.TargetLanguage
		out.TranslationTargetCode = cfg.Translation.TargetCode
	}
	out.DBManaged = true
	if h.deps.AIProviderStore != nil {
		if rows, err := h.deps.AIProviderStore.ActiveModels(r.Context()); err == nil {
			out.ActiveModels = make([]activeModelSummary, 0, len(rows))
			for _, m := range rows {
				out.ActiveModels = append(out.ActiveModels, activeModelSummary{
					Purpose:      m.Purpose,
					ModelID:      m.ModelID,
					DisplayName:  m.DisplayName,
					ProviderName: m.ProviderName,
					ProviderType: m.ProviderType,
				})
			}
		}
	}
	writeJSON(w, http.StatusOK, out)
}
