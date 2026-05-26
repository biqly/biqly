package auth

import (
	"context"
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"
	"time"
)

type EmailSender interface {
	SendEmailVerification(ctx context.Context, email, token string) error
	SendPasswordReset(ctx context.Context, email, token string) error
	SendEmailChangeConfirmation(ctx context.Context, email, token string, newEmail bool) error
	SendAccountUnlock(ctx context.Context, email, token string) error
	SendNewDeviceLogin(ctx context.Context, email string, info DeviceLoginInfo) error
	SendAccountDeletionScheduled(ctx context.Context, email string, purgeAt time.Time) error
	SendDuplicateRegistrationNotice(ctx context.Context, email string) error
}

type DeviceLoginInfo struct {
	UserAgent string
	IPAddress string
	OccurredAt time.Time
}

type SMTPEmailSender struct {
	config *Config
}

func NewSMTPEmailSender(cfg *Config) *SMTPEmailSender {
	return &SMTPEmailSender{config: cfg}
}

func (s *SMTPEmailSender) frontendURL(path string) string {
	base := strings.TrimRight(s.config.FrontendBaseURL, "/")
	if base == "" {
		base = "http://localhost:3333"
	}
	return base + path
}

func (s *SMTPEmailSender) SendEmailVerification(ctx context.Context, email, token string) error {
	verificationURL := s.frontendURL(fmt.Sprintf("/auth/verify-email?token=%s", token))
	subject := "Verify your Biqly account"
	body := fmt.Sprintf("Please verify your account by clicking the following link:\n%s\n\nThis link will expire in 24 hours.", verificationURL)
	return s.send(email, subject, body)
}

func (s *SMTPEmailSender) SendPasswordReset(ctx context.Context, email, token string) error {
	resetURL := s.frontendURL(fmt.Sprintf("/auth/reset-password?token=%s", token))
	subject := "Reset your Biqly password"
	body := fmt.Sprintf("You requested to reset your password. Click the link below to set a new password:\n%s\n\nThis link will expire in 1 hour.", resetURL)
	return s.send(email, subject, body)
}

func (s *SMTPEmailSender) SendEmailChangeConfirmation(ctx context.Context, email, token string, newEmail bool) error {
	confirmURL := s.frontendURL(fmt.Sprintf("/auth/email-change/confirm?token=%s", token))
	subject := "Confirm your Biqly email change"
	body := fmt.Sprintf("Confirm this email change by clicking the following link:\n%s\n\nThis link will expire in 48 hours.", confirmURL)
	if newEmail {
		body = fmt.Sprintf("Confirm this as your new Biqly email address:\n%s\n\nThis link will expire in 48 hours.", confirmURL)
	}
	return s.send(email, subject, body)
}

func (s *SMTPEmailSender) SendAccountUnlock(ctx context.Context, email, token string) error {
	unlockURL := s.frontendURL(fmt.Sprintf("/auth/unlock-account?token=%s", token))
	subject := "Your Biqly account is locked"
	body := fmt.Sprintf("Your account was locked due to multiple failed sign-in attempts.\n\nIf this was you, click below to unlock and reset your password:\n%s\n\nThis link expires in 1 hour. If you didn't try to sign in, please change your password immediately.", unlockURL)
	return s.send(email, subject, body)
}

func (s *SMTPEmailSender) SendNewDeviceLogin(ctx context.Context, email string, info DeviceLoginInfo) error {
	subject := "New sign-in to your Biqly account"
	body := fmt.Sprintf(
		"We detected a sign-in to your Biqly account from a new device.\n\nTime:       %s\nIP address: %s\nDevice:     %s\n\nIf this wasn't you, change your password and revoke active sessions from the security page:\n%s",
		info.OccurredAt.UTC().Format(time.RFC1123),
		info.IPAddress,
		info.UserAgent,
		s.frontendURL("/auth/security"),
	)
	return s.send(email, subject, body)
}

func (s *SMTPEmailSender) SendDuplicateRegistrationNotice(ctx context.Context, email string) error {
	subject := "Sign-up attempt on existing Biqly account"
	body := fmt.Sprintf(
		"Someone tried to register a new Biqly account using this email address, but an account already exists.\n\nIf this was you, you can sign in at %s or reset your password at %s.\n\nIf you did not attempt this, you can safely ignore this email.",
		s.frontendURL("/auth/signin"),
		s.frontendURL("/auth/forgot-password"),
	)
	return s.send(email, subject, body)
}

func (s *SMTPEmailSender) SendAccountDeletionScheduled(ctx context.Context, email string, purgeAt time.Time) error {
	subject := "Your Biqly account is scheduled for deletion"
	body := fmt.Sprintf(
		"Your Biqly account has been scheduled for deletion. All personal data will be permanently removed on %s (UTC).\n\nTo cancel deletion before that date, sign in and restore your account from %s.",
		purgeAt.UTC().Format(time.RFC1123),
		s.frontendURL("/auth/account"),
	)
	return s.send(email, subject, body)
}

func (s *SMTPEmailSender) send(to, subject, body string) error {
	if s.config.SMTPHost == "" {
		return fmt.Errorf("SMTP host is not configured")
	}

	addr := fmt.Sprintf("%s:%d", s.config.SMTPHost, s.config.SMTPPort)
	msg := []byte(fmt.Sprintf("To: %s\r\nSubject: %s\r\n\r\n%s\r\n", to, subject, body))
	auth := smtp.PlainAuth("", s.config.SMTPUser, s.config.SMTPPass, s.config.SMTPHost)

	//nolint:gosec
	err := smtp.SendMail(addr, auth, s.config.SMTPFrom, []string{to}, msg)
	if err != nil {
		return fmt.Errorf("send email via SMTP: %w", err)
	}
	return nil
}

type MockEmailSender struct {
	SentEmails map[string][]string
}

func NewMockEmailSender() *MockEmailSender {
	return &MockEmailSender{
		SentEmails: make(map[string][]string),
	}
}

func (m *MockEmailSender) SendEmailVerification(ctx context.Context, email, token string) error {
	msg := fmt.Sprintf("Verification token: %s", token)
	m.SentEmails[email] = append(m.SentEmails[email], msg)
	slog.Info("MOCK EMAIL SENT: Verification email", "to", email, "token", token)
	return nil
}

func (m *MockEmailSender) SendPasswordReset(ctx context.Context, email, token string) error {
	msg := fmt.Sprintf("Reset token: %s", token)
	m.SentEmails[email] = append(m.SentEmails[email], msg)
	slog.Info("MOCK EMAIL SENT: Password reset email", "to", email, "token", token)
	return nil
}

func (m *MockEmailSender) SendEmailChangeConfirmation(ctx context.Context, email, token string, newEmail bool) error {
	msg := fmt.Sprintf("Email change token: %s", token)
	if newEmail {
		msg = fmt.Sprintf("New email change token: %s", token)
	}
	m.SentEmails[email] = append(m.SentEmails[email], msg)
	slog.Info("MOCK EMAIL SENT: Email change confirmation", "to", email, "token", token, "new_email", newEmail)
	return nil
}

func (m *MockEmailSender) SendAccountUnlock(ctx context.Context, email, token string) error {
	m.SentEmails[email] = append(m.SentEmails[email], fmt.Sprintf("Unlock token: %s", token))
	slog.Info("MOCK EMAIL SENT: Account unlock", "to", email, "token", token)
	return nil
}

func (m *MockEmailSender) SendNewDeviceLogin(ctx context.Context, email string, info DeviceLoginInfo) error {
	m.SentEmails[email] = append(m.SentEmails[email], fmt.Sprintf("New device: %s @ %s", info.UserAgent, info.IPAddress))
	slog.Info("MOCK EMAIL SENT: New device login", "to", email, "ip", info.IPAddress, "ua", info.UserAgent)
	return nil
}

func (m *MockEmailSender) SendDuplicateRegistrationNotice(ctx context.Context, email string) error {
	m.SentEmails[email] = append(m.SentEmails[email], "Duplicate registration attempt")
	slog.Info("MOCK EMAIL SENT: Duplicate registration", "to", email)
	return nil
}

func (m *MockEmailSender) SendAccountDeletionScheduled(ctx context.Context, email string, purgeAt time.Time) error {
	m.SentEmails[email] = append(m.SentEmails[email], fmt.Sprintf("Deletion scheduled for: %s", purgeAt.Format(time.RFC3339)))
	slog.Info("MOCK EMAIL SENT: Account deletion scheduled", "to", email, "purge_at", purgeAt)
	return nil
}
