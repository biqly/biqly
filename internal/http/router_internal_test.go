package http

import (
	"context"
	"github.com/bytedance/sonic"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/pkg/internalapi"
)

func TestRouter_InternalRoutesRequireToken(t *testing.T) {
	t.Parallel()
	handler := Router(&app.Dependencies{
		Config: &config.Config{
			Security: config.SecurityConfig{InternalAPIToken: "secret-token"},
		},
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequestWithContext(context.Background(), stdhttp.MethodGet, "/internal/health", nil)
	handler.ServeHTTP(w, r)

	if w.Code != stdhttp.StatusUnauthorized {
		t.Fatalf("status: got %d, want 401", w.Code)
	}
	var env internalapi.Error
	if err := sonic.ConfigStd.NewDecoder(w.Body).Decode(&env); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if env.Code != internalapi.CodeUnauthorized {
		t.Fatalf("code: got %q, want %q", env.Code, internalapi.CodeUnauthorized)
	}
}

func TestRouter_InternalRoutesAcceptInternalTokenHeader(t *testing.T) {
	t.Parallel()
	handler := Router(&app.Dependencies{
		Config: &config.Config{
			Security: config.SecurityConfig{InternalAPIToken: "secret-token"},
		},
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequestWithContext(context.Background(), stdhttp.MethodGet, "/internal/health", nil)
	r.Header.Set("X-Internal-Token", "secret-token")
	handler.ServeHTTP(w, r)

	if w.Code != stdhttp.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
}

func TestRouter_InternalRoutesAcceptBearerToken(t *testing.T) {
	t.Parallel()
	handler := Router(&app.Dependencies{
		Config: &config.Config{
			Security: config.SecurityConfig{InternalAPIToken: "secret-token"},
		},
	})

	w := httptest.NewRecorder()
	r := httptest.NewRequestWithContext(context.Background(), stdhttp.MethodGet, "/internal/health", nil)
	r.Header.Set("Authorization", "Bearer secret-token")
	handler.ServeHTTP(w, r)

	if w.Code != stdhttp.StatusOK {
		t.Fatalf("status: got %d, want 200", w.Code)
	}
}

func TestRouter_InternalRoutesFailClosedWhenTokenUnset(t *testing.T) {
	t.Parallel()
	handler := Router(&app.Dependencies{Config: &config.Config{}})

	w := httptest.NewRecorder()
	r := httptest.NewRequestWithContext(context.Background(), stdhttp.MethodGet, "/internal/health", nil)
	handler.ServeHTTP(w, r)

	if w.Code != stdhttp.StatusForbidden {
		t.Fatalf("status: got %d, want 403", w.Code)
	}
}
