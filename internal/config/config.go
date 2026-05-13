// Package config provides application configuration loaded from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds all application configuration.
type Config struct {
	HTTP     HTTPConfig
	Metadata MetadataConfig
	Redis    RedisConfig
	Query    QueryConfig
	Security SecurityConfig
	AI       AIConfig
}

// HTTPConfig holds HTTP server configuration.
type HTTPConfig struct {
	Host string
	Port int
}

// MetadataConfig holds metadata database connection configuration.
type MetadataConfig struct {
	DSN string
}

// RedisConfig holds Redis connection configuration.
type RedisConfig struct {
	DSN string
}

// QueryConfig holds query execution limits and timeouts.
type QueryConfig struct {
	TimeoutSeconds    int
	MaxRows           int
	MaxRuntimeSeconds int
	// HistoryListLimit caps rows returned by the query history list API (newest first).
	HistoryListLimit int
	// EvalRunsListLimit caps rows returned by the AI eval runs list admin API.
	EvalRunsListLimit int
}

// SecurityConfig holds encryption key settings.
type SecurityConfig struct {
	EncryptionKey string
	AdminAPIKey   string
}

// AIConfig holds AI provider configuration.
type AIConfig struct {
	Provider           string
	APIKey             string
	BaseURL            string
	Model              string
	MaxTokens          int
	Temperature        float64
	TopP               float64
	NumCtx             int
	HTTPTimeoutSeconds int
	RateLimitPerMinute int
	// MaxPromptInputRunes caps the semantic-model section of NL→query prompts (~4 chars/rune ≈ 1 token).
	MaxPromptInputRunes int
	// DescribeMaxCellRunes truncates each sampled cell in AI Describe before sending to the LLM.
	DescribeMaxCellRunes int
	// DescribeMaxSampleRows is a hard cap on rows sampled for Describe (wide tables × many columns).
	DescribeMaxSampleRows int
	// TranslationModel enables a post-processing translation/normalization layer
	// for AI-generated metadata descriptions.
	TranslationModel string
	// TranslationBaseURL is the OpenAI-compatible base URL for the translation model.
	TranslationBaseURL string
	// TranslationAPIKey is used for translation requests. Empty falls back to APIKey.
	TranslationAPIKey string
	// TranslationTargetLanguage is the human-readable target language name.
	TranslationTargetLanguage string
	// TranslationTargetCode is the BCP-47/ISO target language code.
	TranslationTargetCode string
	// TranslationHTTPTimeoutSeconds is the HTTP timeout for translation requests.
	TranslationHTTPTimeoutSeconds int
	// MaxRetries caps how many times the validator can re-prompt the model after parse/validation failure.
	MaxRetries int
	// MultiCandidateCount enables self-consistency voting: when >1, the service
	// generates this many candidates per question (stepped temperatures) and
	// selects the majority. 1 = single-shot (default).
	MultiCandidateCount int
	// EmbeddingModel names the embeddings model used for vector-based table
	// retrieval. Empty disables the embedder; the router uses keyword-only scoring.
	EmbeddingModel string
	// EmbeddingBaseURL, when set, is the OpenAI-compatible base for POST …/embeddings.
	// Empty means use BaseURL (LLM), then provider default for OpenAI.
	EmbeddingBaseURL string
	// EmbeddingAPIKey, when set, is used only for embedding requests.
	// Empty means use APIKey (LLM).
	EmbeddingAPIKey string
	// EmbeddingHTTPTimeoutSeconds overrides provider HTTP timeout for
	// embedding requests, which can run longer than chat completions when
	// refreshing an entire catalog.
	EmbeddingHTTPTimeoutSeconds int
	// EmbeddingWeight scales the cosine-similarity contribution to the
	// hybrid table-routing score. 0 disables the boost even when embeddings
	// are present; 30 (default) makes a perfect match comparable to a fully
	// matched table-name token.
	EmbeddingWeight float64

	// Query* fields let the NL-to-LogicalQuery path use a different model
	// (typically a smarter one) without disturbing describe / metadata work,
	// which prefers cheaper coverage on a smaller local model. All four are
	// optional: empty falls back to the matching base BI_AI_* setting.
	QueryProvider           string
	QueryModel              string
	QueryBaseURL            string
	QueryAPIKey             string
	QueryHTTPTimeoutSeconds int
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	cfg := &Config{
		HTTP: HTTPConfig{
			Host: getEnv("BI_HTTP_HOST", "0.0.0.0"),
			Port: getEnvAsInt("BI_HTTP_PORT", 8888),
		},
		Metadata: MetadataConfig{
			DSN: getEnv("BI_METADATA_DB_DSN", "postgres://bi_user:bi_password@localhost:5432/bi_metadata?sslmode=disable"),
		},
		Redis: RedisConfig{
			DSN: getEnv("BI_REDIS_DSN", "redis://localhost:6379"),
		},
		Query: QueryConfig{
			TimeoutSeconds:    getEnvAsInt("BI_QUERY_TIMEOUT_SECONDS", 30),
			MaxRows:           getEnvAsInt("BI_QUERY_MAX_ROWS", 10000),
			MaxRuntimeSeconds: getEnvAsInt("BI_QUERY_MAX_RUNTIME_SECONDS", 60),
			HistoryListLimit:  getEnvAsInt("BI_QUERY_HISTORY_LIST_LIMIT", 100),
			EvalRunsListLimit: getEnvAsInt("BI_EVAL_RUNS_LIST_LIMIT", 50),
		},
		Security: SecurityConfig{
			EncryptionKey: getEnv("BI_ENCRYPTION_KEY", "change-this-to-a-secure-32-byte-key!!"),
			AdminAPIKey:   getEnv("BI_ADMIN_API_KEY", ""),
		},
		AI: AIConfig{
			Provider:              getEnv("BI_AI_PROVIDER", "openai"),
			APIKey:                getEnv("BI_AI_API_KEY", ""),
			BaseURL:               getEnv("BI_AI_BASE_URL", ""),
			Model:                 getEnv("BI_AI_MODEL", "gpt-4o"),
			MaxTokens:             getEnvAsInt("BI_AI_MAX_TOKENS", 4096),
			Temperature:           getEnvAsFloat("BI_AI_TEMPERATURE", 0.0),
			TopP:                  getEnvAsFloat("BI_AI_TOP_P", 0.0),
			NumCtx:                getEnvAsInt("BI_AI_NUM_CTX", 0),
			HTTPTimeoutSeconds:    getEnvAsInt("BI_AI_HTTP_TIMEOUT_SECONDS", 300),
			RateLimitPerMinute:    getEnvAsInt("BI_AI_RATE_LIMIT_PER_MINUTE", 20),
			MaxPromptInputRunes:   getEnvAsInt("BI_AI_MAX_PROMPT_RUNES", 80000),
			DescribeMaxCellRunes:  getEnvAsInt("BI_AI_DESCRIBE_MAX_CELL_RUNES", 500),
			DescribeMaxSampleRows: getEnvAsInt("BI_AI_DESCRIBE_MAX_SAMPLE_ROWS", 12),
			TranslationModel:      getEnv("BI_AI_TRANSLATION_MODEL", ""),
			TranslationBaseURL:    getEnv("BI_AI_TRANSLATION_BASE_URL", ""),
			TranslationAPIKey:     getEnv("BI_AI_TRANSLATION_API_KEY", ""),
			TranslationTargetLanguage: getEnv(
				"BI_AI_TRANSLATION_TARGET_LANGUAGE",
				"Turkish",
			),
			TranslationTargetCode:         getEnv("BI_AI_TRANSLATION_TARGET_CODE", "tr"),
			TranslationHTTPTimeoutSeconds: getEnvAsInt("BI_AI_TRANSLATION_HTTP_TIMEOUT_SECONDS", 120),
			MaxRetries:                    getEnvAsInt("BI_AI_MAX_RETRIES", 2),
			MultiCandidateCount:           getEnvAsInt("BI_AI_MULTI_CANDIDATE_COUNT", 1),
			EmbeddingModel:                getEnv("BI_AI_EMBEDDING_MODEL", ""),
			EmbeddingBaseURL:              getEnv("BI_AI_EMBEDDING_BASE_URL", ""),
			EmbeddingAPIKey:               getEnv("BI_AI_EMBEDDING_API_KEY", ""),
			EmbeddingHTTPTimeoutSeconds: getEnvAsInt(
				"BI_AI_EMBEDDING_HTTP_TIMEOUT_SECONDS",
				getEnvAsInt("BI_AI_HTTP_TIMEOUT_SECONDS", 600),
			),
			EmbeddingWeight:         getEnvAsFloat("BI_AI_EMBEDDING_WEIGHT", 30.0),
			QueryProvider:           getEnv("BI_AI_QUERY_PROVIDER", ""),
			QueryModel:              getEnv("BI_AI_QUERY_MODEL", ""),
			QueryBaseURL:            getEnv("BI_AI_QUERY_BASE_URL", ""),
			QueryAPIKey:             getEnv("BI_AI_QUERY_API_KEY", ""),
			QueryHTTPTimeoutSeconds: getEnvAsInt("BI_AI_QUERY_HTTP_TIMEOUT_SECONDS", 0),
		},
	}

	if cfg.Metadata.DSN == "" {
		return nil, fmt.Errorf("BI_METADATA_DB_DSN is required")
	}
	if cfg.Security.EncryptionKey == "" {
		return nil, fmt.Errorf("BI_ENCRYPTION_KEY is required")
	}
	if cfg.Security.EncryptionKey == "change-this-to-a-secure-32-byte-key!!" {
		return nil, fmt.Errorf("BI_ENCRYPTION_KEY must be changed from its default value")
	}

	return cfg, nil
}

// AIHTTPTimeout returns the provider HTTP timeout as time.Duration.
func (c AIConfig) AIHTTPTimeout() time.Duration {
	seconds := c.HTTPTimeoutSeconds
	if seconds <= 0 {
		seconds = 300
	}
	return time.Duration(seconds) * time.Second
}

// EmbeddingHTTPTimeout returns the HTTP timeout for embedding requests.
func (c AIConfig) EmbeddingHTTPTimeout() time.Duration {
	seconds := c.EmbeddingHTTPTimeoutSeconds
	if seconds <= 0 {
		seconds = c.HTTPTimeoutSeconds
	}
	if seconds <= 0 {
		seconds = 300
	}
	return time.Duration(seconds) * time.Second
}

// AIRequestTimeout returns the server-side request budget for AI endpoints.
func (c AIConfig) AIRequestTimeout() time.Duration {
	timeout := c.AIHTTPTimeout()
	if embeddingTimeout := c.EmbeddingHTTPTimeout(); embeddingTimeout > timeout {
		timeout = embeddingTimeout
	}
	if translationTimeout := c.TranslationHTTPTimeout(); translationTimeout > timeout {
		timeout = translationTimeout
	}
	return timeout + 30*time.Second
}

// EffectiveQueryConfig returns an AIConfig whose Provider/APIKey/BaseURL/Model
// are overridden by BI_AI_QUERY_* when those are set, so the NL-to-LogicalQuery
// path can route to a smarter (or just different) model than describe /
// metadata work. Every other tuning knob (max tokens, retries, etc.) is shared
// with the base config — this is a transport override, not a separate runtime.
//
// HasQueryOverride() reports whether any field was overridden so callers can
// avoid building a duplicate provider when nothing changed.
func (c AIConfig) EffectiveQueryConfig() AIConfig {
	out := c
	if s := strings.TrimSpace(c.QueryProvider); s != "" {
		out.Provider = s
	}
	if s := strings.TrimSpace(c.QueryModel); s != "" {
		out.Model = s
	}
	if s := strings.TrimSpace(c.QueryBaseURL); s != "" {
		out.BaseURL = s
	}
	if s := strings.TrimSpace(c.QueryAPIKey); s != "" {
		out.APIKey = s
	}
	if c.QueryHTTPTimeoutSeconds > 0 {
		out.HTTPTimeoutSeconds = c.QueryHTTPTimeoutSeconds
	}
	return out
}

// HasQueryOverride reports whether any BI_AI_QUERY_* knob is set. When false,
// callers should reuse the base AI provider instead of constructing a second
// one that points at the same endpoint and model.
func (c AIConfig) HasQueryOverride() bool {
	return strings.TrimSpace(c.QueryProvider) != "" ||
		strings.TrimSpace(c.QueryModel) != "" ||
		strings.TrimSpace(c.QueryBaseURL) != "" ||
		strings.TrimSpace(c.QueryAPIKey) != "" ||
		c.QueryHTTPTimeoutSeconds > 0
}

// EffectiveEmbeddingAPIKey returns BI_AI_EMBEDDING_API_KEY when set, otherwise BI_AI_API_KEY.
func (c AIConfig) EffectiveEmbeddingAPIKey() string {
	if s := strings.TrimSpace(c.EmbeddingAPIKey); s != "" {
		return s
	}
	return c.APIKey
}

// EffectiveEmbeddingBaseURL resolves the embeddings HTTP base (no trailing path).
// Order: BI_AI_EMBEDDING_BASE_URL, then BI_AI_BASE_URL, then OpenAI default when provider is OpenAI-compatible.
func (c AIConfig) EffectiveEmbeddingBaseURL() string {
	if s := strings.TrimSpace(c.EmbeddingBaseURL); s != "" {
		return strings.TrimRight(s, "/")
	}
	if s := strings.TrimSpace(c.BaseURL); s != "" {
		return strings.TrimRight(s, "/")
	}
	p := strings.ToLower(strings.TrimSpace(c.Provider))
	switch p {
	case "", "openai", "openai-compatible":
		return "https://api.openai.com/v1"
	default:
		return ""
	}
}

// EmbeddingsConfigured reports whether vector table routing / embed-metadata can call an embeddings API.
func (c AIConfig) EmbeddingsConfigured() bool {
	if strings.TrimSpace(c.EmbeddingModel) == "" {
		return false
	}
	if strings.TrimSpace(c.EffectiveEmbeddingAPIKey()) == "" {
		return false
	}
	return strings.TrimSpace(c.EffectiveEmbeddingBaseURL()) != ""
}

// EffectiveTranslationAPIKey returns BI_AI_TRANSLATION_API_KEY when set, otherwise BI_AI_API_KEY.
func (c AIConfig) EffectiveTranslationAPIKey() string {
	if s := strings.TrimSpace(c.TranslationAPIKey); s != "" {
		return s
	}
	return c.APIKey
}

// EffectiveTranslationBaseURL resolves the OpenAI-compatible translation base URL.
func (c AIConfig) EffectiveTranslationBaseURL() string {
	if s := strings.TrimSpace(c.TranslationBaseURL); s != "" {
		return strings.TrimRight(s, "/")
	}
	return strings.TrimRight(strings.TrimSpace(c.BaseURL), "/")
}

// TranslationHTTPTimeout returns the HTTP timeout for translation requests.
func (c AIConfig) TranslationHTTPTimeout() time.Duration {
	seconds := c.TranslationHTTPTimeoutSeconds
	if seconds <= 0 {
		seconds = c.HTTPTimeoutSeconds
	}
	if seconds <= 0 {
		seconds = 120
	}
	return time.Duration(seconds) * time.Second
}

// TranslationConfigured reports whether metadata description translation is enabled.
func (c AIConfig) TranslationConfigured() bool {
	if strings.TrimSpace(c.TranslationModel) == "" {
		return false
	}
	return strings.TrimSpace(c.EffectiveTranslationBaseURL()) != ""
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvAsInt(key string, defaultVal int) int {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return defaultVal
	}
	return val
}

func getEnvAsFloat(key string, defaultVal float64) float64 {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		return defaultVal
	}
	return val
}

// HTTPAddr returns the full HTTP listen address.
func (c *Config) HTTPAddr() string {
	return fmt.Sprintf("%s:%d", c.HTTP.Host, c.HTTP.Port)
}

// QueryTimeout returns the query timeout as time.Duration.
func (c *Config) QueryTimeout() time.Duration {
	return time.Duration(c.Query.TimeoutSeconds) * time.Second
}

// MaxQueryRuntime returns the maximum query runtime as time.Duration.
func (c *Config) MaxQueryRuntime() time.Duration {
	return time.Duration(c.Query.MaxRuntimeSeconds) * time.Second
}
