package observability

import (
	"testing"
	"time"

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

	assertMetric(t, m.catalogDBQueries, 2, "catalog_db_queries_total")
	assertMetric(t, m.catalogDBErrors, 1, "catalog_db_query_errors_total")
	assertMetric(t, m.queryCompileErrors, 1, "query_compile_errors_total")
	assertMetric(t, m.queryRowsReturned, 7, "query_rows_returned_total")
	assertMetric(t, m.llmTokensUsed, 1234, "llm_tokens_used_total")
	assertMetric(t, m.llmTokensPromptTotal, 800, "biqly_llm_tokens_prompt_total")
	assertMetric(t, m.llmTokensCompletionTotal, 434, "biqly_llm_tokens_completion_total")
	assertMetric(t, m.aiRequestsTotal, 2, "bi_ai_requests_total")
	assertMetric(t, m.aiRetriesTotal, 2, "bi_ai_retries_total")
	assertMetric(t, m.aiClarifications, 1, "bi_ai_clarifications_total")
	assertMetric(t, m.queryCompiles, 2, "query_compile_total")
	assertMetric(t, m.ambiguityDetected, 2, "biqly_ambiguity_detected_total")
	assertMetric(t, m.ambiguityBySource.WithLabelValues("llm"), 1, "biqly_ambiguity_by_source{source=llm}")
	assertMetric(t, m.ambiguityClarified, 1, "biqly_ambiguity_clarified_total")
	assertMetric(t, m.ambiguityRoundCapReached, 1, "biqly_ambiguity_round_cap_reached_total")
	assertMetric(t, m.memoryStoreConfirmed, 1, "biqly_memory_store_confirmed_total")
	assertMetric(t, m.memoryStoreRecall, 2, "biqly_memory_store_recall_hits_total")
	assertMetric(t, m.memoryRecallFeedback.WithLabelValues("true", "positive"), 1, "biqly_memory_recall_feedback_total{recall=true,rating=positive}")
	assertMetric(t, m.enrichContextGapsFound, 5, "biqly_enrich_context_gaps_found_total")
	assertMetric(t, m.enrichContextApplied, 3, "biqly_enrich_context_applied_total")
}

func TestMetricsRecordTier2(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := NewMetrics(reg)

	m.RecordNATSPublish(time.Millisecond*100, true)
	m.RecordNATSPublish(time.Millisecond*200, false)
	m.RecordNATSConsume(true)
	m.RecordNATSConsume(false)
	m.RecordNATSDLQMove()
	m.RecordNATSConsumerPending(15)
	m.RecordMemoryRecallMiss()
	m.RecordMemoryRecallLatency(time.Millisecond * 50)
	m.RecordMemoryStoreConfirmedEmbeddingError()
	m.RecordAmbiguityClarificationRound(3)
	m.RecordAmbiguityResolution("resolved")
	m.RecordAmbiguityResolution("abandoned")
	m.RecordLLMResponseCacheHit()
	m.RecordLLMResponseCacheMiss()
	m.RecordEnrichContextSuggestionsGenerated(12)
	m.RecordEnrichContextSuggestLatency(time.Second * 3)
	m.RecordEnrichContextApplyErrors(2)

	assertMetric(t, m.natsPublishTotal, 1, "biqly_nats_publish_total")
	assertMetric(t, m.natsPublishErrors, 1, "biqly_nats_publish_errors_total")
	assertMetric(t, m.natsConsumeTotal, 1, "biqly_nats_consume_total")
	assertMetric(t, m.natsConsumeErrors, 1, "biqly_nats_consume_errors_total")
	assertMetric(t, m.natsDLQMoves, 1, "biqly_nats_dlq_moves_total")
	assertMetric(t, m.natsConsumerPending, 15, "biqly_nats_consumer_pending")
	assertMetric(t, m.memoryRecallMisses, 1, "biqly_memory_recall_misses_total")
	assertMetric(t, m.memoryStoreConfirmedEmbedErrors, 1, "biqly_memory_store_confirmed_embedding_errors_total")
	assertMetric(t, m.ambiguityResolutionTotal.WithLabelValues("resolved"), 1, "biqly_ambiguity_resolution_total{outcome=resolved}")
	assertMetric(t, m.ambiguityResolutionTotal.WithLabelValues("abandoned"), 1, "biqly_ambiguity_resolution_total{outcome=abandoned}")
	assertMetric(t, m.llmResponseCacheHits, 1, "biqly_llm_response_cache_hits_total")
	assertMetric(t, m.llmResponseCacheMisses, 1, "biqly_llm_response_cache_misses_total")
	assertMetric(t, m.enrichContextSuggestionsGenerated, 12, "biqly_enrich_context_suggestions_generated_total")
	assertMetric(t, m.enrichContextApplyErrors, 2, "biqly_enrich_context_apply_errors_total")
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

	assertMetric(t, m.ambiguityBySource.WithLabelValues("rule_based"), 1, "rule_based source count")
	assertMetric(t, m.ambiguityBySource.WithLabelValues("llm"), 1, "llm source count")
	assertMetric(t, m.ambiguityBySource.WithLabelValues("other"), 2, "other source count")

	// Record known and unknown AI repair error codes
	m.RecordAIRepair(true, 1, []string{"UNKNOWN_DIMENSION", "UNKNOWN_METRIC", "UNKNOWN_FIELD", "SOME_RANDOM_CODE_1", "SOME_RANDOM_CODE_2"})

	assertMetric(t, m.aiRepairByErrorCode.WithLabelValues("UNKNOWN_DIMENSION"), 1, "UNKNOWN_DIMENSION count")
	assertMetric(t, m.aiRepairByErrorCode.WithLabelValues("other"), 2, "other error code count")

	if err := CheckGatheredCardinality(reg); err != nil {
		t.Fatalf("CheckGatheredCardinality: %v", err)
	}
}

func assertMetric(t *testing.T, collector prometheus.Collector, want float64, name string) {
	t.Helper()
	if got := testutil.ToFloat64(collector); got != want {
		t.Fatalf("%s = %v, want %v", name, got, want)
	}
}
