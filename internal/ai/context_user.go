package ai

import (
	"context"

	"github.com/biqly/biqly/internal/ai/prompt"
)

// WithUserID stores the authenticated user id on the context for AI resolution.
func WithUserID(ctx context.Context, userID string) context.Context {
	return prompt.WithUserID(ctx, userID)
}

// UserIDFromContext returns the user id previously stored with WithUserID.
func UserIDFromContext(ctx context.Context) string {
	return prompt.UserIDFromContext(ctx)
}

