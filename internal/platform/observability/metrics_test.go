package observability

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestMetricsRecord(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	m.RecordCatalogDBQuery(1500, true)
	m.RecordCatalogDBQuery(500, false)
	m.RecordModelPublish(250, true)
	m.RecordQueryCompile(125, true)
	m.RecordQueryCompile(25, false)
	m.RecordQueryExecution(200, true, 7)
	m.RecordQueryExecution(300, false, 0)
	m.RecordLLMRequest(750, 1234, 50)
	m.RecordAIRequest(120, true, 2, false)
	m.RecordAIRequest(90, false, 0, true)
	m.RecordAmbiguityAnalysis(12, "rule_based", true)
	m.RecordAmbiguityAnalysis(34, "llm", true)
	m.RecordAmbiguityClarified()

	if got := testutil.ToFloat64(m.catalogDBQueries); got != 2 {
		t.Fatalf("catalog_db_queries_total = %v, want 2", got)
	}
	if got := testutil.ToFloat64(m.catalogDBErrors); got != 1 {
		t.Fatalf("catalog_db_query_errors_total = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.queryCompileErrors); got != 1 {
		t.Fatalf("query_compile_errors_total = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.queryRowsReturned); got != 7 {
		t.Fatalf("query_rows_returned_total = %v, want 7", got)
	}
	if got := testutil.ToFloat64(m.llmTokensUsed); got != 1234 {
		t.Fatalf("llm_tokens_used_total = %v, want 1234", got)
	}
	if got := testutil.ToFloat64(m.aiRequestsTotal); got != 2 {
		t.Fatalf("bi_ai_requests_total = %v, want 2", got)
	}
	if got := testutil.ToFloat64(m.aiRetriesTotal); got != 2 {
		t.Fatalf("bi_ai_retries_total = %v, want 2", got)
	}
	if got := testutil.ToFloat64(m.aiClarifications); got != 1 {
		t.Fatalf("bi_ai_clarifications_total = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.queryCompiles); got != 2 {
		t.Fatalf("query_compile_total = %v, want 2", got)
	}
	if got := testutil.ToFloat64(m.ambiguityDetected); got != 2 {
		t.Fatalf("biqly_ambiguity_detected_total = %v, want 2", got)
	}
	if got := testutil.ToFloat64(m.ambiguityBySource.WithLabelValues("llm")); got != 1 {
		t.Fatalf("biqly_ambiguity_by_source{source=llm} = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.ambiguityClarified); got != 1 {
		t.Fatalf("biqly_ambiguity_clarified_total = %v, want 1", got)
	}
}

func TestDefaultSingleton(t *testing.T) {
	a, b := Default(), Default()
	if a != b {
		t.Fatal("Default() must return the same singleton")
	}
}
