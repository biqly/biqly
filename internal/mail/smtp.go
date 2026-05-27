package mail

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

	"github.com/biqly/biqly/internal/emailaddr"
)

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
	to      string
	headers map[string]string
	body    []byte
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

// ErrUnknownTemplate is returned when SendTemplate is called with a template
// name the sender does not recognize.
var ErrUnknownTemplate = errors.New("unknown email template")

// SendTemplate is the generic entry point used by the worker's HTTP handler.
// It maps a wire-format template name plus a raw data map (tokens, flags,
// device metadata as sent by the auth service) into the render data the
// template expects — building any frontend URLs from the sender's configured
// FrontendBaseURL — then enqueues the message for delivery.
func (s *SMTPEmailSender) SendTemplate(ctx context.Context, template, to string, data map[string]any) error {
	rendered, err := s.buildTemplateData(template, data)
	if err != nil {
		return err
	}
	return s.sendTemplate(ctx, to, template, rendered)
}

// buildTemplateData translates the generic wire payload into the per-template
// render map. URL construction lives here so the mail service is the single
// owner of FrontendBaseURL and link layout.
func (s *SMTPEmailSender) buildTemplateData(template string, data map[string]any) (map[string]any, error) {
	str := func(key string) string {
		if v, ok := data[key].(string); ok {
			return v
		}
		return ""
	}
	displayTime := func(key string) string {
		raw := str(key)
		if t, err := time.Parse(time.RFC3339, raw); err == nil {
			return t.UTC().Format(time.RFC1123)
		}
		return raw
	}

	switch template {
	case "verification":
		return map[string]any{"URL": s.frontendURL(fmt.Sprintf("/auth/verify-email?token=%s", str("token")))}, nil
	case "password_reset":
		return map[string]any{"URL": s.frontendURL(fmt.Sprintf("/auth/reset-password?token=%s", str("token")))}, nil
	case "email_change":
		newEmail, _ := data["new_email"].(bool)
		return map[string]any{
			"URL":      s.frontendURL(fmt.Sprintf("/auth/email-change/confirm?token=%s", str("token"))),
			"NewEmail": newEmail,
		}, nil
	case "account_unlock":
		return map[string]any{"URL": s.frontendURL(fmt.Sprintf("/auth/unlock-account?token=%s", str("token")))}, nil
	case "new_device":
		return map[string]any{
			"OccurredAt":  displayTime("occurred_at"),
			"IPAddress":   str("ip_address"),
			"UserAgent":   str("user_agent"),
			"SecurityURL": s.frontendURL("/auth/security"),
		}, nil
	case "deletion_scheduled":
		return map[string]any{
			"PurgeAt":    displayTime("purge_at"),
			"AccountURL": s.frontendURL("/auth/account"),
		}, nil
	case "duplicate_registration":
		return map[string]any{
			"SignInURL": s.frontendURL("/auth/signin"),
			"ForgotURL": s.frontendURL("/auth/forgot-password"),
		}, nil
	case "magic_link":
		return map[string]any{"URL": s.frontendURL(fmt.Sprintf("/auth/magic-link?token=%s", str("token")))}, nil
	default:
		return nil, fmt.Errorf("%w: %q", ErrUnknownTemplate, template)
	}
}

// sendTemplate is the single entry point shared by all helper methods. It
// runs block-list and rate-limit checks, renders the template, builds the
// multipart message, and dispatches via the queue (async) or directly.
func (s *SMTPEmailSender) sendTemplate(ctx context.Context, to, name string, data map[string]any) error {
	normalized, err := emailaddr.Normalize(to)
	if err != nil {
		return err
	}
	if s.blocks != nil {
		blocked, err := s.blocks.IsBlocked(ctx, normalized)
		if err != nil {
			slog.Warn("email block-list check failed; allowing send", "err", err, "to", emailaddr.Mask(normalized))
		} else if blocked {
			slog.Info("email suppressed by block list", "to", emailaddr.Mask(normalized), "template", name)
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
			slog.Warn("email queue full; sending synchronously", "to", emailaddr.Mask(normalized))
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
		slog.Warn("email rate-limit redis call failed; allowing send", "err", err, "to", emailaddr.Mask(email))
		return nil
	}
	if count == 1 {
		// First send of the day; expire the counter at end of UTC day plus
		// a small buffer so concurrent sends don't reset to zero mid-day.
		_ = s.redis.Expire(ctx, key, 26*time.Hour).Err()
	}
	if int(count) > s.config.EmailDailyLimit {
		slog.Info("email suppressed by daily rate limit", "to", emailaddr.Mask(email), "count", count, "limit", s.config.EmailDailyLimit)
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
			"to", emailaddr.Mask(job.to),
			"err", err,
		)
		if attempt == maxAttempts {
			slog.Error("email permanently failed after retries", "to", emailaddr.Mask(job.to), "err", err)
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
	return smtp.SendMail(addr, auth, s.config.SMTPFrom, []string{job.to}, job.body)
}
