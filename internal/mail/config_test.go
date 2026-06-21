package mail

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewConfigFromEnv_Defaults(t *testing.T) {
	cfg := NewConfigFromEnv()
	// With no env vars set, all string fields are empty and int fields use defaults.
	assert.Equal(t, "", cfg.SMTPHost)
	assert.Equal(t, 587, cfg.SMTPPort)
	assert.Equal(t, "", cfg.SMTPUser)
	assert.Equal(t, "", cfg.SMTPPass)
	assert.Equal(t, "", cfg.SMTPFrom)
	assert.Equal(t, "ABI", cfg.SMTPSenderName)
	assert.Equal(t, "http://localhost:3333", cfg.FrontendBaseURL)
	assert.Equal(t, "en", cfg.EmailDefaultLocale)
	assert.Equal(t, 10, cfg.EmailDailyLimit)
	assert.Equal(t, 256, cfg.EmailQueueSize)
	assert.Equal(t, 3, cfg.EmailRetries)
	assert.Equal(t, 8890, cfg.Port)
}

func TestNewConfigFromEnv_Overrides(t *testing.T) {
	t.Setenv("BI_MAIL_SMTP_HOST", "smtp.example.com")
	t.Setenv("BI_MAIL_SMTP_PORT", "999")
	t.Setenv("BI_MAIL_SMTP_USER", "user1")
	t.Setenv("BI_MAIL_SMTP_PASS", "pass1")
	t.Setenv("BI_MAIL_SMTP_FROM", "noreply@example.com")
	t.Setenv("BI_MAIL_SMTP_SENDER_NAME", "Custom Sender")
	t.Setenv("BI_MAIL_FRONTEND_BASE_URL", "https://app.example.com")
	t.Setenv("BI_MAIL_EMAIL_DEFAULT_LOCALE", "tr")
	t.Setenv("BI_MAIL_EMAIL_DAILY_LIMIT", "50")
	t.Setenv("BI_MAIL_EMAIL_QUEUE_SIZE", "512")
	t.Setenv("BI_MAIL_EMAIL_RETRIES", "5")
	t.Setenv("BI_MAIL_PORT", "9000")

	cfg := NewConfigFromEnv()
	assert.Equal(t, "smtp.example.com", cfg.SMTPHost)
	assert.Equal(t, 999, cfg.SMTPPort)
	assert.Equal(t, "user1", cfg.SMTPUser)
	assert.Equal(t, "pass1", cfg.SMTPPass)
	assert.Equal(t, "noreply@example.com", cfg.SMTPFrom)
	assert.Equal(t, "Custom Sender", cfg.SMTPSenderName)
	assert.Equal(t, "https://app.example.com", cfg.FrontendBaseURL)
	assert.Equal(t, "tr", cfg.EmailDefaultLocale)
	assert.Equal(t, 50, cfg.EmailDailyLimit)
	assert.Equal(t, 512, cfg.EmailQueueSize)
	assert.Equal(t, 5, cfg.EmailRetries)
	assert.Equal(t, 9000, cfg.Port)
}

func TestNewConfigFromEnv_InvalidIntFallback(t *testing.T) {
	t.Setenv("BI_MAIL_SMTP_PORT", "not-a-number")
	t.Setenv("BI_MAIL_EMAIL_DAILY_LIMIT", "-5") // nonNegativeIntEnv: -5 < 0 so invalid
	t.Setenv("BI_MAIL_EMAIL_QUEUE_SIZE", "0")   // positiveIntEnv: 0 is invalid
	t.Setenv("BI_MAIL_EMAIL_RETRIES", "-1")     // nonNegativeIntEnv: -1 is invalid
	t.Setenv("BI_MAIL_PORT", "also-bad")

	cfg := NewConfigFromEnv()
	assert.Equal(t, 587, cfg.SMTPPort, "invalid port falls back")
	assert.Equal(t, 10, cfg.EmailDailyLimit, "-5 falls back to default 10 for nonNegativeIntEnv")
	assert.Equal(t, 256, cfg.EmailQueueSize, "0 falls back for positiveIntEnv")
	assert.Equal(t, 3, cfg.EmailRetries, "-1 falls back for nonNegativeIntEnv")
	assert.Equal(t, 8890, cfg.Port, "invalid port falls back")
}

func TestStringEnv_EmptyReturnsDefault(t *testing.T) {
	assert.Equal(t, "default", stringEnv("UNSET_VAR_12345", "default"))
}

func TestIntEnv_ParseErrorReturnsDefault(t *testing.T) {
	t.Setenv("BI_MAIL_TEST_INT", "not-a-number")
	assert.Equal(t, 42, intEnv("BI_MAIL_TEST_INT", 42))
}

func TestPositiveIntEnv_ZeroReturnsDefault(t *testing.T) {
	t.Setenv("BI_MAIL_TEST_POS", "0")
	assert.Equal(t, 99, positiveIntEnv("BI_MAIL_TEST_POS", 99))
}

func TestPositiveIntEnv_NegativeReturnsDefault(t *testing.T) {
	t.Setenv("BI_MAIL_TEST_POS_NEG", "-1")
	assert.Equal(t, 99, positiveIntEnv("BI_MAIL_TEST_POS_NEG", 99))
}

func TestNonNegativeIntEnv_NegativeReturnsDefault(t *testing.T) {
	t.Setenv("BI_MAIL_TEST_NONNEG", "-1")
	assert.Equal(t, 10, nonNegativeIntEnv("BI_MAIL_TEST_NONNEG", 10))
}

func TestNonNegativeIntEnv_ZeroIsValid(t *testing.T) {
	t.Setenv("BI_MAIL_TEST_NONNEG_ZERO", "0")
	assert.Equal(t, 0, nonNegativeIntEnv("BI_MAIL_TEST_NONNEG_ZERO", 10))
}
