package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// sendRequest is the wire payload for POST /internal/mail/send.
type sendRequest struct {
	Template string         `json:"template"`
	To       string         `json:"to"`
	Data     map[string]any `json:"data,omitempty"`
}

// APIClient implements EmailSender by forwarding each request to a standalone
// mail worker over HTTP. Delivery is asynchronous on the worker side: a 2xx
// response means the message was accepted for delivery, not that SMTP
// succeeded. Callers treat send failures as non-fatal (fire-and-forget).
type APIClient struct {
	baseURL       string
	internalToken string
	httpClient    *http.Client
}

// NewAPIClient builds a client targeting the mail worker at baseURL, attaching
// internalToken to every request via the X-Internal-Token header.
func NewAPIClient(baseURL, internalToken string, httpClient *http.Client) *APIClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Second}
	}
	return &APIClient{
		baseURL:       strings.TrimRight(baseURL, "/"),
		internalToken: internalToken,
		httpClient:    httpClient,
	}
}

func (c *APIClient) send(ctx context.Context, template, to string, data map[string]any) (err error) {
	body, err := json.Marshal(sendRequest{Template: template, To: to, Data: data})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/internal/mail/send", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.internalToken != "" {
		req.Header.Set("X-Internal-Token", c.internalToken)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		snippet, err := io.ReadAll(io.LimitReader(resp.Body, 512))
		if err != nil {
			return fmt.Errorf("read mail service error response: %w", err)
		}
		return fmt.Errorf("mail service returned %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	return nil
}

func (c *APIClient) SendEmailVerification(ctx context.Context, email, token string) error {
	return c.send(ctx, "verification", email, map[string]any{"token": token})
}

func (c *APIClient) SendPasswordReset(ctx context.Context, email, token string) error {
	return c.send(ctx, "password_reset", email, map[string]any{"token": token})
}

func (c *APIClient) SendEmailChangeConfirmation(ctx context.Context, email, token string, newEmail bool) error {
	return c.send(ctx, "email_change", email, map[string]any{"token": token, "new_email": newEmail})
}

func (c *APIClient) SendAccountUnlock(ctx context.Context, email, token string) error {
	return c.send(ctx, "account_unlock", email, map[string]any{"token": token})
}

func (c *APIClient) SendNewDeviceLogin(ctx context.Context, email string, info DeviceLoginInfo) error {
	return c.send(ctx, "new_device", email, map[string]any{
		"user_agent":  info.UserAgent,
		"ip_address":  info.IPAddress,
		"occurred_at": info.OccurredAt.UTC().Format(time.RFC3339),
	})
}

func (c *APIClient) SendAccountDeletionScheduled(ctx context.Context, email string, purgeAt time.Time) error {
	return c.send(ctx, "deletion_scheduled", email, map[string]any{
		"purge_at": purgeAt.UTC().Format(time.RFC3339),
	})
}

func (c *APIClient) SendDuplicateRegistrationNotice(ctx context.Context, email string) error {
	return c.send(ctx, "duplicate_registration", email, nil)
}

func (c *APIClient) SendMagicLink(ctx context.Context, email, token string) error {
	return c.send(ctx, "magic_link", email, map[string]any{"token": token})
}

func (c *APIClient) SendInvitation(ctx context.Context, email, token, roleName string, expiresAt time.Time) error {
	return c.send(ctx, "invitation", email, map[string]any{
		"token":      token,
		"role_name":  roleName,
		"expires_at": expiresAt.UTC().Format(time.RFC3339),
	})
}

func (c *APIClient) SendDriftAlert(ctx context.Context, email string, modelName string, driftsText string, drifts []map[string]any, modelURL string) error {
	return c.send(ctx, "drift_alert", email, map[string]any{
		"ModelName":  modelName,
		"DriftsText": driftsText,
		"Drifts":     drifts,
		"ModelURL":   modelURL,
	})
}
