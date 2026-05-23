package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMetricsHandlerIncludesCatalogMetrics(t *testing.T) {
	old := globalMetrics
	t.Cleanup(func() { globalMetrics = old })
	globalMetrics = &Metrics{}
	globalMetrics.RecordCatalogDBQuery(1500, true)
	globalMetrics.RecordCatalogDBQuery(500, false)
	globalMetrics.RecordModelPublish(250, true)
	globalMetrics.RecordQueryCompile(125, true)
	globalMetrics.RecordQueryCompile(25, false)
	globalMetrics.RecordQueryExecution(200, true, 7)
	globalMetrics.RecordQueryExecution(300, false, 0)
	globalMetrics.RecordLLMRequest(750, 1234, 50)

	rec := httptest.NewRecorder()
	req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", nil)
	MetricsHandler(rec, req)

	body := rec.Body.String()
	for _, want := range []string{
		"go_goroutines ",
		"go_memstats_alloc_bytes ",
		"process_uptime_seconds ",
		"catalog_db_queries_total 2\n",
		"catalog_db_query_errors_total 1\n",
		"catalog_db_query_duration_seconds 2.000\n",
		"model_publish_total 1\n",
		"model_publish_duration_seconds 0.250\n",
		"query_compile_total 2\n",
		"query_compile_errors_total 1\n",
		"query_compile_duration_seconds 0.150\n",
		"query_execute_total 2\n",
		"query_execute_errors_total 1\n",
		"query_execute_duration_seconds 0.500\n",
		"query_rows_returned 7\n",
		"llm_request_duration_seconds 0.750\n",
		"llm_tokens_used_total 1234\n",
		"prompt_build_duration_seconds 0.050\n",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("metrics body missing %q:\n%s", want, body)
		}
	}
}
