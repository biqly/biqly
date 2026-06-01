package handlers

import (
	"net/http"
	"strings"

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

	MaxPromptInputRunes      int `json:"max_prompt_input_runes,omitempty"`
	EffectiveMaxPromptRunes  int `json:"effective_max_prompt_runes,omitempty"`
	ContextWindowTokens      int `json:"context_window_tokens,omitempty"`
	ContextWindowSource      string `json:"context_window_source,omitempty"`

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
	if strings.TrimSpace(cfg.BaseURL) != "" {
		return cfg.BaseURL
	}
	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
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
	if strings.TrimSpace(cfg.EmbeddingBaseURL) != "" {
		return eff
	}
	if strings.TrimSpace(cfg.BaseURL) != "" {
		return eff + " (from BI_AI_BASE_URL; override with BI_AI_EMBEDDING_BASE_URL)"
	}
	return eff + " (default when embedding URL env vars are empty)"
}

func translationBaseURLEffectiveLabel(cfg config.AIConfig) string {
	eff := cfg.EffectiveTranslationBaseURL()
	if eff == "" {
		return "— (set BI_AI_TRANSLATION_BASE_URL or BI_AI_BASE_URL)"
	}
	if strings.TrimSpace(cfg.TranslationBaseURL) != "" {
		return eff
	}
	return eff + " (from BI_AI_BASE_URL; override with BI_AI_TRANSLATION_BASE_URL)"
}

// RuntimeSettings returns non-secret AI configuration for the UI (env-backed).
func (h *AIHandler) RuntimeSettings(w http.ResponseWriter, r *http.Request) {
	cfg := h.deps.Config.AI
	queryCfg := cfg.EffectiveQueryConfig()
	profile := prompt.LookupModelContextProfile(queryCfg.Model, queryCfg.NumCtx)
	out := aiRuntimeSettingsResponse{
		Provider:         cfg.Provider,
		LLMModel:         cfg.Model,
		BaseURL:          cfg.BaseURL,
		BaseURLEffective: effectiveAIBaseURL(cfg),
		APIKeyConfigured: strings.TrimSpace(cfg.APIKey) != "",
	}
	out.QueryModelOverride = cfg.HasQueryOverride()
	out.QueryProvider = queryCfg.Provider
	out.QueryModel = queryCfg.Model
	out.QueryBaseURL = queryCfg.BaseURL
	out.QueryBaseURLEffective = effectiveAIBaseURL(queryCfg)
	out.QueryAPIKeyConfigured = strings.TrimSpace(queryCfg.APIKey) != ""
	out.QueryAPIKeyDedicated = strings.TrimSpace(cfg.QueryAPIKey) != ""
	if cfg.QueryHTTPTimeoutSeconds > 0 {
		out.QueryHTTPTimeoutSeconds = cfg.QueryHTTPTimeoutSeconds
	}
	out.MaxPromptInputRunes = queryCfg.MaxPromptInputRunes
	out.EffectiveMaxPromptRunes = prompt.EffectiveMaxPromptRunes(queryCfg, queryCfg.Model)
	out.ContextWindowTokens = profile.ContextWindowTokens
	out.ContextWindowSource = profile.Source
	embedCfg := cfg
	if cfg.DBManaged && h.deps.AIProviderStore != nil {
		embedCfg = h.deps.AIProviderStore.EffectiveConfigForEmbeddings()
	}
	if embedCfg.EmbeddingsConfigured() {
		out.EmbeddingsEnabled = true
		out.EmbeddingModel = strings.TrimSpace(embedCfg.EmbeddingModel)
		out.EmbeddingBaseURL = embedCfg.EmbeddingBaseURL
		out.EmbeddingBaseURLEffective = embeddingBaseURLEffectiveLabel(embedCfg)
		out.EmbeddingAPIKeyConfigured = strings.TrimSpace(embedCfg.EffectiveEmbeddingAPIKey()) != ""
		out.EmbeddingAPIKeyDedicated = strings.TrimSpace(embedCfg.EmbeddingAPIKey) != ""
	}
	if cfg.TranslationConfigured() {
		out.TranslationEnabled = true
		out.TranslationModel = strings.TrimSpace(cfg.TranslationModel)
		out.TranslationBaseURL = cfg.TranslationBaseURL
		out.TranslationBaseURLEffective = translationBaseURLEffectiveLabel(cfg)
		out.TranslationAPIKeyConfigured = strings.TrimSpace(cfg.EffectiveTranslationAPIKey()) != ""
		out.TranslationAPIKeyDedicated = strings.TrimSpace(cfg.TranslationAPIKey) != ""
		out.TranslationTargetLanguage = cfg.TranslationTargetLanguage
		out.TranslationTargetCode = cfg.TranslationTargetCode
	}
	out.DBManaged = cfg.DBManaged
	if cfg.DBManaged && h.deps.AIProviderStore != nil {
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
