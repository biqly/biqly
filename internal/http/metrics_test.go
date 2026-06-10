package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestMetricsHandlerExposesPrometheus verifies the /metrics endpoint serves
// Prometheus text exposition and that recorded BI engine collectors appear in
// the output. Exact counter values are asserted in the observability package
// against an isolated registry; here counters share the process-wide default
// registry, so we assert presence rather than precise values.
func TestMetricsHandlerExposesPrometheus(t *testing.T) {
	m := GetMetrics()
	m.RecordCatalogDBQuery(1500, true)
	m.RecordModelPublish(250, true)
	m.RecordQueryCompile(125, true)
	m.RecordQueryExecution(200, true, 7)
	m.RecordLLMRequest(750, 800, 434, 50)
	m.RecordAIRequest(120, true, 1, false)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", nil)
	MetricsHandler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{
		"go_goroutines",
		"catalog_db_queries_total",
		"model_publish_total",
		"query_compile_total",
		"query_execute_total",
		"query_rows_returned_total",
		"llm_request_duration_seconds",
		"llm_tokens_used_total",
		"prompt_build_duration_seconds",
		"bi_ai_requests_total",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics body missing %q:\n%s", want, body)
		}
	}
}
