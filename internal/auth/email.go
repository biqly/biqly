package auth

import (
	"context"
	"fmt"
	"log/slog"
	"net/smtp"
)

type EmailSender interface {
	SendEmailVerification(ctx context.Context, email, token string) error
	SendPasswordReset(ctx context.Context, email, token string) error
	SendEmailChangeConfirmation(ctx context.Context, email, token string, newEmail bool) error
}

type SMTPEmailSender struct {
	config *Config
}

func NewSMTPEmailSender(cfg *Config) *SMTPEmailSender {
	return &SMTPEmailSender{config: cfg}
}

func (s *SMTPEmailSender) SendEmailVerification(ctx context.Context, email, token string) error {
	verificationURL := fmt.Sprintf("http://localhost:3333/auth/verify-email?token=%s", token)
	subject := "Verify your Biqly account"
	body := fmt.Sprintf("Please verify your account by clicking the following link:\n%s\n\nThis link will expire in 24 hours.", verificationURL)
	return s.send(email, subject, body)
}

func (s *SMTPEmailSender) SendPasswordReset(ctx context.Context, email, token string) error {
	resetURL := fmt.Sprintf("http://localhost:3333/auth/reset-password?token=%s", token)
	subject := "Reset your Biqly password"
	body := fmt.Sprintf("You requested to reset your password. Click the link below to set a new password:\n%s\n\nThis link will expire in 1 hour.", resetURL)
	return s.send(email, subject, body)
}

func (s *SMTPEmailSender) SendEmailChangeConfirmation(ctx context.Context, email, token string, newEmail bool) error {
	confirmURL := fmt.Sprintf("http://localhost:3333/auth/email-change/confirm?token=%s", token)
	subject := "Confirm your Biqly email change"
	body := fmt.Sprintf("Confirm this email change by clicking the following link:\n%s\n\nThis link will expire in 48 hours.", confirmURL)
	if newEmail {
		body = fmt.Sprintf("Confirm this as your new Biqly email address:\n%s\n\nThis link will expire in 48 hours.", confirmURL)
	}
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
