package mail

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

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

func (m *MockEmailSender) SendInvitation(_ context.Context, email, token, roleName string, expiresAt time.Time) error {
	m.record(email, "invitation", fmt.Sprintf("Invitation token: %s, role: %s, expires: %s", token, roleName, expiresAt.Format(time.RFC3339)))
	return nil
}

