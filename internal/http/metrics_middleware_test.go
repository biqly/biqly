package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/platform/observability"
	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
)

func TestHTTPMetricsMiddlewareRecordsRequest(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := observability.NewMetrics(reg)

	r := chi.NewRouter()
	r.Use(HTTPMetricsMiddleware(metrics))
	r.Get("/api/query/run", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/api/query/run", nil)
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	fams, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather: %v", err)
	}
	var found bool
	for _, fam := range fams {
		if fam.GetName() == "biqly_http_requests_total" && len(fam.GetMetric()) > 0 {
			found = true
			break
		}
	}
	if !found {
		names := make([]string, 0, len(fams))
		for _, fam := range fams {
			names = append(names, fam.GetName())
		}
		t.Fatalf("missing biqly_http_requests_total in gathered metrics: %s", strings.Join(names, ", "))
	}
}
