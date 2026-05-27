package mail

import (
	"os"
	"strconv"
)

// Config holds the SMTP connection settings, transactional-email policy knobs,
// and worker runtime settings consumed by the standalone mail worker.
// SMTPEmailSender reads only the SMTP/policy fields; the Port, DBDSN, RedisDSN
// and InternalToken fields are consumed by the worker entrypoint.
type Config struct {
	// SMTP / email policy (used by SMTPEmailSender)
	SMTPHost           string
	SMTPPort           int
	SMTPUser           string
	SMTPPass           string
	SMTPFrom           string
	SMTPSenderName     string
	FrontendBaseURL    string
	EmailDefaultLocale string
	EmailDailyLimit    int
	EmailQueueSize     int
	EmailRetries       int

	// Worker runtime (used by cmd/mail)
	Port          int
	DBDSN         string
	RedisDSN      string
	InternalToken string
}

// NewConfigFromEnv loads the worker configuration from BI_MAIL_* environment
// variables, applying the same defaults the auth service previously used for
// its embedded sender.
func NewConfigFromEnv() *Config {
	return &Config{
		SMTPHost:           os.Getenv("BI_MAIL_SMTP_HOST"),
		SMTPPort:           intEnv("BI_MAIL_SMTP_PORT", 587),
		SMTPUser:           os.Getenv("BI_MAIL_SMTP_USER"),
		SMTPPass:           os.Getenv("BI_MAIL_SMTP_PASS"),
		SMTPFrom:           os.Getenv("BI_MAIL_SMTP_FROM"),
		SMTPSenderName:     stringEnv("BI_MAIL_SMTP_SENDER_NAME", "ABI"),
		FrontendBaseURL:    stringEnv("BI_MAIL_FRONTEND_BASE_URL", "http://localhost:3333"),
		EmailDefaultLocale: stringEnv("BI_MAIL_EMAIL_DEFAULT_LOCALE", "en"),
		EmailDailyLimit:    nonNegativeIntEnv("BI_MAIL_EMAIL_DAILY_LIMIT", 10),
		EmailQueueSize:     positiveIntEnv("BI_MAIL_EMAIL_QUEUE_SIZE", 256),
		EmailRetries:       nonNegativeIntEnv("BI_MAIL_EMAIL_RETRIES", 3),
		Port:               intEnv("BI_MAIL_PORT", 8890),
		DBDSN:              os.Getenv("BI_MAIL_DB_DSN"),
		RedisDSN:           os.Getenv("BI_MAIL_REDIS_DSN"),
		InternalToken:      os.Getenv("BI_MAIL_INTERNAL_TOKEN"),
	}
}

func stringEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func intEnv(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func positiveIntEnv(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return n
		}
	}
	return def
}

func nonNegativeIntEnv(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
	}
	return def
}
