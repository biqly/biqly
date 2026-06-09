package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOtelRouteFilter(t *testing.T) {
	t.Parallel()

	cases := []struct {
		path string
		want bool
	}{
		{"/health", false},
		{"/healthz", false},
		{"/ready", false},
		{"/readyz", false},
		{"/metrics", false},
		{"/internal/health", false},
		{"/api/query/run", true},
		{"/api/auth/login", true},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()
			req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, tc.path, http.NoBody)
			if got := otelRouteFilter(req); got != tc.want {
				t.Fatalf("otelRouteFilter(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

func TestOTELHTTPHandler_WrapsHandler(t *testing.T) {
	t.Parallel()

	var called bool
	inner := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	})

	handler := OTELHTTPHandler("biqly-test", inner)
	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/ping", http.NoBody)
	handler.ServeHTTP(rec, req)

	if !called {
		t.Fatal("inner handler was not called")
	}
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status: got %d, want 204", rec.Code)
	}
}
