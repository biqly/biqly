package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/smtp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// ErrEmailBlocked is returned when the destination address is on the
// transactional block list (bounced, marked-as-spam, manually blocked).
var ErrEmailBlocked = errors.New("email address is blocked")

// ErrEmailRateLimited is returned when the destination address has already
// received the configured maximum number of emails for the current day.
var ErrEmailRateLimited = errors.New("daily email rate limit exceeded for recipient")

type EmailSender interface {
	SendEmailVerification(ctx context.Context, email, token string) error
	SendPasswordReset(ctx context.Context, email, token string) error
	SendEmailChangeConfirmation(ctx context.Context, email, token string, newEmail bool) error
	SendAccountUnlock(ctx context.Context, email, token string) error
	SendNewDeviceLogin(ctx context.Context, email string, info DeviceLoginInfo) error
	SendAccountDeletionScheduled(ctx context.Context, email string, purgeAt time.Time) error
	SendDuplicateRegistrationNotice(ctx context.Context, email string) error
	SendMagicLink(ctx context.Context, email, token string) error
}

type DeviceLoginInfo struct {
	UserAgent  string
	IPAddress  string
	OccurredAt time.Time
}

// SMTPEmailSender renders transactional emails from the localized template
// registry, applies block-list and per-recipient daily rate-limit checks,
// and dispatches via SMTP — optionally through a bounded retrying queue.
type SMTPEmailSender struct {
	config   *Config
	registry *emailTemplateRegistry
	blocks   EmailBlockListRepo
	redis    *redis.Client
	queue    chan emailJob
	stop     chan struct{}
	wg       sync.WaitGroup
}

type emailJob struct {
	to       string
	headers  map[string]string
	body     []byte
	attempts int
}

// NewSMTPEmailSender constructs a sender wired to the config's SMTP settings
// and the supplied block list / redis dependencies. Either dependency may be
// nil to disable the corresponding check. When EmailQueueSize > 0 the sender
// dispatches asynchronously and retries transient SMTP failures up to
// EmailRetries times with exponential backoff (1s, 4s, 16s, ...).
func NewSMTPEmailSender(cfg *Config, blocks EmailBlockListRepo, rdb *redis.Client) (*SMTPEmailSender, error) {
	registry, err := newEmailTemplateRegistry(cfg.EmailDefaultLocale)
	if err != nil {
		return nil, err
	}
	s := &SMTPEmailSender{
		config:   cfg,
		registry: registry,
		blocks:   blocks,
		redis:    rdb,
		stop:     make(chan struct{}),
	}
	if cfg.EmailQueueSize > 0 {
		s.queue = make(chan emailJob, cfg.EmailQueueSize)
		s.wg.Add(1)
		go s.runWorker()
	}
	return s, nil
}

// MustSMTPEmailSender mirrors NewSMTPEmailSender but panics on configuration
// errors. It is intended for bootstrap code (main.go) where a template
// compile failure indicates a programming bug, not a runtime error.
func MustSMTPEmailSender(cfg *Config, blocks EmailBlockListRepo, rdb *redis.Client) *SMTPEmailSender {
	s, err := NewSMTPEmailSender(cfg, blocks, rdb)
	if err != nil {
		panic(err)
	}
	return s
}

// Close drains in-flight queued mail and stops the worker. Safe to call
// multiple times.
func (s *SMTPEmailSender) Close() {
	if s.queue == nil {
		return
	}
	select {
	case <-s.stop:
		return
	default:
	}
	close(s.stop)
	close(s.queue)
	s.wg.Wait()
}

func (s *SMTPEmailSender) frontendURL(path string) string {
	base := strings.TrimRight(s.config.FrontendBaseURL, "/")
	if base == "" {
		base = "http://localhost:3333"
	}
	return base + path
}

func (s *SMTPEmailSender) SendEmailVerification(ctx context.Context, email, token string) error {
	url := s.frontendURL(fmt.Sprintf("/auth/verify-email?token=%s", token))
	return s.sendTemplate(ctx, email, "verification", map[string]any{"URL": url})
}

func (s *SMTPEmailSender) SendPasswordReset(ctx context.Context, email, token string) error {
	url := s.frontendURL(fmt.Sprintf("/auth/reset-password?token=%s", token))
	return s.sendTemplate(ctx, email, "password_reset", map[string]any{"URL": url})
}

func (s *SMTPEmailSender) SendEmailChangeConfirmation(ctx context.Context, email, token string, newEmail bool) error {
	url := s.frontendURL(fmt.Sprintf("/auth/email-change/confirm?token=%s", token))
	return s.sendTemplate(ctx, email, "email_change", map[string]any{"URL": url, "NewEmail": newEmail})
}

func (s *SMTPEmailSender) SendAccountUnlock(ctx context.Context, email, token string) error {
	url := s.frontendURL(fmt.Sprintf("/auth/unlock-account?token=%s", token))
	return s.sendTemplate(ctx, email, "account_unlock", map[string]any{"URL": url})
}

func (s *SMTPEmailSender) SendNewDeviceLogin(ctx context.Context, email string, info DeviceLoginInfo) error {
	return s.sendTemplate(ctx, email, "new_device", map[string]any{
		"OccurredAt":  info.OccurredAt.UTC().Format(time.RFC1123),
		"IPAddress":   info.IPAddress,
		"UserAgent":   info.UserAgent,
		"SecurityURL": s.frontendURL("/auth/security"),
	})
}

func (s *SMTPEmailSender) SendAccountDeletionScheduled(ctx context.Context, email string, purgeAt time.Time) error {
	return s.sendTemplate(ctx, email, "deletion_scheduled", map[string]any{
		"PurgeAt":    purgeAt.UTC().Format(time.RFC1123),
		"AccountURL": s.frontendURL("/auth/account"),
	})
}

func (s *SMTPEmailSender) SendDuplicateRegistrationNotice(ctx context.Context, email string) error {
	return s.sendTemplate(ctx, email, "duplicate_registration", map[string]any{
		"SignInURL": s.frontendURL("/auth/signin"),
		"ForgotURL": s.frontendURL("/auth/forgot-password"),
	})
}

func (s *SMTPEmailSender) SendMagicLink(ctx context.Context, email, token string) error {
	url := s.frontendURL(fmt.Sprintf("/auth/magic-link?token=%s", token))
	return s.sendTemplate(ctx, email, "magic_link", map[string]any{"URL": url})
}

// sendTemplate is the single entry point shared by all helper methods. It
// runs block-list and rate-limit checks, renders the template, builds the
// multipart message, and dispatches via the queue (async) or directly.
func (s *SMTPEmailSender) sendTemplate(ctx context.Context, to, name string, data map[string]any) error {
	normalized, err := NormalizeEmail(to)
	if err != nil {
		return err
	}
	if s.blocks != nil {
		blocked, err := s.blocks.IsBlocked(ctx, normalized)
		if err != nil {
			slog.Warn("email block-list check failed; allowing send", "err", err, "to", MaskEmail(normalized))
		} else if blocked {
			slog.Info("email suppressed by block list", "to", MaskEmail(normalized), "template", name)
			return ErrEmailBlocked
		}
	}
	if err := s.checkRateLimit(ctx, normalized); err != nil {
		return err
	}

	subject, textBody, htmlBody, err := s.registry.Render(name, s.config.EmailDefaultLocale, data)
	if err != nil {
		return err
	}
	headers := map[string]string{
		"From":             s.config.SMTPFrom,
		"To":               normalized,
		"Subject":          subject,
		"Auto-Submitted":   "auto-generated",
		"List-Unsubscribe": fmt.Sprintf("<mailto:%s?subject=unsubscribe>", s.config.SMTPFrom),
	}
	msg, err := buildMultipartMessage(headers, textBody, htmlBody)
	if err != nil {
		return err
	}

	job := emailJob{to: normalized, headers: headers, body: msg}
	if s.queue != nil {
		select {
		case s.queue <- job:
			return nil
		default:
			slog.Warn("email queue full; sending synchronously", "to", MaskEmail(normalized))
			return s.dispatch(job)
		}
	}
	return s.dispatch(job)
}

func (s *SMTPEmailSender) checkRateLimit(ctx context.Context, email string) error {
	if s.redis == nil || s.config.EmailDailyLimit <= 0 {
		return nil
	}
	day := time.Now().UTC().Format("20060102")
	key := "email_count:" + email + ":" + day
	count, err := s.redis.Incr(ctx, key).Result()
	if err != nil {
		slog.Warn("email rate-limit redis call failed; allowing send", "err", err, "to", MaskEmail(email))
		return nil
	}
	if count == 1 {
		// First send of the day; expire the counter at end of UTC day plus
		// a small buffer so concurrent sends don't reset to zero mid-day.
		_ = s.redis.Expire(ctx, key, 26*time.Hour).Err()
	}
	if int(count) > s.config.EmailDailyLimit {
		slog.Info("email suppressed by daily rate limit", "to", MaskEmail(email), "count", count, "limit", s.config.EmailDailyLimit)
		return ErrEmailRateLimited
	}
	return nil
}

func (s *SMTPEmailSender) runWorker() {
	defer s.wg.Done()
	for job := range s.queue {
		s.handleJob(job)
	}
}

func (s *SMTPEmailSender) handleJob(job emailJob) {
	maxAttempts := s.config.EmailRetries + 1
	if maxAttempts < 1 {
		maxAttempts = 1
	}
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err := s.dispatch(job)
		if err == nil {
			return
		}
		slog.Warn("email dispatch failed",
			"attempt", attempt,
			"max_attempts", maxAttempts,
			"to", MaskEmail(job.to),
			"err", err,
		)
		if attempt == maxAttempts {
			slog.Error("email permanently failed after retries", "to", MaskEmail(job.to), "err", err)
			return
		}
		backoff := time.Duration(1<<(2*(attempt-1))) * time.Second // 1s, 4s, 16s, 64s...
		select {
		case <-s.stop:
			return
		case <-time.After(backoff):
		}
	}
}

func (s *SMTPEmailSender) dispatch(job emailJob) error {
	if s.config.SMTPHost == "" {
		return fmt.Errorf("SMTP host is not configured")
	}
	addr := s.config.SMTPHost + ":" + strconv.Itoa(s.config.SMTPPort)
	auth := smtp.PlainAuth("", s.config.SMTPUser, s.config.SMTPPass, s.config.SMTPHost)
	//nolint:gosec // PlainAuth over TLS is the SMTP submission norm; transport is configured separately.
	return smtp.SendMail(addr, auth, s.config.SMTPFrom, []string{job.to}, job.body)
}

// MockEmailSender records sent messages for tests. It implements EmailSender
// directly and skips block-list / rate-limit / queueing concerns.
type MockEmailSender struct {
	SentEmails map[string][]string
}

func NewMockEmailSender() *MockEmailSender {
	return &MockEmailSender{
		SentEmails: make(map[string][]string),
	}
}

func (m *MockEmailSender) record(email, kind, info string) {
	m.SentEmails[email] = append(m.SentEmails[email], info)
	slog.Info("MOCK EMAIL SENT", "kind", kind, "to", email, "info", info)
}

func (m *MockEmailSender) SendEmailVerification(_ context.Context, email, token string) error {
	m.record(email, "verification", "Verification token: "+token)
	return nil
}

func (m *MockEmailSender) SendPasswordReset(_ context.Context, email, token string) error {
	m.record(email, "password_reset", "Reset token: "+token)
	return nil
}

func (m *MockEmailSender) SendEmailChangeConfirmation(_ context.Context, email, token string, newEmail bool) error {
	if newEmail {
		m.record(email, "email_change_new", "New email change token: "+token)
	} else {
		m.record(email, "email_change_old", "Email change token: "+token)
	}
	return nil
}

func (m *MockEmailSender) SendAccountUnlock(_ context.Context, email, token string) error {
	m.record(email, "account_unlock", "Unlock token: "+token)
	return nil
}

func (m *MockEmailSender) SendNewDeviceLogin(_ context.Context, email string, info DeviceLoginInfo) error {
	m.record(email, "new_device", fmt.Sprintf("New device: %s @ %s", info.UserAgent, info.IPAddress))
	return nil
}

func (m *MockEmailSender) SendAccountDeletionScheduled(_ context.Context, email string, purgeAt time.Time) error {
	m.record(email, "deletion_scheduled", "Deletion scheduled for: "+purgeAt.Format(time.RFC3339))
	return nil
}

func (m *MockEmailSender) SendDuplicateRegistrationNotice(_ context.Context, email string) error {
	m.record(email, "duplicate_registration", "Duplicate registration")
	return nil
}

func (m *MockEmailSender) SendMagicLink(_ context.Context, email, token string) error {
	m.record(email, "magic_link", "Magic link token: "+token)
	return nil
}
