package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPublicEmbedHeaders(t *testing.T) {
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	// Simulate the router: strict SecurityHeaders outside, PublicEmbedHeaders inside.
	h := SecurityHeaders(SecurityHeadersConfig{ContentSecurityPolicy: "default-src 'self'; frame-ancestors 'none'"})(
		PublicEmbedHeaders(inner))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/public/dashboards/tok", http.NoBody))

	assert.Empty(t, rec.Header().Get("X-Frame-Options"))
	assert.Equal(t, "frame-ancestors *", rec.Header().Get("Content-Security-Policy"))
	assert.Equal(t, "cross-origin", rec.Header().Get("Cross-Origin-Resource-Policy"))
	assert.Equal(t, "noindex, nofollow", rec.Header().Get("X-Robots-Tag"))
	assert.Equal(t, "nosniff", rec.Header().Get("X-Content-Type-Options"), "other hardening must survive")
}

func TestPublicRateLimiter_NilClientPassesThrough(t *testing.T) {
	rl := NewPublicRateLimiter(nil, 1)
	called := 0
	h := rl.Middleware()(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) { called++ }))
	for range 5 {
		h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/x", http.NoBody))
	}
	assert.Equal(t, 5, called, "nil redis must fail open")
}
