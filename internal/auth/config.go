package auth

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port               int
	DBDSN              string
	RedisDSN           string
	JWTPrivateKeyPath  string
	JWTPublicKeyPath   string
	JWTAccessTTL       time.Duration
	JWTRefreshTTL      time.Duration
	JWTIssuer          string
	JWTAudience        string
	InternalToken      string
	EncryptionKey      string
	RateLimitPerMin    int
	CORSAllowedOrigins []string
	GitHubClientID     string
	GitHubClientSecret string
	GitHubRedirectURL  string
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
	WebAuthnRPID       string
	WebAuthnRPName     string
	WebAuthnOrigins    []string
	SMTPHost           string
	SMTPPort           int
	SMTPUser           string
	SMTPPass           string
	SMTPFrom           string
}

func LoadConfig() (*Config, error) {
	portStr := os.Getenv("BI_AUTH_PORT")
	port := 8889
	if portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil {
			port = p
		}
	}

	dbDSN := os.Getenv("BI_AUTH_DB_DSN")
	if dbDSN == "" {
		//nolint:gosec
		dbDSN = "postgres://bi_auth_user:bi_auth_password@localhost:5434/bi_auth?sslmode=disable"
	}

	redisDSN := os.Getenv("BI_AUTH_REDIS_DSN")
	if redisDSN == "" {
		redisDSN = "redis://localhost:6379"
	}

	accessTTL := 15 * time.Minute
	if ttlStr := os.Getenv("BI_AUTH_JWT_ACCESS_TTL"); ttlStr != "" {
		if d, err := time.ParseDuration(ttlStr); err == nil {
			accessTTL = d
		}
	}

	refreshTTL := 7 * 24 * time.Hour
	if ttlStr := os.Getenv("BI_AUTH_JWT_REFRESH_TTL"); ttlStr != "" {
		if d, err := time.ParseDuration(ttlStr); err == nil {
			refreshTTL = d
		}
	}

	rateLimit := 60
	if rStr := os.Getenv("BI_AUTH_RATE_LIMIT_PER_MINUTE"); rStr != "" {
		if r, err := strconv.Atoi(rStr); err == nil {
			rateLimit = r
		}
	}

	githubRedirect := os.Getenv("BI_AUTH_GITHUB_REDIRECT_URL")
	if githubRedirect == "" {
		githubRedirect = "http://localhost:8889/api/auth/oauth/github/callback"
	}

	googleRedirect := os.Getenv("BI_AUTH_GOOGLE_REDIRECT_URL")
	if googleRedirect == "" {
		googleRedirect = "http://localhost:8889/api/auth/oauth/google/callback"
	}

	rpID := os.Getenv("BI_AUTH_WEBAUTHN_RP_ID")
	if rpID == "" {
		rpID = "localhost"
	}

	rpName := os.Getenv("BI_AUTH_WEBAUTHN_RP_NAME")
	if rpName == "" {
		rpName = "Biqly"
	}

	originsStr := os.Getenv("BI_AUTH_WEBAUTHN_RP_ORIGINS")
	var origins []string
	if originsStr != "" {
		origins = strings.Split(originsStr, ",")
	} else {
		origins = []string{"http://localhost:5173", "http://localhost:3333"}
	}

	smtpPortStr := os.Getenv("BI_AUTH_SMTP_PORT")
	smtpPort := 587
	if smtpPortStr != "" {
		if p, err := strconv.Atoi(smtpPortStr); err == nil {
			smtpPort = p
		}
	}

	issuer := os.Getenv("BI_AUTH_JWT_ISSUER")
	if issuer == "" {
		issuer = DefaultJWTIssuer
	}
	audience := os.Getenv("BI_AUTH_JWT_AUDIENCE")
	if audience == "" {
		audience = DefaultJWTAudience
	}

	var corsOrigins []string
	if v := strings.TrimSpace(os.Getenv("BI_AUTH_CORS_ALLOWED_ORIGINS")); v != "" {
		for o := range strings.SplitSeq(v, ",") {
			if trimmed := strings.TrimSpace(o); trimmed != "" {
				corsOrigins = append(corsOrigins, trimmed)
			}
		}
	}

	cfg := &Config{
		Port:               port,
		DBDSN:              dbDSN,
		RedisDSN:           redisDSN,
		JWTPrivateKeyPath:  os.Getenv("BI_AUTH_JWT_PRIVATE_KEY_PATH"),
		JWTPublicKeyPath:   os.Getenv("BI_AUTH_JWT_PUBLIC_KEY_PATH"),
		JWTAccessTTL:       accessTTL,
		JWTRefreshTTL:      refreshTTL,
		JWTIssuer:          issuer,
		JWTAudience:        audience,
		InternalToken:      os.Getenv("BI_AUTH_INTERNAL_TOKEN"),
		EncryptionKey:      os.Getenv("BI_AUTH_ENCRYPTION_KEY"),
		RateLimitPerMin:    rateLimit,
		CORSAllowedOrigins: corsOrigins,
		GitHubClientID:     os.Getenv("BI_AUTH_GITHUB_CLIENT_ID"),
		GitHubClientSecret: os.Getenv("BI_AUTH_GITHUB_CLIENT_SECRET"),
		GitHubRedirectURL:  githubRedirect,
		GoogleClientID:     os.Getenv("BI_AUTH_GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("BI_AUTH_GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL:  googleRedirect,
		WebAuthnRPID:       rpID,
		WebAuthnRPName:     rpName,
		WebAuthnOrigins:    origins,
		SMTPHost:           os.Getenv("BI_AUTH_SMTP_HOST"),
		SMTPPort:           smtpPort,
		SMTPUser:           os.Getenv("BI_AUTH_SMTP_USER"),
		SMTPPass:           os.Getenv("BI_AUTH_SMTP_PASS"),
		SMTPFrom:           os.Getenv("BI_AUTH_SMTP_FROM"),
	}

	if cfg.InternalToken == "" {
		cfg.InternalToken = "dev-internal-token"
	}

	return cfg, nil
}

func (c *Config) HTTPAddr() string {
	return ":" + strconv.Itoa(c.Port)
}
