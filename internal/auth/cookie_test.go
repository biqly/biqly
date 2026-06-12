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

func TestProductionCookieSecureFailClosed(t *testing.T) {
	req := httptest.NewRequestWithContext(context.Background(), "GET", "http://example.com/", http.NoBody)

	t.Run("production env", func(t *testing.T) {
		t.Setenv("BI_ENV", "production")
		t.Setenv("KUBERNETES_SERVICE_HOST", "")
		if !CookieSecure(req, localHTTPDevPort) {
			t.Fatal("CookieSecure must be true in production even on local dev port over HTTP")
		}
	})

	t.Run("kubernetes", func(t *testing.T) {
		t.Setenv("BI_ENV", "development")
		t.Setenv("KUBERNETES_SERVICE_HOST", "10.0.0.1")
		if !CookieSecure(req, localHTTPDevPort) {
			t.Fatal("CookieSecure must be true in Kubernetes even on local dev port over HTTP")
		}
	})
}

func TestWriteResponseCookieInsecureOnLocalDevPort(t *testing.T) {
	t.Setenv("BI_ENV", "development")
	t.Setenv("KUBERNETES_SERVICE_HOST", "")

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), "GET", "http://example.com/", http.NoBody)
	//nolint:gosec // G124: Secure is applied by WriteResponseCookie; this test asserts local dev behavior.
	WriteResponseCookie(rr, req, localHTTPDevPort, &http.Cookie{Name: "session", Value: "token"})

	cookies := rr.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %d, want 1", len(cookies))
	}
	if cookies[0].Secure {
		t.Fatal("plain HTTP on local auth dev port must omit Secure so browsers accept the cookie")
	}
	if !cookies[0].HttpOnly {
		t.Fatal("session cookie must be HttpOnly")
	}
}

func TestWriteResponseCookieSecureInProduction(t *testing.T) {
	t.Setenv("BI_ENV", "production")
	t.Setenv("KUBERNETES_SERVICE_HOST", "")

	rr := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), "GET", "http://example.com/", http.NoBody)
	//nolint:gosec // G124: Secure is applied by WriteResponseCookie; this test asserts production behavior.
	WriteResponseCookie(rr, req, localHTTPDevPort, &http.Cookie{Name: "session", Value: "token"})

	cookies := rr.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("cookies = %d, want 1", len(cookies))
	}
	if !cookies[0].Secure {
		t.Fatal("production must set Secure on session cookie even on port 8889 over HTTP")
	}
	if !cookies[0].HttpOnly {
		t.Fatal("session cookie must be HttpOnly")
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
