// Package config provides application configuration loaded from environment variables.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"time"

	"github.com/biqly/biqly/internal/env"
)

// Config holds all application configuration.
type Config struct {
	HTTP        HTTPConfig
	Logging     LoggingConfig
	Metadata    MetadataConfig
	Redis       RedisConfig
	Query       QueryConfig
	Security    SecurityConfig
	Services    ServicesConfig
	AI          AIConfig
	NATS        NATSConfig
	Jobs        JobsConfig
	Auth        AuthConfig
	Composite   CompositeConfig
	PII         PIIConfig
	Drift       DriftConfig
	Mail        MailConfig
	Agent       AgentConfig
	WebAgent    WebAgentConfig
	PublicShare PublicShareConfig
	// DeploymentMode is the deployment posture: "cloud" (default), "private",
	// or "airgapped". Airgapped fails closed on external LLM/embedding egress:
	// provider endpoints must resolve to private, in-cluster hosts.
	DeploymentMode string
}

// DeploymentModeAirgapped is the fail-closed, no-external-egress posture.
const DeploymentModeAirgapped = "airgapped"

// DeploymentModeCloud is the public-cloud posture: admin-configured provider
// base URLs must be public (private/internal targets are rejected as SSRF).
const DeploymentModeCloud = "cloud"

// DriftConfig controls the background schema drift check.
type DriftConfig struct {
	CheckInterval time.Duration
}

// AgentModeShadow/AgentModeActive mirror agent.ModeShadow/agent.ModeActive
// (internal/agent) without importing that package, keeping config decoupled
// from the runtime pipeline it configures.
const (
	AgentModeShadow = "shadow"
	AgentModeActive = "active"
)

// AgentConfig controls the agentic query-runner service: a NATS-driven
// planner/tool-execution pipeline that can supersede the legacy single-shot
// NL-to-SQL path once graduated out of shadow mode.
type AgentConfig struct {
	// Enabled toggles the agent pipeline. Disabled by default — the legacy
	// pipeline handles all traffic until this is turned on.
	Enabled bool
	// Mode is AgentModeShadow (compute for comparison, never surface to the
	// user) or AgentModeActive (the agent's result reaches the user).
	Mode string
	// MaxSteps caps planner tool-call iterations per run, 1-6.
	MaxSteps int
	// MaxClarificationRounds caps clarification round-trips per run, 0-2.
	MaxClarificationRounds int
	// Timeout bounds total run wall-clock time, 1-45 seconds.
	Timeout time.Duration
	// MaxRows caps rows returned by any query.execute tool call, 1-1000.
	MaxRows int
	// JobSubject/StepSubject/ResultSubject/ErrorSubject are the NATS subjects
	// the agent job queue, planner, and workers publish/consume on.
	JobSubject    string
	StepSubject   string
	ResultSubject string
	ErrorSubject  string
	// WorkspaceAllowlist restricts the agent pipeline to specific workspace
	// IDs during rollout; empty means all workspaces.
	WorkspaceAllowlist []string
	// LegacyFallbackEnabled falls back to the legacy NL-to-SQL pipeline when
	// an agent run fails, times out, or its workspace is outside the allowlist.
	LegacyFallbackEnabled bool
}

// WebAgentConfig controls the in-request SSE web agent path. It is separate
// from AgentConfig because the NATS runner validates a 45s job envelope, while
// web runs keep the HTTP stream open and use a longer request budget.
type WebAgentConfig struct {
	// Enabled toggles POST /api/agent/chat. Disabled by default.
	Enabled bool
	// MaxSteps caps planner tool-call iterations per run, 1-6.
	MaxSteps int
	// MaxClarificationRounds caps clarification round-trips per run, 0-2.
	MaxClarificationRounds int
	// Timeout bounds total web run wall-clock time, 1-120 seconds.
	Timeout time.Duration
	// WorkspaceAllowlist restricts the web agent to specific workspace IDs
	// during rollout; empty means all workspaces.
	WorkspaceAllowlist []string
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
	Concurrency   int
}

// JobsConfig toggles background AI job processing.
type JobsConfig struct {
	Enabled         bool
	ConsumerEnabled bool
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

// PublicShareConfig tunes the anonymous dashboard-share endpoints.
type PublicShareConfig struct {
	CacheTTL           time.Duration
	RateLimitPerMinute int
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
	// APIURL is the internal base URL of the API gateway/monolith that the
	// standalone MCP service forwards governed tool calls to. Empty in the
	// monolith (MCP dispatches to its own in-process router instead).
	APIURL string
	// APIHost overrides the Host header sent to APIURL. Set it when APIURL
	// points at an in-cluster gateway whose HTTPRoutes match on the public
	// hostname (e.g. APIURL=http://eg-gw-http.envoy-gateway-system... with
	// APIHost=abi.il1.nl) so tool dispatches stay inside the cluster instead
	// of round-tripping through Cloudflare.
	APIHost string
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
	// TieredEnabled runs synonym/homonym checks before temporal/scope and gates LLM tier usage.
	TieredEnabled bool
	// MaxLLMTierPerQuestion caps how many clarification rounds may invoke the LLM ambiguity tier.
	MaxLLMTierPerQuestion int
	// ClarifyPolicyEnabled gates the clarify-vs-default policy: clarify only for
	// genuine toss-ups, otherwise proceed with the top interpretation plus a
	// caveat. Disabling it restores the legacy "always clarify when the analyzer
	// fires" behavior.
	ClarifyPolicyEnabled bool
}

// AIMemoryConfig groups the NL→SQL confirmed-query memory recall knobs.
type AIMemoryConfig struct {
	// RecallEnabled toggles confirmed-query few-shot recall injection.
	RecallEnabled bool
	// RecallLimit caps recalled confirmed examples appended to the prompt.
	RecallLimit int
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
	// WorkspaceDailyTokenBudget caps total LLM tokens (prompt+completion) a
	// single workspace may spend per UTC day. 0 disables the cap.
	WorkspaceDailyTokenBudget int
	// AnswerEnabled controls the post-execution natural-language answer synthesis:
	// a separate, lightweight LLM call that summarizes the query result in one or
	// two sentences in the user's locale. Enabled by default.
	AnswerEnabled bool
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
	Memory      AIMemoryConfig
}

// AIQueryView is the resolved NL-to-LogicalQuery path (BI_AI_QUERY_* overrides applied).
type AIQueryView struct {
	Config   AIConfig
	Override bool
}

// Configured reports whether the query LLM path can call a provider.
func (v AIQueryView) Configured() bool {
	cfg := v.Config.Connection
	if strings.TrimSpace(cfg.Model) == "" {
		return false
	}
	if strings.TrimSpace(cfg.APIKey) != "" {
		return true
	}
	switch strings.ToLower(strings.TrimSpace(cfg.Provider)) {
	case "", "openai", "openai-compatible":
		return strings.TrimSpace(cfg.BaseURL) != ""
	default:
		return false
	}
}

// AIEmbeddingView is the resolved embedding HTTP connection.
type AIEmbeddingView struct {
	Model       string
	BaseURL     string
	APIKey      string
	HTTPTimeout time.Duration
}

// Configured reports whether vector routing / embed-metadata can call an embeddings API.
func (v AIEmbeddingView) Configured() bool {
	if strings.TrimSpace(v.Model) == "" {
		return false
	}
	if strings.TrimSpace(v.APIKey) == "" {
		return false
	}
	return strings.TrimSpace(v.BaseURL) != ""
}

// AITranslationView is the resolved translation HTTP connection.
type AITranslationView struct {
	Model       string
	BaseURL     string
	APIKey      string
	HTTPTimeout time.Duration
}

// Configured reports whether metadata description translation is enabled.
func (v AITranslationView) Configured() bool {
	if strings.TrimSpace(v.Model) == "" {
		return false
	}
	return strings.TrimSpace(v.BaseURL) != ""
}

// Load reads configuration from environment variables.
func Load() (*Config, error) {
	cfg := loadConfigFromEnv()
	if err := validateLoadedConfig(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func loadConfigFromEnv() *Config {
	return &Config{
		DeploymentMode: strings.ToLower(strings.TrimSpace(getEnv("BI_DEPLOYMENT_MODE", "cloud"))),
		Drift: DriftConfig{
			CheckInterval: getEnvAsDuration("BI_DRIFT_CHECK_INTERVAL", 6*time.Hour),
		},
		Agent:    loadAgentConfigFromEnv(),
		WebAgent: loadWebAgentConfigFromEnv(),
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
		PublicShare: PublicShareConfig{
			CacheTTL:           getEnvAsDuration("BI_PUBLIC_SHARE_CACHE_TTL", 60*time.Second),
			RateLimitPerMinute: getEnvAsInt("BI_PUBLIC_SHARE_RATE_LIMIT", 60),
		},
		Security: SecurityConfig{
			// No default: an empty key is rejected by validateLoadedConfig as
			// "required". Shipping a usable placeholder key in the binary would be
			// a real key any code path bypassing Load()/validation could run with.
			EncryptionKey:    getEnv("BI_ENCRYPTION_KEY", ""),
			AdminAPIKey:      getEnv("BI_ADMIN_API_KEY", ""),
			InternalAPIToken: getEnv("BI_INTERNAL_API_TOKEN", ""),
			APIKey:           getEnv("BI_API_KEY", ""),
			MetricsAPIKey:    getEnv("BI_METRICS_API_KEY", ""),
		},
		Services: ServicesConfig{
			CatalogURL: strings.TrimRight(getEnv("BI_CATALOG_SERVICE_URL", ""), "/"),
			QueryURL:   strings.TrimRight(getEnv("BI_QUERY_SERVICE_URL", ""), "/"),
			AIURL:      strings.TrimRight(getEnv("BI_AI_SERVICE_URL", ""), "/"),
			APIURL:     strings.TrimRight(getEnv("BI_API_SERVICE_URL", ""), "/"),
			APIHost:    strings.TrimSpace(getEnv("BI_API_SERVICE_HOST", "")),
		},
		AI: loadAIConfigFromEnv(),
		NATS: NATSConfig{
			URL:           getEnv("BI_NATS_URL", ""),
			Stream:        getEnv("BI_NATS_STREAM", "BIQLY_AI_JOBS"),
			Subject:       getEnv("BI_NATS_SUBJECT", "biqly.ai.jobs"),
			ConsumerGroup: getEnv("BI_NATS_CONSUMER_GROUP", "biqly-ai-workers"),
			Concurrency:   getEnvAsInt("BI_AI_JOBS_CONCURRENCY", 1),
		},
		Jobs: JobsConfig{
			Enabled:         getEnvAsBool("BI_AI_JOBS_ENABLED", true),
			ConsumerEnabled: getEnvAsBool("BI_AI_JOBS_CONSUMER_ENABLED", true),
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
}

func loadAgentConfigFromEnv() AgentConfig {
	return AgentConfig{
		Enabled:                getEnvAsBool("BI_AGENT_ENABLED", false),
		Mode:                   strings.ToLower(strings.TrimSpace(getEnv("BI_AGENT_MODE", AgentModeShadow))),
		MaxSteps:               getEnvAsInt("BI_AGENT_MAX_STEPS", 6),
		MaxClarificationRounds: getEnvAsInt("BI_AGENT_MAX_CLARIFICATION_ROUNDS", 2),
		Timeout:                getEnvAsDuration("BI_AGENT_TIMEOUT", 45*time.Second),
		MaxRows:                getEnvAsInt("BI_AGENT_MAX_ROWS", 1000),
		JobSubject:             getEnv("BI_AGENT_JOB_SUBJECT", "biqly.agent.jobs"),
		StepSubject:            getEnv("BI_AGENT_STEP_SUBJECT", "biqly.agent.steps"),
		ResultSubject:          getEnv("BI_AGENT_RESULT_SUBJECT", "biqly.agent.results"),
		ErrorSubject:           getEnv("BI_AGENT_ERROR_SUBJECT", "biqly.agent.errors"),
		WorkspaceAllowlist:     splitCSV(getEnv("BI_AGENT_WORKSPACE_ALLOWLIST", "")),
		LegacyFallbackEnabled:  getEnvAsBool("BI_AGENT_LEGACY_FALLBACK_ENABLED", true),
	}
}

func loadWebAgentConfigFromEnv() WebAgentConfig {
	allowlist := getEnv("BI_WEB_AGENT_WORKSPACE_ALLOWLIST", getEnv("BI_AGENT_WORKSPACE_ALLOWLIST", ""))
	return WebAgentConfig{
		Enabled:                getEnvAsBool("BI_WEB_AGENT_ENABLED", false),
		MaxSteps:               getEnvAsInt("BI_WEB_AGENT_MAX_STEPS", 6),
		MaxClarificationRounds: getEnvAsInt("BI_WEB_AGENT_MAX_CLARIFICATION_ROUNDS", 2),
		Timeout:                getEnvAsDuration("BI_WEB_AGENT_TIMEOUT", 120*time.Second),
		WorkspaceAllowlist:     splitCSV(allowlist),
	}
}

func loadAIConfigFromEnv() AIConfig {
	return AIConfig{
		// Connection/model selection is intentionally NOT read from the environment —
		// it comes only from ai_providers / ai_models via ProviderStore.
		Connection: AIConnectionConfig{
			HTTPTimeoutSeconds: getEnvAsInt("BI_AI_HTTP_TIMEOUT_SECONDS", 300),
			RateLimitPerMinute: getEnvAsInt("BI_AI_RATE_LIMIT_PER_MINUTE", 20),
		},
		Generation: AIGenerationConfig{
			MaxPromptInputRunes:       getEnvAsInt("BI_AI_MAX_PROMPT_RUNES", 80000),
			MaxRetries:                getEnvAsInt("BI_AI_MAX_RETRIES", 2),
			MultiCandidateCount:       getEnvAsInt("BI_AI_MULTI_CANDIDATE_COUNT", 1),
			WorkspaceDailyTokenBudget: getEnvAsInt("BI_AI_WORKSPACE_DAILY_TOKEN_BUDGET", 0),
			AnswerEnabled:             getEnvAsBool("BI_AI_ANSWER_ENABLED", true),
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
			MaxOptions:            getEnvAsInt("BI_AI_AMBIGUITY_MAX_OPTIONS", 5),
			LLMEnabled:            getEnvAsBool("BI_AI_AMBIGUITY_LLM_ENABLED", false),
			TieredEnabled:         getEnvAsBool("BI_AI_AMBIGUITY_TIERED_ENABLED", false),
			MaxLLMTierPerQuestion: getEnvAsInt("BI_AI_AMBIGUITY_MAX_LLM_TIER_PER_QUESTION", 1),
			ClarifyPolicyEnabled:  getEnvAsBool("BI_AI_CLARIFY_POLICY_ENABLED", true),
		},
		Memory: AIMemoryConfig{
			RecallEnabled: getEnvAsBool("BI_AI_MEMORY_RECALL_ENABLED", true),
			RecallLimit:   getEnvAsInt("BI_AI_MEMORY_RECALL_LIMIT", 5),
		},
	}
}

func validateLoadedConfig(cfg *Config) error {
	if !slices.Contains([]string{"cloud", "private", DeploymentModeAirgapped}, cfg.DeploymentMode) {
		return fmt.Errorf("BI_DEPLOYMENT_MODE must be one of cloud, private, airgapped; got %q", cfg.DeploymentMode)
	}
	if cfg.Metadata.DSN == "" {
		return errors.New("BI_METADATA_DB_DSN is required")
	}
	// The metadata DB carries encrypted datasource secrets and audit data;
	// running its connection without TLS in production is a real exposure. Warn
	// rather than fail — some clusters front Postgres with a network policy — but
	// make the insecure posture visible.
	if env.IsProduction() && strings.Contains(cfg.Metadata.DSN, "sslmode=disable") {
		slog.Warn("BI_METADATA_DB_DSN uses sslmode=disable in production; metadata DB traffic is unencrypted")
	}
	if cfg.Security.EncryptionKey == "" {
		return errors.New("BI_ENCRYPTION_KEY is required")
	}
	if cfg.Security.EncryptionKey == "change-this-to-a-secure-32-byte-key!!" {
		return errors.New("BI_ENCRYPTION_KEY must be changed from its default value")
	}
	if err := validateFloatRange("BI_PII_DETECTION_THRESHOLD", cfg.PII.DetectionThreshold, 0, 1); err != nil {
		return err
	}
	if err := validateFloatRange("BI_AI_AMBIGUITY_CONFIDENCE_THRESHOLD", cfg.AI.Ambiguity.ConfidenceThreshold, 0, 1); err != nil {
		return err
	}
	if cfg.AI.Ambiguity.MaxLLMTierPerQuestion < 0 {
		return fmt.Errorf("BI_AI_AMBIGUITY_MAX_LLM_TIER_PER_QUESTION must be >= 0, got %d", cfg.AI.Ambiguity.MaxLLMTierPerQuestion)
	}
	if err := validateFloatRange("BI_AI_EMBEDDING_WEIGHT", cfg.AI.Embedding.Weight, 0, 100); err != nil {
		return err
	}
	if err := validateAgentConfig(cfg.Agent); err != nil {
		return err
	}
	if err := validateWebAgentConfig(cfg.WebAgent); err != nil {
		return err
	}
	// Fail-closed: auth must stay enabled in production/Kubernetes (see TestProductionAuthEnabledFailClosed).
	if env.IsProduction() && !cfg.Auth.Enabled {
		return errors.New("BI_AUTH_ENABLED must be true in production")
	}
	return nil
}

// HTTPTimeout returns the base provider HTTP timeout.
func (c AIConfig) HTTPTimeout() time.Duration {
	seconds := c.Connection.HTTPTimeoutSeconds
	if seconds <= 0 {
		seconds = 300
	}
	return time.Duration(seconds) * time.Second
}

// RequestTimeout returns the server-side request budget for AI endpoints.
func (c AIConfig) RequestTimeout() time.Duration {
	timeout := c.HTTPTimeout()
	emb := c.ResolvedEmbedding()
	if emb.HTTPTimeout > timeout {
		timeout = emb.HTTPTimeout
	}
	tr := c.ResolvedTranslation()
	if tr.HTTPTimeout > timeout {
		timeout = tr.HTTPTimeout
	}
	return timeout + 30*time.Second
}

// ResolvedQuery applies BI_AI_QUERY_* overrides for the NL-to-LogicalQuery path.
func (c AIConfig) ResolvedQuery() AIQueryView {
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
	return AIQueryView{
		Config: out,
		Override: strings.TrimSpace(c.Query.Provider) != "" ||
			strings.TrimSpace(c.Query.Model) != "" ||
			strings.TrimSpace(c.Query.BaseURL) != "" ||
			strings.TrimSpace(c.Query.APIKey) != "" ||
			c.Query.HTTPTimeoutSeconds > 0,
	}
}

// ResolvedEmbedding resolves BI_AI_EMBEDDING_* with fallbacks to the base connection.
func (c AIConfig) ResolvedEmbedding() AIEmbeddingView {
	seconds := c.Embedding.HTTPTimeoutSeconds
	if seconds <= 0 {
		seconds = c.Connection.HTTPTimeoutSeconds
	}
	if seconds <= 0 {
		seconds = 300
	}
	return AIEmbeddingView{
		Model:       strings.TrimSpace(c.Embedding.Model),
		BaseURL:     resolveEmbeddingBaseURL(c),
		APIKey:      resolveEmbeddingAPIKey(c),
		HTTPTimeout: time.Duration(seconds) * time.Second,
	}
}

// ResolvedTranslation resolves BI_AI_TRANSLATION_* with fallbacks to the base connection.
func (c AIConfig) ResolvedTranslation() AITranslationView {
	seconds := c.Translation.HTTPTimeoutSeconds
	if seconds <= 0 {
		seconds = c.Connection.HTTPTimeoutSeconds
	}
	if seconds <= 0 {
		seconds = 120
	}
	return AITranslationView{
		Model:       strings.TrimSpace(c.Translation.Model),
		BaseURL:     resolveTranslationBaseURL(c),
		APIKey:      resolveTranslationAPIKey(c),
		HTTPTimeout: time.Duration(seconds) * time.Second,
	}
}

func resolveEmbeddingAPIKey(c AIConfig) string {
	if s := strings.TrimSpace(c.Embedding.APIKey); s != "" {
		return s
	}
	return c.Connection.APIKey
}

func resolveEmbeddingBaseURL(c AIConfig) string {
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

func resolveTranslationAPIKey(c AIConfig) string {
	if s := strings.TrimSpace(c.Translation.APIKey); s != "" {
		return s
	}
	return c.Connection.APIKey
}

func resolveTranslationBaseURL(c AIConfig) string {
	if s := strings.TrimSpace(c.Translation.BaseURL); s != "" {
		return strings.TrimRight(s, "/")
	}
	return strings.TrimRight(strings.TrimSpace(c.Connection.BaseURL), "/")
}

// These thin wrappers delegate to internal/env, the single source of truth for
// env parsing, so this loader and the per-service loaders (auth/mail/ai-eval)
// share one parsing/invalid-value policy that cannot drift.

func getEnv(key, defaultVal string) string { return env.String(key, defaultVal) }

func splitCSV(raw string) []string { return env.SplitCSV(raw) }

func getEnvAsInt(key string, defaultVal int) int { return env.Int(key, defaultVal) }

func getEnvAsFloat(key string, defaultVal float64) float64 { return env.Float(key, defaultVal) }

// validateAgentConfig enforces the same bounds internal/agent's job
// validation applies, checked once at load time so a misconfigured
// deployment fails fast instead of rejecting every run.
func validateAgentConfig(cfg AgentConfig) error {
	if !slices.Contains([]string{AgentModeShadow, AgentModeActive}, cfg.Mode) {
		return fmt.Errorf("BI_AGENT_MODE must be %q or %q, got %q", AgentModeShadow, AgentModeActive, cfg.Mode)
	}
	if cfg.MaxSteps < 1 || cfg.MaxSteps > 6 {
		return fmt.Errorf("BI_AGENT_MAX_STEPS must be between 1 and 6, got %d", cfg.MaxSteps)
	}
	if cfg.MaxClarificationRounds < 0 || cfg.MaxClarificationRounds > 2 {
		return fmt.Errorf("BI_AGENT_MAX_CLARIFICATION_ROUNDS must be between 0 and 2, got %d", cfg.MaxClarificationRounds)
	}
	if cfg.Timeout < 1*time.Second || cfg.Timeout > 45*time.Second {
		return fmt.Errorf("BI_AGENT_TIMEOUT must be between 1s and 45s, got %s", cfg.Timeout)
	}
	if cfg.MaxRows < 1 || cfg.MaxRows > 1000 {
		return fmt.Errorf("BI_AGENT_MAX_ROWS must be between 1 and 1000, got %d", cfg.MaxRows)
	}
	return nil
}

func validateWebAgentConfig(cfg WebAgentConfig) error {
	if cfg.MaxSteps < 1 || cfg.MaxSteps > 6 {
		return fmt.Errorf("BI_WEB_AGENT_MAX_STEPS must be between 1 and 6, got %d", cfg.MaxSteps)
	}
	if cfg.MaxClarificationRounds < 0 || cfg.MaxClarificationRounds > 2 {
		return fmt.Errorf("BI_WEB_AGENT_MAX_CLARIFICATION_ROUNDS must be between 0 and 2, got %d", cfg.MaxClarificationRounds)
	}
	if cfg.Timeout < 1*time.Second || cfg.Timeout > 120*time.Second {
		return fmt.Errorf("BI_WEB_AGENT_TIMEOUT must be between 1s and 120s, got %s", cfg.Timeout)
	}
	return nil
}

func validateFloatRange(key string, val, minVal, maxVal float64) error {
	if val < minVal || val > maxVal {
		return fmt.Errorf("%s must be between %g and %g, got %g", key, minVal, maxVal, val)
	}
	return nil
}

func getEnvAsBool(key string, defaultVal bool) bool { return env.Bool(key, defaultVal) }

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

// HTTPWriteTimeout is the http.Server WriteTimeout for the AI service.
// Go's WriteTimeout is absolute from response start, so it must cover the
// longest in-request path: web-agent SSE (BI_WEB_AGENT_TIMEOUT), AI provider
// HTTP calls, and query max runtime. A too-low value resets the SSE stream
// mid-run (Envoy UPE / browser "network error") even while heartbeats flow.
func (c *Config) HTTPWriteTimeout() time.Duration {
	const buffer = 15 * time.Second
	timeout := c.MaxQueryRuntime() + buffer
	if web := c.WebAgent.Timeout + buffer; web > timeout {
		timeout = web
	}
	if ai := c.AI.RequestTimeout(); ai > timeout {
		timeout = ai
	}
	return timeout
}

func getEnvAsDuration(key string, defaultVal time.Duration) time.Duration {
	return env.Duration(key, defaultVal)
}
