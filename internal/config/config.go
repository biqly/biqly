// Package config provides application configuration loaded from environment variables.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/biqly/biqly/internal/env"
)

// Config holds all application configuration.
type Config struct {
	HTTP      HTTPConfig
	Logging   LoggingConfig
	Metadata  MetadataConfig
	Redis     RedisConfig
	Query     QueryConfig
	Security  SecurityConfig
	Services  ServicesConfig
	AI        AIConfig
	NATS      NATSConfig
	Jobs      JobsConfig
	Auth      AuthConfig
	Composite CompositeConfig
	PII       PIIConfig
	Drift     DriftConfig
	Mail      MailConfig
}

// DriftConfig controls the background schema drift check.
type DriftConfig struct {
	CheckInterval time.Duration
}

// MailConfig holds details to access the mail worker.
type MailConfig struct {
	ServiceURL    string
	InternalToken string
	FrontendURL   string
}

// PIIConfig controls automatic PII detection and role-based masking.
type PIIConfig struct {
	// Enabled toggles the whole PII subsystem (detection + masking).
	Enabled bool
	// DetectionThreshold is the minimum combined confidence (0–1) required
	// to flag a column as PII.
	DetectionThreshold float64
	// SampleDataLimit is the number of non-NULL sample values fetched per
	// column during a scan.
	SampleDataLimit int
	// AutoScanOnSync runs a PII scan after every metadata sync (can still be
	// suppressed per request via ?scan_pii=false).
	AutoScanOnSync bool
	// DefaultMaskingStrategy names the masking strategy applied to columns
	// without an explicit per-column strategy ("partial" is the only
	// built-in today).
	DefaultMaskingStrategy string
}

// CompositeConfig caps the size of composite semantic models. Zero disables a limit.
type CompositeConfig struct {
	// MaxComponents caps component models per composite.
	MaxComponents int
	// MaxCrossJoins caps active cross-model joins per composite.
	MaxCrossJoins int
	// MaxMergedFields caps combined dimensions + metrics of the resolved model.
	MaxMergedFields int
}

// AuthConfig wires the monolith to the standalone auth service.
// When Enabled is false, all /api/* routes fall back to the legacy
// APIKeyAuth middleware. When Enabled is true, /api/* routes verify a JWT
// against the auth service's public key, and routes can additionally enforce
// permission and datasource-access checks via the bimw.RequirePermission /
// bimw.RequireDatasourceAccess middleware.
type AuthConfig struct {
	Enabled       bool
	ServiceURL    string
	InternalToken string
}

// NATSConfig holds NATS JetStream settings for async AI jobs.
type NATSConfig struct {
	URL           string
	Stream        string
	Subject       string
	ConsumerGroup string
}

// JobsConfig toggles background AI job processing.
type JobsConfig struct {
	Enabled bool
}

// HTTPConfig holds HTTP server configuration.
type HTTPConfig struct {
	Host string
	Port int
	// CORSAllowedOrigins is the explicit set of origins allowed by CORS.
	// Empty means "no cross-origin requests" — the legacy wildcard
	// {"https://*", "http://*"} is no longer the default.
	CORSAllowedOrigins []string
	// HSTSEnabled toggles Strict-Transport-Security. Enable only when the
	// service is reachable exclusively over HTTPS (e.g. behind a TLS-terminating
	// gateway in production).
	HSTSEnabled bool
}

// LoggingConfig holds structured logger configuration.
type LoggingConfig struct {
	Level  string
	Format string
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
	// InternalAPIToken protects /internal/* peer-service endpoints.
	InternalAPIToken string
	// APIKey, when set, gates all /api/* routes via the APIKeyAuth middleware.
	// Clients must send either `X-API-Key: <key>` or `Authorization: Bearer <key>`.
	// When empty the API is left unauthenticated and a warning is logged at startup.
	APIKey string
	// MetricsAPIKey, when set, gates the /metrics endpoint with the same
	// scheme as APIKey. Scrapers (Prometheus, Datadog Agent) must send the
	// shared secret. Empty leaves /metrics public — preserved as default
	// for in-cluster Prometheus that already has NetworkPolicy isolation.
	MetricsAPIKey string
}

// ServicesConfig holds upstream service URLs used when the monolith runs as a BFF.
type ServicesConfig struct {
	CatalogURL string
	QueryURL   string
	AIURL      string
}

// QueryLLMConfig overrides the connection used by the NL-to-LogicalQuery path
// (typically a smarter model) without disturbing describe / metadata work,
// which prefers cheaper coverage on a smaller local model. All fields are
// optional: empty falls back to the matching base AIConfig connection setting.
// QueryLLMConfig overrides the connection used by the NL-to-LogicalQuery path
// (typically a smarter model) without disturbing describe / metadata work,
// which prefers cheaper coverage on a smaller local model. All fields are
// optional: empty falls back to the matching base AIConfig connection setting.
type QueryLLMConfig struct {
	Provider           string
	Model              string
	BaseURL            string
	APIKey             string
	HTTPTimeoutSeconds int
}

// EmbeddingConfig groups the settings for vector-based table retrieval and the
// embed-metadata pipeline.
type EmbeddingConfig struct {
	// Model names the embeddings model used for vector-based table retrieval.
	// Empty disables the embedder; the router uses keyword-only scoring.
	Model string
	// BaseURL, when set, is the OpenAI-compatible base for POST …/embeddings.
	// Empty means use the LLM BaseURL, then the provider default for OpenAI.
	BaseURL string
	// APIKey, when set, is used only for embedding requests. Empty falls back
	// to the LLM APIKey.
	APIKey string
	// HTTPTimeoutSeconds overrides the provider HTTP timeout for embedding
	// requests, which can run longer than chat completions when refreshing an
	// entire catalog.
	HTTPTimeoutSeconds int
	// Weight scales the cosine-similarity contribution to the hybrid
	// table-routing score. 0 disables the boost even when embeddings are
	// present; 30 (default) makes a perfect match comparable to a fully matched
	// table-name token.
	Weight float64
	// DenySchemas lists schema names whose tables MUST NOT be embedded —
	// table/column identifiers will not be sent to an external embedding API.
	// Use for schemas holding regulated data.
	DenySchemas []string
	// DenyTables lists "schema.table" pairs to exclude from embedding even when
	// the schema is otherwise allowed.
	DenyTables []string
}

// TranslationConfig groups the post-processing translation/normalization layer
// for AI-generated metadata descriptions.
type TranslationConfig struct {
	// Model enables a post-processing translation/normalization layer for
	// AI-generated metadata descriptions.
	Model string
	// BaseURL is the OpenAI-compatible base URL for the translation model.
	BaseURL string
	// APIKey is used for translation requests. Empty falls back to the LLM APIKey.
	APIKey string
	// TargetLanguage is the human-readable target language name.
	TargetLanguage string
	// TargetCode is the BCP-47/ISO target language code.
	TargetCode string
	// HTTPTimeoutSeconds is the HTTP timeout for translation requests.
	HTTPTimeoutSeconds int
}

// RoutingConfig groups the hybrid table-router tuning knobs and the caps used
// when synthesizing semantic models from raw introspected metadata.
type RoutingConfig struct {
	// LexiconPath overrides embedded NL token synonyms and intent vocabulary (JSON).
	LexiconPath string
	// WeightsPath overrides table-routing score weights and boost rules (JSON).
	WeightsPath string
	// MaxDimensions caps dimensions in auto-generated semantic models (prompt size).
	MaxDimensions int
	// MaxMetrics caps metrics in auto-generated semantic models.
	MaxMetrics int
	// MaxColumnsPerTable caps ranked columns per wide table during auto-routing.
	MaxColumnsPerTable int
	// MaxDateGrainExtras caps date-grain dimension variants per date column.
	MaxDateGrainExtras int
	// SlimNumericMetrics when true emits only sum_/max_ per numeric column (not avg_/min_).
	SlimNumericMetrics bool
}

// AmbiguityConfig groups the pre-LLM semantic ambiguity clarification knobs.
type AmbiguityConfig struct {
	// CheckEnabled toggles pre-LLM semantic ambiguity clarification.
	CheckEnabled bool
	// ConfidenceThreshold is the minimum interpretation confidence to count toward clarification.
	ConfidenceThreshold float64
	// MaxOptions caps the selectable clarification options returned to the user.
	MaxOptions int
	// LLMEnabled enables the provider-backed ambiguity fallback after deterministic checks pass.
	LLMEnabled bool
}

// AIConnectionConfig groups shared LLM HTTP connection settings. Provider/model
// selection is sourced from ai_providers / ai_models via ProviderStore; only
// HTTPTimeoutSeconds and RateLimitPerMinute are environment-driven operational knobs.
type AIConnectionConfig struct {
	Provider           string
	APIKey             string
	BaseURL            string
	Model              string
	HTTPTimeoutSeconds int
	RateLimitPerMinute int
}

// AIGenerationConfig groups token/generation tuning shared across chat paths.
type AIGenerationConfig struct {
	MaxTokens           int
	Temperature         float64
	TopP                float64
	NumCtx              int
	MaxPromptInputRunes int
	MaxRetries          int
	MultiCandidateCount int
}

// AIDescribeConfig groups sampling limits for the AI Describe metadata path.
type AIDescribeConfig struct {
	MaxCellRunes  int
	MaxSampleRows int
}

// AICacheConfig groups AI query response cache tuning.
type AICacheConfig struct {
	ResponseTTLSeconds int
}

// AIConfig holds AI provider configuration as purpose-based sub-configs only.
type AIConfig struct {
	Connection  AIConnectionConfig
	Generation  AIGenerationConfig
	Describe    AIDescribeConfig
	Cache       AICacheConfig
	Query       QueryLLMConfig
	Embedding   EmbeddingConfig
	Translation TranslationConfig
	Routing     RoutingConfig
	Ambiguity   AmbiguityConfig
}

// Load reads configuration from environment variables.
//
//nolint:funlen
func Load() (*Config, error) {
	cfg := &Config{
		Drift: DriftConfig{
			CheckInterval: getEnvAsDuration("BI_DRIFT_CHECK_INTERVAL", 6*time.Hour),
		},
		Mail: MailConfig{
			ServiceURL:    getEnv("BI_AUTH_MAIL_SERVICE_URL", "http://localhost:8890"),
			InternalToken: getEnv("BI_AUTH_MAIL_INTERNAL_TOKEN", ""),
			FrontendURL:   getEnv("BI_AUTH_FRONTEND_BASE_URL", "http://localhost:3333"),
		},
		HTTP: HTTPConfig{
			Host:               getEnv("BI_HTTP_HOST", "0.0.0.0"),
			Port:               getEnvAsInt("BI_HTTP_PORT", 8888),
			CORSAllowedOrigins: splitCSV(getEnv("BI_CORS_ALLOWED_ORIGINS", "")),
			HSTSEnabled:        getEnvAsBool("BI_HSTS_ENABLED", env.HSTSEnabledDefault()),
		},
		Logging: LoggingConfig{
			Level:  strings.ToLower(strings.TrimSpace(getEnv("BI_LOG_LEVEL", "info"))),
			Format: strings.ToLower(strings.TrimSpace(getEnv("BI_LOG_FORMAT", "json"))),
		},
		Metadata: MetadataConfig{
			DSN: getEnv("BI_METADATA_DB_DSN", "postgres://localhost:5432/bi_metadata?sslmode=disable"),
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
			EncryptionKey:    getEnv("BI_ENCRYPTION_KEY", "change-this-to-a-secure-32-byte-key!!"),
			AdminAPIKey:      getEnv("BI_ADMIN_API_KEY", ""),
			InternalAPIToken: getEnv("BI_INTERNAL_API_TOKEN", ""),
			APIKey:           getEnv("BI_API_KEY", ""),
			MetricsAPIKey:    getEnv("BI_METRICS_API_KEY", ""),
		},
		Services: ServicesConfig{
			CatalogURL: strings.TrimRight(getEnv("BI_CATALOG_SERVICE_URL", ""), "/"),
			QueryURL:   strings.TrimRight(getEnv("BI_QUERY_SERVICE_URL", ""), "/"),
			AIURL:      strings.TrimRight(getEnv("BI_AI_SERVICE_URL", ""), "/"),
		},
		AI: AIConfig{
			// Connection/model selection is intentionally NOT read from the environment —
			// it comes only from ai_providers / ai_models via ProviderStore.
			Connection: AIConnectionConfig{
				HTTPTimeoutSeconds: getEnvAsInt("BI_AI_HTTP_TIMEOUT_SECONDS", 300),
				RateLimitPerMinute: getEnvAsInt("BI_AI_RATE_LIMIT_PER_MINUTE", 20),
			},
			Generation: AIGenerationConfig{
				MaxPromptInputRunes: getEnvAsInt("BI_AI_MAX_PROMPT_RUNES", 80000),
				MaxRetries:          getEnvAsInt("BI_AI_MAX_RETRIES", 2),
				MultiCandidateCount: getEnvAsInt("BI_AI_MULTI_CANDIDATE_COUNT", 1),
			},
			Describe: AIDescribeConfig{
				MaxCellRunes:  getEnvAsInt("BI_AI_DESCRIBE_MAX_CELL_RUNES", 500),
				MaxSampleRows: getEnvAsInt("BI_AI_DESCRIBE_MAX_SAMPLE_ROWS", 12),
			},
			Cache: AICacheConfig{
				ResponseTTLSeconds: getEnvAsInt("BI_AI_RESPONSE_CACHE_TTL", 3600),
			},
			Translation: TranslationConfig{
				TargetLanguage: getEnv(
					"BI_AI_TRANSLATION_TARGET_LANGUAGE",
					"Turkish",
				),
				TargetCode:         getEnv("BI_AI_TRANSLATION_TARGET_CODE", "tr"),
				HTTPTimeoutSeconds: getEnvAsInt("BI_AI_TRANSLATION_HTTP_TIMEOUT_SECONDS", 120),
			},
			Embedding: EmbeddingConfig{
				HTTPTimeoutSeconds: getEnvAsInt(
					"BI_AI_EMBEDDING_HTTP_TIMEOUT_SECONDS",
					getEnvAsInt("BI_AI_HTTP_TIMEOUT_SECONDS", 600),
				),
				Weight:      getEnvAsFloat("BI_AI_EMBEDDING_WEIGHT", 30.0),
				DenySchemas: splitCSV(getEnv("BI_AI_EMBEDDING_DENY_SCHEMAS", "")),
				DenyTables:  splitCSV(getEnv("BI_AI_EMBEDDING_DENY_TABLES", "")),
			},
			Routing: RoutingConfig{
				LexiconPath:        getEnv("BI_AI_ROUTING_LEXICON_PATH", ""),
				WeightsPath:        getEnv("BI_AI_ROUTING_WEIGHTS_PATH", ""),
				MaxDimensions:      getEnvAsInt("BI_AI_ROUTE_MAX_DIMENSIONS", 0),
				MaxMetrics:         getEnvAsInt("BI_AI_ROUTE_MAX_METRICS", 0),
				MaxColumnsPerTable: getEnvAsInt("BI_AI_ROUTE_MAX_COLUMNS_PER_TABLE", 0),
				MaxDateGrainExtras: getEnvAsInt("BI_AI_ROUTE_MAX_DATE_GRAIN_EXTRAS", 0),
				SlimNumericMetrics: getEnvAsBool("BI_AI_ROUTE_SLIM_NUMERIC_METRICS", true),
			},
			Ambiguity: AmbiguityConfig{
				CheckEnabled: getEnvAsBool("BI_AI_AMBIGUITY_CHECK_ENABLED", true),
				ConfidenceThreshold: getEnvAsFloat(
					"BI_AI_AMBIGUITY_CONFIDENCE_THRESHOLD",
					0.70,
				),
				MaxOptions: getEnvAsInt("BI_AI_AMBIGUITY_MAX_OPTIONS", 5),
				LLMEnabled: getEnvAsBool("BI_AI_AMBIGUITY_LLM_ENABLED", false),
			},
		},
		NATS: NATSConfig{
			URL:           getEnv("BI_NATS_URL", ""),
			Stream:        getEnv("BI_NATS_STREAM", "BIQLY_AI_JOBS"),
			Subject:       getEnv("BI_NATS_SUBJECT", "biqly.ai.jobs"),
			ConsumerGroup: getEnv("BI_NATS_CONSUMER_GROUP", "biqly-ai-workers"),
		},
		Jobs: JobsConfig{
			Enabled: getEnvAsBool("BI_AI_JOBS_ENABLED", true),
		},
		Auth: AuthConfig{
			Enabled:       getEnvAsBool("BI_AUTH_ENABLED", false),
			ServiceURL:    strings.TrimRight(getEnv("BI_AUTH_SERVICE_URL", ""), "/"),
			InternalToken: getEnv("BI_AUTH_INTERNAL_TOKEN", ""),
		},
		Composite: CompositeConfig{
			MaxComponents:   getEnvAsInt("BI_COMPOSITE_MAX_COMPONENTS", 8),
			MaxCrossJoins:   getEnvAsInt("BI_COMPOSITE_MAX_CROSS_JOINS", 16),
			MaxMergedFields: getEnvAsInt("BI_COMPOSITE_MAX_MERGED_FIELDS", 300),
		},
		PII: PIIConfig{
			Enabled:                getEnvAsBool("BI_PII_ENABLED", true),
			DetectionThreshold:     getEnvAsFloat("BI_PII_DETECTION_THRESHOLD", 0.6),
			SampleDataLimit:        getEnvAsInt("BI_PII_SAMPLE_DATA_LIMIT", 50),
			AutoScanOnSync:         getEnvAsBool("BI_PII_AUTO_SCAN_ON_SYNC", true),
			DefaultMaskingStrategy: getEnv("BI_PII_DEFAULT_MASKING_STRATEGY", "partial"),
		},
	}

	if cfg.Metadata.DSN == "" {
		return nil, errors.New("BI_METADATA_DB_DSN is required")
	}
	if cfg.Security.EncryptionKey == "" {
		return nil, errors.New("BI_ENCRYPTION_KEY is required")
	}
	if cfg.Security.EncryptionKey == "change-this-to-a-secure-32-byte-key!!" {
		return nil, errors.New("BI_ENCRYPTION_KEY must be changed from its default value")
	}
	if err := validateFloatRange("BI_PII_DETECTION_THRESHOLD", cfg.PII.DetectionThreshold, 0, 1); err != nil {
		return nil, err
	}
	if err := validateFloatRange("BI_AI_AMBIGUITY_CONFIDENCE_THRESHOLD", cfg.AI.Ambiguity.ConfidenceThreshold, 0, 1); err != nil {
		return nil, err
	}
	if err := validateFloatRange("BI_AI_EMBEDDING_WEIGHT", cfg.AI.Embedding.Weight, 0, 100); err != nil {
		return nil, err
	}
	if env.IsProduction() && !cfg.Auth.Enabled {
		return nil, errors.New("BI_AUTH_ENABLED must be true in production")
	}

	return cfg, nil
}

// AIHTTPTimeout returns the provider HTTP timeout as time.Duration.
func (c AIConfig) AIHTTPTimeout() time.Duration {
	seconds := c.Connection.HTTPTimeoutSeconds
	if seconds <= 0 {
		seconds = 300
	}
	return time.Duration(seconds) * time.Second
}

// EmbeddingHTTPTimeout returns the HTTP timeout for embedding requests.
func (c AIConfig) EmbeddingHTTPTimeout() time.Duration {
	seconds := c.Embedding.HTTPTimeoutSeconds
	if seconds <= 0 {
		seconds = c.Connection.HTTPTimeoutSeconds
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
	if s := strings.TrimSpace(c.Query.Provider); s != "" {
		out.Connection.Provider = s
	}
	if s := strings.TrimSpace(c.Query.Model); s != "" {
		out.Connection.Model = s
	}
	if s := strings.TrimSpace(c.Query.BaseURL); s != "" {
		out.Connection.BaseURL = s
	}
	if s := strings.TrimSpace(c.Query.APIKey); s != "" {
		out.Connection.APIKey = s
	}
	if c.Query.HTTPTimeoutSeconds > 0 {
		out.Connection.HTTPTimeoutSeconds = c.Query.HTTPTimeoutSeconds
	}
	return out
}

// QueryLLMConfigured reports whether the NL-to-LogicalQuery path (and golden eval) can call an LLM.
// Uses EffectiveQueryConfig. Model is required. API key may be omitted when BaseURL targets a
// keyless local OpenAI-compatible server (Ollama, llama-server, etc.).
func (c AIConfig) QueryLLMConfigured() bool {
	cfg := c.EffectiveQueryConfig()
	if strings.TrimSpace(cfg.Connection.Model) == "" {
		return false
	}
	if strings.TrimSpace(cfg.Connection.APIKey) != "" {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(cfg.Connection.Provider)) {
	case "", "openai", "openai-compatible":
		return strings.TrimSpace(cfg.Connection.BaseURL) != ""
	default:
		return false
	}
}

// HasQueryOverride reports whether any BI_AI_QUERY_* knob is set. When false,
// callers should reuse the base AI provider instead of constructing a second
// one that points at the same endpoint and model.
func (c AIConfig) HasQueryOverride() bool {
	return strings.TrimSpace(c.Query.Provider) != "" ||
		strings.TrimSpace(c.Query.Model) != "" ||
		strings.TrimSpace(c.Query.BaseURL) != "" ||
		strings.TrimSpace(c.Query.APIKey) != "" ||
		c.Query.HTTPTimeoutSeconds > 0
}

// EffectiveEmbeddingAPIKey returns BI_AI_EMBEDDING_API_KEY when set, otherwise BI_AI_API_KEY.
func (c AIConfig) EffectiveEmbeddingAPIKey() string {
	if s := strings.TrimSpace(c.Embedding.APIKey); s != "" {
		return s
	}
	return c.Connection.APIKey
}

// EffectiveEmbeddingBaseURL resolves the embeddings HTTP base (no trailing path).
// Order: BI_AI_EMBEDDING_BASE_URL, then BI_AI_BASE_URL, then OpenAI default when provider is OpenAI-compatible.
func (c AIConfig) EffectiveEmbeddingBaseURL() string {
	if s := strings.TrimSpace(c.Embedding.BaseURL); s != "" {
		return strings.TrimRight(s, "/")
	}
	if s := strings.TrimSpace(c.Connection.BaseURL); s != "" {
		return strings.TrimRight(s, "/")
	}
	p := strings.ToLower(strings.TrimSpace(c.Connection.Provider))
	switch p {
	case "", "openai", "openai-compatible":
		return "https://api.openai.com/v1"
	default:
		return ""
	}
}

// EmbeddingsConfigured reports whether vector table routing / embed-metadata can call an embeddings API.
func (c AIConfig) EmbeddingsConfigured() bool {
	if strings.TrimSpace(c.Embedding.Model) == "" {
		return false
	}
	if strings.TrimSpace(c.EffectiveEmbeddingAPIKey()) == "" {
		return false
	}
	return strings.TrimSpace(c.EffectiveEmbeddingBaseURL()) != ""
}

// EffectiveTranslationAPIKey returns BI_AI_TRANSLATION_API_KEY when set, otherwise BI_AI_API_KEY.
func (c AIConfig) EffectiveTranslationAPIKey() string {
	if s := strings.TrimSpace(c.Translation.APIKey); s != "" {
		return s
	}
	return c.Connection.APIKey
}

// EffectiveTranslationBaseURL resolves the OpenAI-compatible translation base URL.
func (c AIConfig) EffectiveTranslationBaseURL() string {
	if s := strings.TrimSpace(c.Translation.BaseURL); s != "" {
		return strings.TrimRight(s, "/")
	}
	return strings.TrimRight(strings.TrimSpace(c.Connection.BaseURL), "/")
}

// TranslationHTTPTimeout returns the HTTP timeout for translation requests.
func (c AIConfig) TranslationHTTPTimeout() time.Duration {
	seconds := c.Translation.HTTPTimeoutSeconds
	if seconds <= 0 {
		seconds = c.Connection.HTTPTimeoutSeconds
	}
	if seconds <= 0 {
		seconds = 120
	}
	return time.Duration(seconds) * time.Second
}

// TranslationConfigured reports whether metadata description translation is enabled.
func (c AIConfig) TranslationConfigured() bool {
	if strings.TrimSpace(c.Translation.Model) == "" {
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

// splitCSV parses a comma-separated string into a slice of trimmed,
// non-empty values. Returns nil for an empty input so callers can
// distinguish "unset" from "set to empty list".
func splitCSV(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func getEnvAsInt(key string, defaultVal int) int {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		slog.Warn("ignoring invalid int env var; using default",
			"key", key, "value", valStr, "default", defaultVal, "error", err,
		)
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
		slog.Warn("ignoring invalid float env var; using default",
			"key", key, "value", valStr, "default", defaultVal, "error", err,
		)
		return defaultVal
	}
	return val
}

func validateFloatRange(key string, val, minVal, maxVal float64) error {
	if val < minVal || val > maxVal {
		return fmt.Errorf("%s must be between %g and %g, got %g", key, minVal, maxVal, val)
	}
	return nil
}

func getEnvAsBool(key string, defaultVal bool) bool {
	valStr := strings.TrimSpace(os.Getenv(key))
	if valStr == "" {
		return defaultVal
	}
	switch strings.ToLower(valStr) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return defaultVal
	}
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

func getEnvAsDuration(key string, defaultVal time.Duration) time.Duration {
	valStr := os.Getenv(key)
	if valStr == "" {
		return defaultVal
	}
	val, err := time.ParseDuration(valStr)
	if err != nil {
		slog.Warn("ignoring invalid duration env var; using default",
			"key", key, "value", valStr, "default", defaultVal, "error", err,
		)
		return defaultVal
	}
	return val
}
