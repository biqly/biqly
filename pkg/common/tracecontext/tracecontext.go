// Package tracecontext stores W3C trace context headers on contexts.
package tracecontext

import (
	"context"
	"strings"
)

type contextKey struct{}

// WithTraceparent returns a context carrying traceparent.
func WithTraceparent(ctx context.Context, traceparent string) context.Context {
	traceparent = strings.TrimSpace(traceparent)
	if traceparent == "" {
		return ctx
	}
	return context.WithValue(ctx, contextKey{}, traceparent)
}

// TraceparentFromContext returns the traceparent stored on ctx, if any.
func TraceparentFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	traceparent, _ := ctx.Value(contextKey{}).(string)
	return traceparent
}
