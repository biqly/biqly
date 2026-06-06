package http

import (
	"context"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/query"
)

func TestAIRouter_UsesAIRequestTimeout(t *testing.T) {
	t.Parallel()
	deps := &app.Dependencies{
		Config: &config.Config{
			Query: config.QueryConfig{MaxRuntimeSeconds: 60},
			AI: config.AIConfig{
				HTTPTimeoutSeconds: 12,
				TranslationConfig:  config.TranslationConfig{TranslationHTTPTimeoutSeconds: 90},
			},
		},
	}

	if got := aiServiceRequestTimeout(deps); got != 120*time.Second {
		t.Fatalf("timeout = %s, want AI request timeout", got)
	}
}

func TestAIRouter_OnlyMountsAIRoutes(t *testing.T) {
	t.Parallel()
	handler := AIRouter(&app.Dependencies{
		Config: &config.Config{
			Query: config.QueryConfig{MaxRuntimeSeconds: 60, MaxRows: 1000},
			AI:    config.AIConfig{Model: "test-model"},
		},
		Validator: query.NewValidator(1000),
	})

	aiRec := httptest.NewRecorder()
	aiReq := httptest.NewRequestWithContext(context.Background(), stdhttp.MethodPost, "/api/ai/query", nil)
	handler.ServeHTTP(aiRec, aiReq)
	if aiRec.Code != stdhttp.StatusBadRequest {
		t.Fatalf("ai status: got %d, want 400", aiRec.Code)
	}

	queryRec := httptest.NewRecorder()
	queryReq := httptest.NewRequestWithContext(context.Background(), stdhttp.MethodPost, "/api/query/compile", nil)
	handler.ServeHTTP(queryRec, queryReq)
	if queryRec.Code != stdhttp.StatusNotFound {
		t.Fatalf("query status: got %d, want 404", queryRec.Code)
	}
}

func TestAIRouter_InternalHealthRequiresToken(t *testing.T) {
	t.Parallel()
	handler := AIRouter(&app.Dependencies{
		Config: &config.Config{
			Query:    config.QueryConfig{MaxRuntimeSeconds: 60, MaxRows: 1000},
			Security: config.SecurityConfig{InternalAPIToken: "secret-token"},
			AI:       config.AIConfig{Model: "test-model"},
		},
		Validator: query.NewValidator(1000),
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), stdhttp.MethodGet, "/internal/health", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != stdhttp.StatusUnauthorized {
		t.Fatalf("status without token: got %d, want 401", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(context.Background(), stdhttp.MethodGet, "/internal/health", nil)
	req.Header.Set("X-Internal-Token", "secret-token")
	handler.ServeHTTP(rec, req)
	if rec.Code != stdhttp.StatusOK {
		t.Fatalf("status with token: got %d, want 200", rec.Code)
	}
}
