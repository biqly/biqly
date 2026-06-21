package mail

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Test remaining APIClient Send* methods.

func TestAPIClient_SendEmailVerification(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(srv.Close)

	c := NewAPIClient(srv.URL, "tok", nil)
	err := c.SendEmailVerification(context.Background(), "a@b.com", "tok1")
	require.NoError(t, err)
}

func TestAPIClient_SendPasswordReset(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(srv.Close)

	c := NewAPIClient(srv.URL, "tok", nil)
	require.NoError(t, c.SendPasswordReset(context.Background(), "a@b.com", "tok2"))
}

func TestAPIClient_SendEmailChangeConfirmation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(srv.Close)

	c := NewAPIClient(srv.URL, "tok", nil)
	require.NoError(t, c.SendEmailChangeConfirmation(context.Background(), "a@b.com", "tok3", true))
}

func TestAPIClient_SendAccountUnlock(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(srv.Close)

	c := NewAPIClient(srv.URL, "tok", nil)
	require.NoError(t, c.SendAccountUnlock(context.Background(), "a@b.com", "tok4"))
}

func TestAPIClient_SendAccountDeletionScheduled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(srv.Close)

	c := NewAPIClient(srv.URL, "tok", nil)
	require.NoError(t, c.SendAccountDeletionScheduled(context.Background(), "a@b.com", time.Now()))
}

func TestAPIClient_SendDuplicateRegistrationNotice(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(srv.Close)

	c := NewAPIClient(srv.URL, "tok", nil)
	require.NoError(t, c.SendDuplicateRegistrationNotice(context.Background(), "a@b.com"))
}

func TestAPIClient_SendInvitation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(srv.Close)

	c := NewAPIClient(srv.URL, "tok", nil)
	require.NoError(t, c.SendInvitation(context.Background(), "a@b.com", "tok5", "admin", time.Now()))
}

func TestAPIClient_SendDriftAlert(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(srv.Close)

	c := NewAPIClient(srv.URL, "tok", nil)
	require.NoError(t, c.SendDriftAlert(context.Background(), "a@b.com", "model", "desc", []map[string]any{}, "url"))
}

func TestAPIClient_Send_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte("something went wrong"))
	}))
	t.Cleanup(srv.Close)

	c := NewAPIClient(srv.URL, "tok", nil)
	err := c.SendEmailVerification(context.Background(), "a@b.com", "tok")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestAPIClient_Send_NoToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Empty(t, r.Header.Get("X-Internal-Token"))
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(srv.Close)

	c := NewAPIClient(srv.URL, "", nil)
	require.NoError(t, c.SendMagicLink(context.Background(), "a@b.com", "tok"))
}

func TestNewAPIClient_NilHTTPClient(t *testing.T) {
	c := NewAPIClient("http://example.com", "tok", nil)
	assert.NotNil(t, c.httpClient)
	assert.Equal(t, "http://example.com", c.baseURL)
}

// ---------- Server blocked/rate-limited path ----------

func TestServerHandleSend_BlockedRecipient(t *testing.T) {
	blocks := NewMemoryEmailBlockListRepo()
	require.NoError(t, blocks.Block(context.Background(), "blocked@example.com", "bounce", "test"))

	sender, err := NewSMTPEmailSender(&Config{
		FrontendBaseURL:    "https://app.example.com",
		EmailDefaultLocale: "en",
		EmailQueueSize:     1,
	}, blocks, nil)
	require.NoError(t, err)
	t.Cleanup(sender.Close)

	srv := httptest.NewServer(NewServer(sender, "secret-token").Routes())
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		srv.URL+"/internal/mail/send",
		strings.NewReader(`{"template":"verification","to":"blocked@example.com","data":{"token":"x"}}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Token", "secret-token")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()
	// Blocked recipients still return 202 Accepted (suppressed silently).
	assert.Equal(t, http.StatusAccepted, resp.StatusCode)
}

func TestServerHandleSend_InternalError(t *testing.T) {
	// Use a sender with no SMTP host configured to trigger a dispatch error
	sender, err := NewSMTPEmailSender(&Config{
		FrontendBaseURL:    "https://app.example.com",
		EmailDefaultLocale: "en",
		SMTPFrom:           "no-reply@example.com",
		EmailQueueSize:     0, // synchronous
	}, nil, nil)
	require.NoError(t, err)

	srv := httptest.NewServer(NewServer(sender, "secret-token").Routes())
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		srv.URL+"/internal/mail/send",
		strings.NewReader(`{"template":"verification","to":"user@example.com","data":{"token":"x"}}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Token", "secret-token")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()
	assert.Equal(t, http.StatusInternalServerError, resp.StatusCode)
}

func TestServerHandleSend_MissingTemplate(t *testing.T) {
	s := newTestSender(t)
	srv := httptest.NewServer(NewServer(s, "secret-token").Routes())
	t.Cleanup(srv.Close)

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		srv.URL+"/internal/mail/send",
		strings.NewReader(`{"template":"","to":"","data":{}}`))
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Token", "secret-token")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { require.NoError(t, resp.Body.Close()) }()
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
}

func TestServerAuthorized_EmptyToken(t *testing.T) {
	s := &Server{internalToken: ""}
	r := httptest.NewRequestWithContext(context.Background(), http.MethodPost, "/internal/mail/send", http.NoBody)
	assert.False(t, s.authorized(r))
}
