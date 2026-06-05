package mail

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestSender(t *testing.T) *SMTPEmailSender {
	t.Helper()
	cfg := &Config{
		FrontendBaseURL:    "https://app.example.com",
		EmailDefaultLocale: "en",
		EmailQueueSize:     1, // async: enqueue and return before SMTP dispatch
	}
	sender, err := NewSMTPEmailSender(cfg, nil, nil)
	require.NoError(t, err)
	t.Cleanup(sender.Close)
	return sender
}

func TestBuildTemplateData(t *testing.T) {
	s := newTestSender(t)

	v, err := s.buildTemplateData("verification", map[string]any{"token": "abc123"})
	require.NoError(t, err)
	assert.Equal(t, "https://app.example.com/auth/verify-email?token=abc123", v["URL"])

	ec, err := s.buildTemplateData("email_change", map[string]any{"token": "t", "new_email": true})
	require.NoError(t, err)
	assert.Equal(t, true, ec["NewEmail"])
	assert.Contains(t, ec["URL"], "/auth/email-change/confirm?token=t")

	nd, err := s.buildTemplateData("new_device", map[string]any{
		"user_agent":  "curl/8",
		"ip_address":  "1.2.3.4",
		"occurred_at": "2026-05-27T10:00:00Z",
	})
	require.NoError(t, err)
	assert.Equal(t, "curl/8", nd["UserAgent"])
	assert.Equal(t, "1.2.3.4", nd["IPAddress"])
	assert.Contains(t, nd, "SecurityURL")
	assert.NotEmpty(t, nd["OccurredAt"])

	_, err = s.buildTemplateData("does_not_exist", nil)
	assert.ErrorIs(t, err, ErrUnknownTemplate)
}

func TestServerHandleSend(t *testing.T) {
	s := newTestSender(t)
	srv := httptest.NewServer(NewServer(s, "secret-token").Routes())
	t.Cleanup(srv.Close)

	post := func(token, body string) *http.Response {
		req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, srv.URL+"/internal/mail/send", strings.NewReader(body))
		require.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("X-Internal-Token", token)
		}
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		return resp
	}
	t.Run("missing token rejected", func(t *testing.T) {
		resp := post("", `{"template":"verification","to":"a@b.com","data":{"token":"x"}}`)
		defer func() { require.NoError(t, resp.Body.Close()) }()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("wrong token rejected", func(t *testing.T) {
		resp := post("nope", `{"template":"verification","to":"a@b.com","data":{"token":"x"}}`)
		defer func() { require.NoError(t, resp.Body.Close()) }()
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("unknown template is 400", func(t *testing.T) {
		resp := post("secret-token", `{"template":"bogus","to":"a@b.com","data":{}}`)
		defer func() { require.NoError(t, resp.Body.Close()) }()
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("invalid recipient is 400", func(t *testing.T) {
		resp := post("secret-token", `{"template":"verification","to":"not-an-email","data":{"token":"x"}}`)
		defer func() { require.NoError(t, resp.Body.Close()) }()
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	})

	t.Run("valid request is accepted", func(t *testing.T) {
		resp := post("secret-token", `{"template":"verification","to":"user@example.com","data":{"token":"x"}}`)
		defer func() { require.NoError(t, resp.Body.Close()) }()
		assert.Equal(t, http.StatusAccepted, resp.StatusCode)
	})
}

func TestAPIClientRoundTrip(t *testing.T) {
	var gotToken string
	var gotReq sendRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("X-Internal-Token")
		body, err := io.ReadAll(r.Body)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(body, &gotReq))
		w.WriteHeader(http.StatusAccepted)
	}))
	t.Cleanup(srv.Close)

	client := NewAPIClient(srv.URL, "tok", nil)

	require.NoError(t, client.SendMagicLink(context.Background(), "user@example.com", "magic-tok"))
	assert.Equal(t, "tok", gotToken)
	assert.Equal(t, "magic_link", gotReq.Template)
	assert.Equal(t, "user@example.com", gotReq.To)
	assert.Equal(t, "magic-tok", gotReq.Data["token"])

	require.NoError(t, client.SendNewDeviceLogin(context.Background(), "user@example.com", DeviceLoginInfo{
		UserAgent:  "Firefox",
		IPAddress:  "9.9.9.9",
		OccurredAt: time.Date(2026, 5, 27, 10, 0, 0, 0, time.UTC),
	}))
	assert.Equal(t, "new_device", gotReq.Template)
	assert.Equal(t, "Firefox", gotReq.Data["user_agent"])
	assert.Equal(t, "9.9.9.9", gotReq.Data["ip_address"])
	assert.Equal(t, "2026-05-27T10:00:00Z", gotReq.Data["occurred_at"])
}
