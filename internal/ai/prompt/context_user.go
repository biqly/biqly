package prompt

import "context"

type userIDContextKey struct{}

// WithUserID stores the authenticated user id on the context for AI resolution.
func WithUserID(ctx context.Context, userID string) context.Context {
	if userID == "" {
		return ctx
	}
	return context.WithValue(ctx, userIDContextKey{}, userID)
}

// UserIDFromContext returns the user id previously stored with WithUserID.
func UserIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(userIDContextKey{}).(string)
	return v
}
