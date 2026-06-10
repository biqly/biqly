package observability

import (
	"errors"
	"net/http"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestHTTPRouteGroupBounded(t *testing.T) {
	cases := map[string]string{
		"/api/ai/query":            "/api/ai/query",
		"/api/ai/query/preview":    "/api/ai/preview",
		"/api/datasources/{id}":    "/api/catalog",
		"/api/query/run":           "/api/query",
		"/api/admin/users":         "/api/admin",
		"/internal/catalog/tables": "/internal",
		"/health":                  "/health",
		"/totally/unknown/{id}":    "other",
	}
	for pattern, want := range cases {
		if got := HTTPRouteGroup(pattern); got != want {
			t.Fatalf("HTTPRouteGroup(%q) = %q, want %q", pattern, got, want)
		}
	}
}

func TestTier1MetricsRecord(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	m.RecordHTTPRequest(http.MethodPost, "/api/ai/query", http.StatusOK, 120)
	m.RecordLLMProviderError("openai", "rate_limit")
	m.RecordLLMProviderRetry("anthropic")
	m.RecordLLMProviderTokens(100, 50)
	m.RecordRoutingResult("hybrid", 0.82, false, false)
	m.RecordRoutingResult("keyword", 0.2, true, false)
	m.RecordEmbeddingAPI("route_recall", 45, errors.New("dial tcp timeout"), http.StatusServiceUnavailable)

	if got := testutil.ToFloat64(m.httpRequestsTotal.WithLabelValues("POST", "2xx")); got != 1 {
		t.Fatalf("http requests = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.llmErrorsTotal.WithLabelValues("openai", "rate_limit")); got != 1 {
		t.Fatalf("llm errors = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.llmRetriesTotal.WithLabelValues("anthropic")); got != 1 {
		t.Fatalf("llm retries = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.routingDecisionsTotal.WithLabelValues("hybrid", "success")); got != 1 {
		t.Fatalf("routing success = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.routingDecisionsTotal.WithLabelValues("keyword", "clarification")); got != 1 {
		t.Fatalf("routing clarification = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.embeddingAPIErrorsTotal.WithLabelValues("route_recall", "network")); got != 1 {
		t.Fatalf("embedding errors = %v, want 1", got)
	}
	if err := CheckGatheredCardinality(reg); err != nil {
		t.Fatalf("CheckGatheredCardinality: %v", err)
	}
}

func TestClassifyProviderError(t *testing.T) {
	if got := ClassifyProviderError(nil, http.StatusTooManyRequests); got != "rate_limit" {
		t.Fatalf("got %q, want rate_limit", got)
	}
	if got := ClassifyProviderError(errors.New("unmarshal response"), 0); got != "parse" {
		t.Fatalf("got %q, want parse", got)
	}
}
