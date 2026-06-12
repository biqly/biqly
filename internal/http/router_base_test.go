package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/biqly/biqly/pkg/common/requestid"
	"github.com/go-chi/chi/v5"
)

func TestApplyBaseMiddlewarePropagatesRequestIDAndSecurityHeaders(t *testing.T) {
	var gotRequestID string
	r := chi.NewRouter()
	ApplyBaseMiddleware(r, BaseMiddlewareConfig{
		Timeout: 5 * time.Second,
		SecurityHeaders: bimw.SecurityHeadersConfig{
			ContentSecurityPolicy: "default-src 'self'",
		},
	})
	r.Get("/probe", func(w http.ResponseWriter, r *http.Request) {
		gotRequestID = requestid.FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/probe", http.NoBody)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("ApplyBaseMiddleware status = %d, want %d", rec.Code, http.StatusOK)
	}
	if gotRequestID == "" {
		t.Error("ApplyBaseMiddleware requestid.FromContext() = empty, want propagated request id")
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("ApplyBaseMiddleware X-Content-Type-Options = %q, want %q", got, "nosniff")
	}
	if got := rec.Header().Get("Content-Security-Policy"); got != "default-src 'self'" {
		t.Errorf("ApplyBaseMiddleware Content-Security-Policy = %q, want %q", got, "default-src 'self'")
	}
}

func TestServiceRoutersUseBaseMiddleware(t *testing.T) {
	_, testFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(testFile), "..", ".."))
	targets := []string{
		"internal/http/router.go",
		"internal/http/ai_router.go",
		"internal/http/query_router.go",
		"internal/http/catalog_router.go",
		"cmd/auth/main.go",
		"cmd/mail/main.go",
	}
	disallowed := []string{
		".Use(middleware.RequestID)",
		".Use(requestIDPropagationMiddleware)",
		".Use(propagateRequestID)",
		".Use(bimw.RealIP)",
		".Use(middleware.Logger)",
		".Use(middleware.Recoverer)",
	}

	for _, target := range targets {
		path := filepath.Join(repoRoot, target)
		// #nosec G304 -- target is selected from the fixed router source whitelist above.
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("os.ReadFile(%q) error = %v", target, err)
		}
		text := string(content)
		if !strings.Contains(text, "ApplyBaseMiddleware(") {
			t.Errorf("%s does not call ApplyBaseMiddleware", target)
		}
		for _, needle := range disallowed {
			if strings.Contains(text, needle) {
				t.Errorf("%s contains manual base middleware %q", target, needle)
			}
		}
	}
}
