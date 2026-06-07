package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCookieSecure(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		listenPort int
		proto      string
		want       bool
	}{
		{name: "local dev http", listenPort: 8889, want: false},
		{name: "local dev https", listenPort: 8889, proto: "https", want: true},
		{name: "production http", listenPort: 8080, want: true},
		{name: "production https", listenPort: 8080, proto: "https", want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequestWithContext(context.Background(), "GET", "http://example.com/", http.NoBody)
			if tt.proto != "" {
				req.Header.Set("X-Forwarded-Proto", tt.proto)
			}
			if got := CookieSecure(req, tt.listenPort); got != tt.want {
				t.Fatalf("CookieSecure() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCSRFCookieSecureInProduction(t *testing.T) {
	t.Parallel()

	handler := CSRF(8080)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/", http.NoBody))

	var csrfCookie *http.Cookie
	for _, c := range rr.Result().Cookies() {
		if c.Name == "csrf_token" {
			csrfCookie = c
			break
		}
	}
	if csrfCookie == nil {
		t.Fatal("expected csrf_token cookie")
	}
	if !csrfCookie.Secure {
		t.Fatal("expected Secure CSRF cookie outside local dev port")
	}
}
