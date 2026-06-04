package drift

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/biqly/biqly/internal/http/middleware"
	"github.com/biqly/biqly/internal/mail"
)

// Notifier handles user notifications for schema drifts.
type Notifier struct {
	mailClient *mail.APIClient
	authClient *middleware.AuthClient
}

// NewNotifier constructs a Notifier.
func NewNotifier(mailClient *mail.APIClient, authClient *middleware.AuthClient) *Notifier {
	return &Notifier{
		mailClient: mailClient,
		authClient: authClient,
	}
}

// SetAuthClient sets the internal auth client.
func (n *Notifier) SetAuthClient(authClient *middleware.AuthClient) {
	n.authClient = authClient
}

// NotifyOwner alerts the model owner and workspace admins about schema drifts.
func (n *Notifier) NotifyOwner(ctx context.Context, report *DriftReport, modelName string, createdBy *string, frontendBaseURL string) error {
	if n.mailClient == nil {
		slog.Warn("mail client is nil; skipping drift notification", "model_id", report.ModelID)
		return nil
	}

	var recipientEmail string
	if createdBy != nil && *createdBy != "" {
		if n.authClient != nil {
			email, err := n.authClient.GetUserEmail(ctx, *createdBy)
			if err != nil {
				slog.Warn("failed to resolve owner email address", "user_id", *createdBy, "err", err)
			} else {
				recipientEmail = email
			}
		} else if strings.Contains(*createdBy, "@") {
			recipientEmail = *createdBy
		}
	}

	if recipientEmail == "" {
		slog.Warn("cannot resolve recipient email for drift alert", "model_id", report.ModelID, "created_by", createdBy)
		return nil
	}

	// Prepare text version of drifts
	var textBuilder strings.Builder
	for _, item := range report.Drifts {
		sev := strings.ToUpper(GetDriftSeverity(item.Type))
		fmt.Fprintf(&textBuilder, "- [%s] %s: %s\n", sev, item.Type, item.Description)
	}
	driftsText := textBuilder.String()

	// Prepare html drifts slice
	driftsPayload := make([]map[string]any, len(report.Drifts))
	for i, d := range report.Drifts {
		driftsPayload[i] = map[string]any{
			"Severity":    GetDriftSeverity(d.Type),
			"Type":        string(d.Type),
			"Field":       d.Field,
			"ColumnRef":   d.ColumnRef,
			"Description": d.Description,
		}
	}

	// Model URL: frontendBaseURL + /modeling/models/{model_id}
	base := strings.TrimRight(frontendBaseURL, "/")
	if base == "" {
		base = "http://localhost:3000" // fallback
	}
	modelURL := fmt.Sprintf("%s/modeling/models/%s", base, report.ModelID)

	slog.Info("sending drift alert email", "to", recipientEmail, "model", modelName, "drifts", len(report.Drifts))
	err := n.mailClient.SendDriftAlert(ctx, recipientEmail, modelName, driftsText, driftsPayload, modelURL)
	if err != nil {
		return fmt.Errorf("send drift alert email: %w", err)
	}

	return nil
}
