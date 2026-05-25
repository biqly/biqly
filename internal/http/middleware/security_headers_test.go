package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSecurityHeadersDefaults(t *testing.T) {
	handler := SecurityHeaders(SecurityHeadersConfig{})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, "nosniff", rr.Header().Get("X-Content-Type-Options"))
	assert.Equal(t, "DENY", rr.Header().Get("X-Frame-Options"))
	assert.Equal(t, "strict-origin-when-cross-origin", rr.Header().Get("Referrer-Policy"))
	assert.Equal(t, "same-origin", rr.Header().Get("Cross-Origin-Opener-Policy"))
	assert.Empty(t, rr.Header().Get("Strict-Transport-Security"))
	assert.Empty(t, rr.Header().Get("Content-Security-Policy"))
}

func TestSecurityHeadersHSTSAndCSP(t *testing.T) {
	handler := SecurityHeaders(SecurityHeadersConfig{
		HSTSEnabled:           true,
		HSTSIncludeSubdomains: true,
		HSTSPreload:           true,
		ContentSecurityPolicy: "default-src 'self'",
	})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	hsts := rr.Header().Get("Strict-Transport-Security")
	assert.True(t, strings.HasPrefix(hsts, "max-age="))
	assert.Contains(t, hsts, "includeSubDomains")
	assert.Contains(t, hsts, "preload")
	assert.Equal(t, "default-src 'self'", rr.Header().Get("Content-Security-Policy"))
}

func TestSecurityHeadersCustomOverrides(t *testing.T) {
	handler := SecurityHeaders(SecurityHeadersConfig{
		FrameOptions:      "SAMEORIGIN",
		ReferrerPolicy:    "no-referrer",
		PermissionsPolicy: "camera=(), microphone=()",
	})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	assert.Equal(t, "SAMEORIGIN", rr.Header().Get("X-Frame-Options"))
	assert.Equal(t, "no-referrer", rr.Header().Get("Referrer-Policy"))
	assert.Equal(t, "camera=(), microphone=()", rr.Header().Get("Permissions-Policy"))
}
