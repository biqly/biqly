package mail

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewMockEmailSender(t *testing.T) {
	m := NewMockEmailSender()
	assert.NotNil(t, m)
	assert.NotNil(t, m.SentEmails)
}

func TestMockEmailSender_SendEmailVerification(t *testing.T) {
	m := NewMockEmailSender()
	err := m.SendEmailVerification(t.Context(), "user@example.com", "tok1")
	assert.NoError(t, err)
	assert.Contains(t, m.SentEmails["user@example.com"][0], "tok1")
}

func TestMockEmailSender_SendPasswordReset(t *testing.T) {
	m := NewMockEmailSender()
	err := m.SendPasswordReset(t.Context(), "user@example.com", "tok2")
	assert.NoError(t, err)
	assert.Contains(t, m.SentEmails["user@example.com"][0], "tok2")
}

func TestMockEmailSender_SendEmailChangeConfirmation(t *testing.T) {
	m := NewMockEmailSender()
	err := m.SendEmailChangeConfirmation(t.Context(), "user@example.com", "tok3", true)
	assert.NoError(t, err)
	assert.Contains(t, m.SentEmails["user@example.com"][0], "New email change")

	err = m.SendEmailChangeConfirmation(t.Context(), "user@example.com", "tok4", false)
	assert.NoError(t, err)
	assert.Contains(t, m.SentEmails["user@example.com"][1], "Email change")
}

func TestMockEmailSender_SendAccountUnlock(t *testing.T) {
	m := NewMockEmailSender()
	err := m.SendAccountUnlock(t.Context(), "user@example.com", "tok5")
	assert.NoError(t, err)
	assert.Contains(t, m.SentEmails["user@example.com"][0], "tok5")
}

func TestMockEmailSender_SendNewDeviceLogin(t *testing.T) {
	m := NewMockEmailSender()
	err := m.SendNewDeviceLogin(t.Context(), "user@example.com", DeviceLoginInfo{
		UserAgent: "chrome", IPAddress: "1.2.3.4",
	})
	assert.NoError(t, err)
	assert.Contains(t, m.SentEmails["user@example.com"][0], "chrome")
}

func TestMockEmailSender_SendAccountDeletionScheduled(t *testing.T) {
	m := NewMockEmailSender()
	err := m.SendAccountDeletionScheduled(t.Context(), "user@example.com", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	assert.NoError(t, err)
	assert.Contains(t, m.SentEmails["user@example.com"][0], "2026-06-01")
}

func TestMockEmailSender_SendDuplicateRegistrationNotice(t *testing.T) {
	m := NewMockEmailSender()
	err := m.SendDuplicateRegistrationNotice(t.Context(), "user@example.com")
	assert.NoError(t, err)
	assert.Contains(t, m.SentEmails["user@example.com"][0], "Duplicate")
}

func TestMockEmailSender_SendMagicLink(t *testing.T) {
	m := NewMockEmailSender()
	err := m.SendMagicLink(t.Context(), "user@example.com", "linktok")
	assert.NoError(t, err)
	assert.Contains(t, m.SentEmails["user@example.com"][0], "linktok")
}

func TestMockEmailSender_SendInvitation(t *testing.T) {
	m := NewMockEmailSender()
	err := m.SendInvitation(t.Context(), "user@example.com", "invitetok", "admin", time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC))
	assert.NoError(t, err)
	assert.Contains(t, m.SentEmails["user@example.com"][0], "invitetok")
}

func TestMockEmailSender_SendDriftAlert(t *testing.T) {
	m := NewMockEmailSender()
	err := m.SendDriftAlert(t.Context(), "user@example.com", "sales_model", "drift desc", []map[string]any{{"key": "val"}}, "https://example.com/model")
	assert.NoError(t, err)
	assert.Contains(t, m.SentEmails["user@example.com"][0], "sales_model")
}
