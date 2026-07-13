package auth

import (
	"errors"
	"os"
	"strconv"
	"time"

	"github.com/biqly/biqly/internal/env"
)

type Config struct {
	Port                 int
	DBDSN                string
	RedisDSN             string
	JWTPrivateKeyPath    string
	JWTPublicKeyPath     string
	JWTAccessTTL         time.Duration
	JWTRefreshTTL        time.Duration
	JWTIssuer            string
	JWTAudience          string
	InternalToken        string
	EncryptionKey        string
	RateLimitPerMin      int
	CORSAllowedOrigins   []string
	GitHubClientID       string
	GitHubClientSecret   string
	GitHubRedirectURL    string
	GoogleClientID       string
	GoogleClientSecret   string
	GoogleRedirectURL    string
	WebAuthnRPID         string
	WebAuthnRPName       string
	WebAuthnOrigins      []string
	FrontendBaseURL      string
	MailServiceURL       string
	MailInternalToken    string
	CatalogServiceURL    string
	InternalAPIToken     string
	MaxActiveSessions    int
	PasswordMaxAgeDays   int
	GDPRPurgeAfterDays   int
	SessionAbsoluteTTL   time.Duration
	SessionIdleTTL       time.Duration
	PasswordPolicy       PasswordPolicy
	HSTSEnabled          bool
	HSTSPreload          bool
	HSTSMaxAgeSeconds    int
	WebAuthnChallengeTTL time.Duration
}

func LoadConfig() (*Config, error) {
	webAuthnOrigins := splitEnvDefault("BI_AUTH_WEBAUTHN_RP_ORIGINS", []string{"http://localhost:5173", "http://localhost:3333"})
	cfg := &Config{
		Port:               intEnv("BI_AUTH_PORT", 8889),
		DBDSN:              stringEnv("BI_AUTH_DB_DSN", "postgres://bi_auth_user:bi_auth_password@localhost:5434/bi_auth?sslmode=disable"),
		RedisDSN:           stringEnv("BI_AUTH_REDIS_DSN", "redis://localhost:6379"),
		JWTPrivateKeyPath:  os.Getenv("BI_AUTH_JWT_PRIVATE_KEY_PATH"),
		JWTPublicKeyPath:   os.Getenv("BI_AUTH_JWT_PUBLIC_KEY_PATH"),
		JWTAccessTTL:       durationEnv("BI_AUTH_JWT_ACCESS_TTL", 15*time.Minute),
		JWTRefreshTTL:      durationEnv("BI_AUTH_JWT_REFRESH_TTL", 7*24*time.Hour),
		JWTIssuer:          stringEnv("BI_AUTH_JWT_ISSUER", DefaultJWTIssuer),
		JWTAudience:        stringEnv("BI_AUTH_JWT_AUDIENCE", DefaultJWTAudience),
		InternalToken:      os.Getenv("BI_AUTH_INTERNAL_TOKEN"),
		EncryptionKey:      os.Getenv("BI_AUTH_ENCRYPTION_KEY"),
		RateLimitPerMin:    intEnv("BI_AUTH_RATE_LIMIT_PER_MINUTE", 60),
		CORSAllowedOrigins: splitEnv("BI_AUTH_CORS_ALLOWED_ORIGINS"),
		GitHubClientID:     os.Getenv("BI_AUTH_GITHUB_CLIENT_ID"),
		GitHubClientSecret: os.Getenv("BI_AUTH_GITHUB_CLIENT_SECRET"),
		GitHubRedirectURL:  stringEnv("BI_AUTH_GITHUB_REDIRECT_URL", "http://localhost:8889/api/auth/oauth/github/callback"),
		GoogleClientID:     os.Getenv("BI_AUTH_GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("BI_AUTH_GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL:  stringEnv("BI_AUTH_GOOGLE_REDIRECT_URL", "http://localhost:8889/api/auth/oauth/google/callback"),
		WebAuthnRPID:       stringEnv("BI_AUTH_WEBAUTHN_RP_ID", "localhost"),
		WebAuthnRPName:     stringEnv("BI_AUTH_WEBAUTHN_RP_NAME", "Biqly"),
		WebAuthnOrigins:    webAuthnOrigins,
		FrontendBaseURL:    stringEnv("BI_AUTH_FRONTEND_BASE_URL", "http://localhost:3333"),
		MailServiceURL:     stringEnv("BI_AUTH_MAIL_SERVICE_URL", "http://localhost:8890"),
		MailInternalToken:  os.Getenv("BI_AUTH_MAIL_INTERNAL_TOKEN"),
		// CatalogServiceURL + InternalAPIToken let the sharing ownership guard
		// resolve a shared resource to its datasource via the catalog service's
		// /internal endpoint (the resources live there, not in the auth DB).
		CatalogServiceURL:    stringEnv("BI_AUTH_CATALOG_SERVICE_URL", "http://localhost:8888"),
		InternalAPIToken:     os.Getenv("BI_INTERNAL_API_TOKEN"),
		MaxActiveSessions:    positiveIntEnv("BI_AUTH_MAX_SESSIONS", 5),
		PasswordMaxAgeDays:   nonNegativeIntEnv("BI_AUTH_PASSWORD_MAX_AGE_DAYS", 0),
		GDPRPurgeAfterDays:   positiveIntEnv("BI_AUTH_GDPR_PURGE_AFTER_DAYS", 30),
		SessionAbsoluteTTL:   positiveDurationEnv("BI_AUTH_SESSION_ABSOLUTE_TTL", 30*24*time.Hour),
		SessionIdleTTL:       positiveDurationEnv("BI_AUTH_SESSION_IDLE_TTL", 4*time.Hour),
		PasswordPolicy:       passwordPolicyFromEnv(),
		HSTSPreload:          boolEnv("BI_AUTH_HSTS_PRELOAD", false),
		HSTSMaxAgeSeconds:    nonNegativeIntEnv("BI_AUTH_HSTS_MAX_AGE_SECONDS", 63072000),
		WebAuthnChallengeTTL: positiveDurationEnv("BI_AUTH_WEBAUTHN_CHALLENGE_TTL", 60*time.Second),
	}

	if cfg.InternalToken == "" {
		// The internal token gates every /internal/auth route. A well-known
		// fallback in production would let anyone who can reach those routes
		// impersonate a trusted internal service, so fail closed there (mirrors
		// the JWT key handling in jwt.go).
		if env.IsProduction() {
			return nil, errors.New("BI_AUTH_INTERNAL_TOKEN is unconfigured under production environment")
		}
		cfg.InternalToken = "dev-internal-token"
	}

	cfg.HSTSEnabled = boolEnv("BI_HSTS_ENABLED", env.HSTSEnabledDefault(cfg.WebAuthnOrigins...))

	return cfg, nil
}

func (c *Config) HTTPAddr() string {
	return ":" + strconv.Itoa(c.Port)
}

// These thin wrappers delegate to internal/env so the auth loader shares one
// env-parsing policy with the other services (was a byte-identical copy).

func stringEnv(key, defaultValue string) string { return env.String(key, defaultValue) }

func intEnv(key string, defaultValue int) int { return env.Int(key, defaultValue) }

func positiveIntEnv(key string, defaultValue int) int { return env.PositiveInt(key, defaultValue) }

func nonNegativeIntEnv(key string, defaultValue int) int {
	return env.NonNegativeInt(key, defaultValue)
}

func durationEnv(key string, defaultValue time.Duration) time.Duration {
	return env.Duration(key, defaultValue)
}

func positiveDurationEnv(key string, defaultValue time.Duration) time.Duration {
	return env.PositiveDuration(key, defaultValue)
}

func splitEnv(key string) []string { return env.CSV(key) }

func splitEnvDefault(key string, defaultValue []string) []string {
	return env.CSVDefault(key, defaultValue)
}

func passwordPolicyFromEnv() PasswordPolicy {
	pw := DefaultPasswordPolicy()
	pw.MinLength = positiveIntEnv("BI_AUTH_PASSWORD_MIN_LEN", pw.MinLength)
	pw.MaxLength = positiveIntEnv("BI_AUTH_PASSWORD_MAX_LEN", pw.MaxLength)
	pw.RequireUpper = boolEnv("BI_AUTH_PASSWORD_REQUIRE_UPPER", pw.RequireUpper)
	pw.RequireLower = boolEnv("BI_AUTH_PASSWORD_REQUIRE_LOWER", pw.RequireLower)
	pw.RequireDigit = boolEnv("BI_AUTH_PASSWORD_REQUIRE_DIGIT", pw.RequireDigit)
	pw.RequireSpecial = boolEnv("BI_AUTH_PASSWORD_REQUIRE_SPECIAL", pw.RequireSpecial)
	pw.MinScore = passwordScoreEnv("BI_AUTH_PASSWORD_MIN_SCORE", pw.MinScore)
	return pw
}

func boolEnv(key string, defaultValue bool) bool {
	if value := os.Getenv(key); value != "" {
		return parseBoolEnv(value, defaultValue)
	}
	return defaultValue
}

func passwordScoreEnv(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if n, err := strconv.Atoi(value); err == nil && n >= 0 && n <= 4 {
			return n
		}
	}
	return defaultValue
}
