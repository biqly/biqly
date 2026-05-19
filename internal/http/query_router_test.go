package http

import (
	"context"
	stdhttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/config"
)

func TestQueryRouter_OnlyMountsQueryPublicRoutes(t *testing.T) {
	t.Parallel()
	handler := QueryRouter(&app.Dependencies{
		Config: &config.Config{
			Query: config.QueryConfig{MaxRuntimeSeconds: 60},
		},
	})

	queryRec := httptest.NewRecorder()
	queryReq := httptest.NewRequestWithContext(context.Background(), stdhttp.MethodPost, "/api/query/compile", nil)
	handler.ServeHTTP(queryRec, queryReq)
	if queryRec.Code != stdhttp.StatusBadRequest {
		t.Fatalf("query status: got %d, want 400", queryRec.Code)
	}

	catalogRec := httptest.NewRecorder()
	catalogReq := httptest.NewRequestWithContext(context.Background(), stdhttp.MethodGet, "/api/datasources", nil)
	handler.ServeHTTP(catalogRec, catalogReq)
	if catalogRec.Code != stdhttp.StatusNotFound {
		t.Fatalf("catalog status: got %d, want 404", catalogRec.Code)
	}
}

func TestQueryRouter_InternalQueryRoutesRequireToken(t *testing.T) {
	t.Parallel()
	handler := QueryRouter(&app.Dependencies{
		Config: &config.Config{
			Query:    config.QueryConfig{MaxRuntimeSeconds: 60},
			Security: config.SecurityConfig{InternalAPIToken: "secret-token"},
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), stdhttp.MethodPost, "/internal/query/compile", nil)
	handler.ServeHTTP(rec, req)
	if rec.Code != stdhttp.StatusUnauthorized {
		t.Fatalf("status without token: got %d, want 401", rec.Code)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequestWithContext(context.Background(), stdhttp.MethodPost, "/internal/query/compile", nil)
	req.Header.Set("X-Internal-Token", "secret-token")
	handler.ServeHTTP(rec, req)
	if rec.Code != stdhttp.StatusBadRequest {
		t.Fatalf("status with token: got %d, want 400", rec.Code)
	}
}
