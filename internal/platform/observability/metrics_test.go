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
	m.RecordLLMRequest(750, 800, 434, 50)
	m.RecordAIStep("llm_generate", 420)
	m.RecordAIStep("prompt_build", 50)
	m.RecordAIRequest(120, true, 2, false)
	m.RecordAIRequest(90, false, 0, true)
	m.RecordAmbiguityAnalysis(12, "rule_based", true)
	m.RecordAmbiguityAnalysis(34, "llm", true)
	m.RecordAmbiguityTier("1")
	m.RecordAmbiguityTier("2")
	m.RecordAmbiguityClarified()
	m.RecordAmbiguityRoundCapReached()
	m.RecordMemoryStoreConfirmed()
	m.RecordMemoryStoreRecall(2)
	m.RecordMemoryRecallFeedback(true, "positive")
	m.RecordMemoryRecallFeedback(false, "negative")
	m.RecordEnrichContextGaps(5)
	m.RecordEnrichContextApplied(3)

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
	if got := testutil.ToFloat64(m.llmTokensPromptTotal); got != 800 {
		t.Fatalf("biqly_llm_tokens_prompt_total = %v, want 800", got)
	}
	if got := testutil.ToFloat64(m.llmTokensCompletionTotal); got != 434 {
		t.Fatalf("biqly_llm_tokens_completion_total = %v, want 434", got)
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
	if got := testutil.ToFloat64(m.ambiguityRoundCapReached); got != 1 {
		t.Fatalf("biqly_ambiguity_round_cap_reached_total = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.memoryStoreConfirmed); got != 1 {
		t.Fatalf("biqly_memory_store_confirmed_total = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.memoryStoreRecall); got != 2 {
		t.Fatalf("biqly_memory_store_recall_hits_total = %v, want 2", got)
	}
	if got := testutil.ToFloat64(m.memoryRecallFeedback.WithLabelValues("true", "positive")); got != 1 {
		t.Fatalf("biqly_memory_recall_feedback_total{recall=true,rating=positive} = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.enrichContextGapsFound); got != 5 {
		t.Fatalf("biqly_enrich_context_gaps_found_total = %v, want 5", got)
	}
	if got := testutil.ToFloat64(m.enrichContextApplied); got != 3 {
		t.Fatalf("biqly_enrich_context_applied_total = %v, want 3", got)
	}
}

func TestDefaultSingleton(t *testing.T) {
	a, b := Default(), Default()
	if a != b {
		t.Fatal("Default() must return the same singleton")
	}
}

func TestMetricsLabelCardinality(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	// Record known and unknown ambiguity sources
	m.RecordAmbiguityAnalysis(10, "rule_based", true)
	m.RecordAmbiguityAnalysis(10, "llm", true)
	m.RecordAmbiguityAnalysis(10, "unbounded_random_source_1", true)
	m.RecordAmbiguityAnalysis(10, "unbounded_random_source_2", true)

	if got := testutil.ToFloat64(m.ambiguityBySource.WithLabelValues("rule_based")); got != 1 {
		t.Fatalf("rule_based source count = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.ambiguityBySource.WithLabelValues("llm")); got != 1 {
		t.Fatalf("llm source count = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.ambiguityBySource.WithLabelValues("other")); got != 2 {
		t.Fatalf("other source count = %v, want 2", got)
	}

	// Record known and unknown AI repair error codes
	m.RecordAIRepair(true, 1, []string{"UNKNOWN_DIMENSION", "UNKNOWN_METRIC", "UNKNOWN_FIELD", "SOME_RANDOM_CODE_1", "SOME_RANDOM_CODE_2"})

	if got := testutil.ToFloat64(m.aiRepairByErrorCode.WithLabelValues("UNKNOWN_DIMENSION")); got != 1 {
		t.Fatalf("UNKNOWN_DIMENSION count = %v, want 1", got)
	}
	if got := testutil.ToFloat64(m.aiRepairByErrorCode.WithLabelValues("other")); got != 2 {
		t.Fatalf("other error code count = %v, want 2", got)
	}

	if err := CheckGatheredCardinality(reg); err != nil {
		t.Fatalf("CheckGatheredCardinality: %v", err)
	}
}
