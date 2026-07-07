package observability

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	ambiguityResolutionOutcomes        = []string{"resolved", "abandoned"}
	ambiguityClarificationRoundBuckets = []float64{1, 2, 3, 4, 5}
	ambiguityLLMTierYieldOutcomes      = []string{"found", "empty", "timeout", "error"}
)

// Agent runtime label sets mirror internal/agent's bounded string consts
// (ToolName, Reason*, RuntimeState failure reason codes, ShadowCategory) by
// value. This package cannot import internal/agent — internal/agent already
// imports observability (see internal/agent/service.go) — so the bound is
// enforced here via BoundLabel instead of a shared Go type.
var (
	agentRunOutcomes = []string{"completed", "failed"}
	// agentTerminalFailureReasons mirrors every reasonCode internal/agent/runtime.go
	// passes to finalizeFail/abandonOrFail.
	agentTerminalFailureReasons = []string{
		"context_canceled", "timeout", "planner_error", "invalid_decision_kind",
		"max_clarification_rounds_exceeded", "max_steps_exceeded", "tool_error",
	}
	// agentToolNames mirrors internal/agent/policy.go's ToolName consts.
	agentToolNames = []string{
		"catalog.resolve", "semantic.resolve", "query.compile", "query.execute", "memory.recall",
	}
	// agentPolicyDenialReasons mirrors internal/agent/policy.go's Reason* consts.
	agentPolicyDenialReasons = []string{
		"tool_not_allowlisted", "retry_budget_exhausted", "airgapped_egress_denied",
		"malformed_arguments", "identity_mismatch", "prompt_injection_suspected",
		"multi_statement_sql_denied", "write_or_ddl_denied", "hidden_column_denied",
		"pii_masking_required", "invalid_join_denied", "row_filter_required", "context_canceled",
	}
	// agentShadowCategories mirrors internal/agent/shadow.go's ShadowCategory consts.
	agentShadowCategories = []string{
		"match", "result_mismatch", "query_mismatch", "latency_regression",
		"clarification_mismatch", "policy_outcome_mismatch", "agent_only_failure",
		"legacy_only_failure", "both_failed",
	}
	agentClarificationRoundBands = []float64{1, 2, 3, 4, 5}
)

type tier2MetricsFactory interface {
	NewCounter(opts prometheus.CounterOpts) prometheus.Counter
	NewCounterVec(opts prometheus.CounterOpts, labelNames []string) *prometheus.CounterVec
	NewHistogram(opts prometheus.HistogramOpts) prometheus.Histogram
	NewHistogramVec(opts prometheus.HistogramOpts, labelNames []string) *prometheus.HistogramVec
	NewGauge(opts prometheus.GaugeOpts) prometheus.Gauge
}

func registerTier2Metrics(f tier2MetricsFactory, m *Metrics) {
	// NATS queue metrics
	m.natsPublishTotal = f.NewCounter(prometheus.CounterOpts{
		Name: "biqly_nats_publish_total", Help: "Total messages successfully published to NATS.",
	})
	m.natsPublishErrors = f.NewCounter(prometheus.CounterOpts{
		Name: "biqly_nats_publish_errors_total", Help: "Total failed NATS publish attempts.",
	})
	m.natsPublishDuration = f.NewHistogram(prometheus.HistogramOpts{
		Name: "biqly_nats_publish_duration_seconds", Help: "NATS publish latency in seconds.", Buckets: prometheus.DefBuckets,
	})
	m.natsConsumeTotal = f.NewCounter(prometheus.CounterOpts{
		Name: "biqly_nats_consume_total", Help: "Total messages successfully consumed from NATS.",
	})
	m.natsConsumeErrors = f.NewCounter(prometheus.CounterOpts{
		Name: "biqly_nats_consume_errors_total", Help: "Total failed NATS consume/processing attempts.",
	})
	m.natsDLQMoves = f.NewCounter(prometheus.CounterOpts{
		Name: "biqly_nats_dlq_moves_total", Help: "Total messages moved to the dead letter queue (DLQ).",
	})
	m.natsConsumerPending = f.NewGauge(prometheus.GaugeOpts{
		Name: "biqly_nats_consumer_pending", Help: "NATS JetStream consumer pending message count.",
	})

	// Memory recall metrics
	m.memoryRecallMisses = f.NewCounter(prometheus.CounterOpts{
		Name: "biqly_memory_recall_misses_total", Help: "Total memory recall attempts that yielded zero results.",
	})
	m.memoryRecallLatency = f.NewHistogram(prometheus.HistogramOpts{
		Name: "biqly_memory_recall_latency_ms", Help: "Memory recall (embedding + similarity sort) latency in milliseconds.", Buckets: ambiguityLatencyMSBuckets,
	})
	m.memoryStoreConfirmedEmbedErrors = f.NewCounter(prometheus.CounterOpts{
		Name: "biqly_memory_store_confirmed_embedding_errors_total", Help: "Total embedding generation errors when storing confirmed queries.",
	})
	m.embeddingCacheHits = f.NewCounter(prometheus.CounterOpts{
		Name: "biqly_embedding_cache_hits_total", Help: "Total question embedding requests served from the in-process cache.",
	})
	m.embeddingCacheMisses = f.NewCounter(prometheus.CounterOpts{
		Name: "biqly_embedding_cache_misses_total", Help: "Total question embedding requests that required an embedding API call.",
	})

	// Clarification metrics
	m.ambiguityClarificationRounds = f.NewHistogram(prometheus.HistogramOpts{
		Name:    "biqly_ambiguity_clarification_rounds_histogram",
		Help:    "Distribution of clarification round counts shown to users.",
		Buckets: ambiguityClarificationRoundBuckets,
	})
	m.ambiguityResolutionTotal = f.NewCounterVec(prometheus.CounterOpts{
		Name: "biqly_ambiguity_resolution_total",
		Help: "Total ambiguity clarifications by outcome (resolved or abandoned).",
	}, []string{"outcome"})
	m.ambiguityLLMTierYield = f.NewCounterVec(prometheus.CounterOpts{
		Name: "biqly_ambiguity_llm_tier_yield_total",
		Help: "LLM ambiguity tier outcomes (found, empty, timeout, error) — measures what the LLM tier actually contributes.",
	}, []string{"outcome"})

	// LLM response cache metrics
	m.llmResponseCacheHits = f.NewCounter(prometheus.CounterOpts{
		Name: "biqly_llm_response_cache_hits_total", Help: "Total LLM query response cache hits.",
	})
	m.llmResponseCacheMisses = f.NewCounter(prometheus.CounterOpts{
		Name: "biqly_llm_response_cache_misses_total", Help: "Total LLM query response cache misses.",
	})

	// Enrich context suggestions metrics
	m.enrichContextSuggestionsGenerated = f.NewCounter(prometheus.CounterOpts{
		Name: "biqly_enrich_context_suggestions_generated_total", Help: "Total business descriptions / enum label suggestions generated.",
	})
	m.enrichContextSuggestLatency = f.NewHistogram(prometheus.HistogramOpts{
		Name: "biqly_enrich_context_suggest_latency_seconds", Help: "Enrich context suggestion generation latency (including LLM call) in seconds.", Buckets: prometheus.DefBuckets,
	})
	m.enrichContextApplyErrors = f.NewCounter(prometheus.CounterOpts{
		Name: "biqly_enrich_context_apply_errors_total", Help: "Total failed attempts to apply context enrichment suggestions.",
	})

	// Agent runtime metrics (internal/agent's planner/policy/tool loop). Never
	// use the run's question, SQL, credentials, or result rows as a label —
	// every label here is bounded via BoundLabel against a fixed value set.
	m.agentRunsTotal = f.NewCounterVec(prometheus.CounterOpts{
		Name: "biqly_agent_runs_total", Help: "Total agent runs by terminal outcome.",
	}, []string{"outcome"})
	m.agentTerminalFailures = f.NewCounterVec(prometheus.CounterOpts{
		Name: "biqly_agent_terminal_failures_total", Help: "Total agent run terminal failures by reason code.",
	}, []string{"reason"})
	m.agentStepDuration = f.NewHistogramVec(prometheus.HistogramOpts{
		Name: "biqly_agent_step_duration_seconds", Help: "Agent tool dispatch latency in seconds, by tool.", Buckets: prometheus.DefBuckets,
	}, []string{"tool"})
	m.agentPolicyDenials = f.NewCounterVec(prometheus.CounterOpts{
		Name: "biqly_agent_policy_denials_total", Help: "Total agent tool-call proposals denied by policy, by reason code.",
	}, []string{"reason"})
	m.agentClarificationRounds = f.NewHistogram(prometheus.HistogramOpts{
		Name:    "biqly_agent_clarification_rounds_histogram",
		Help:    "Distribution of clarification round counts reached by agent runs.",
		Buckets: agentClarificationRoundBands,
	})
	m.agentShadowComparisons = f.NewCounterVec(prometheus.CounterOpts{
		Name: "biqly_agent_shadow_comparisons_total", Help: "Total shadow-mode comparisons between legacy and agent runs, by category.",
	}, []string{"category"})
	m.agentQueueRedeliveries = f.NewCounter(prometheus.CounterOpts{
		Name: "biqly_agent_queue_redeliveries_total",
		Help: "Total agent jobs whose run already existed on job-id lookup (NATS redelivery or crash recovery resume).",
	})
	m.agentPlannerTokens = f.NewCounterVec(prometheus.CounterOpts{
		Name: "biqly_agent_planner_tokens_total", Help: "Total planner LLM tokens used, by kind (prompt, completion).",
	}, []string{"kind"})
}

// RecordAgentRunTerminal records an agent run reaching a terminal state.
func (m *Metrics) RecordAgentRunTerminal(outcome string) {
	if m == nil {
		return
	}
	m.agentRunsTotal.WithLabelValues(BoundLabel(outcome, agentRunOutcomes, "failed")).Inc()
}

// RecordAgentTerminalFailure records the specific reason code an agent run failed with.
func (m *Metrics) RecordAgentTerminalFailure(reason string) {
	if m == nil {
		return
	}
	m.agentTerminalFailures.WithLabelValues(BoundLabel(reason, agentTerminalFailureReasons, "tool_error")).Inc()
}

// RecordAgentStepDuration records one tool dispatch's latency.
func (m *Metrics) RecordAgentStepDuration(tool string, duration time.Duration) {
	if m == nil {
		return
	}
	m.agentStepDuration.WithLabelValues(BoundLabel(tool, agentToolNames, "other")).Observe(duration.Seconds())
}

// RecordAgentPolicyDenial records a policy denial by reason code.
func (m *Metrics) RecordAgentPolicyDenial(reason string) {
	if m == nil {
		return
	}
	m.agentPolicyDenials.WithLabelValues(BoundLabel(reason, agentPolicyDenialReasons, "other")).Inc()
}

// RecordAgentClarificationRound records the clarification round number an agent run reached.
func (m *Metrics) RecordAgentClarificationRound(round int) {
	if m == nil {
		return
	}
	m.agentClarificationRounds.Observe(float64(round))
}

// RecordAgentShadowComparison records one shadow-mode comparison category.
func (m *Metrics) RecordAgentShadowComparison(category string) {
	if m == nil {
		return
	}
	m.agentShadowComparisons.WithLabelValues(BoundLabel(category, agentShadowCategories, "other")).Inc()
}

// RecordAgentQueueRedelivery records an agent job resuming an already-created
// run instead of creating a fresh one — the signal that a NATS message was
// redelivered (or a crash-recovery retry) rather than processed for the
// first time.
func (m *Metrics) RecordAgentQueueRedelivery() {
	if m == nil {
		return
	}
	m.agentQueueRedeliveries.Inc()
}

// RecordAgentPlannerTokens records planner LLM token usage for one Decide call.
func (m *Metrics) RecordAgentPlannerTokens(promptTokens, completionTokens int) {
	if m == nil {
		return
	}
	if promptTokens > 0 {
		m.agentPlannerTokens.WithLabelValues("prompt").Add(float64(promptTokens))
	}
	if completionTokens > 0 {
		m.agentPlannerTokens.WithLabelValues("completion").Add(float64(completionTokens))
	}
}

// RecordNATSPublish records NATS publish metrics.
func (m *Metrics) RecordNATSPublish(duration time.Duration, success bool) {
	if m == nil {
		return
	}
	if success {
		m.natsPublishTotal.Inc()
	} else {
		m.natsPublishErrors.Inc()
	}
	m.natsPublishDuration.Observe(duration.Seconds())
}

// RecordNATSConsume records NATS consume metrics.
func (m *Metrics) RecordNATSConsume(success bool) {
	if m == nil {
		return
	}
	if success {
		m.natsConsumeTotal.Inc()
	} else {
		m.natsConsumeErrors.Inc()
	}
}

// RecordNATSDLQMove records a NATS DLQ move.
func (m *Metrics) RecordNATSDLQMove() {
	if m == nil {
		return
	}
	m.natsDLQMoves.Inc()
}

// RecordNATSConsumerPending updates the consumer pending gauge.
func (m *Metrics) RecordNATSConsumerPending(pending uint64) {
	if m == nil {
		return
	}
	m.natsConsumerPending.Set(float64(pending))
}

// RecordMemoryRecallMiss records a memory recall miss.
func (m *Metrics) RecordMemoryRecallMiss() {
	if m == nil {
		return
	}
	m.memoryRecallMisses.Inc()
}

// RecordMemoryRecallLatency records memory recall latency in milliseconds.
func (m *Metrics) RecordMemoryRecallLatency(duration time.Duration) {
	if m == nil {
		return
	}
	m.memoryRecallLatency.Observe(float64(duration.Milliseconds()))
}

// RecordMemoryStoreConfirmedEmbeddingError records a memory store confirmed embedding error.
func (m *Metrics) RecordMemoryStoreConfirmedEmbeddingError() {
	if m == nil {
		return
	}
	m.memoryStoreConfirmedEmbedErrors.Inc()
}

// RecordEmbeddingCacheHit records a question embedding served from cache.
func (m *Metrics) RecordEmbeddingCacheHit() {
	if m == nil {
		return
	}
	m.embeddingCacheHits.Inc()
}

// RecordEmbeddingCacheMiss records a question embedding that needed an API call.
func (m *Metrics) RecordEmbeddingCacheMiss() {
	if m == nil {
		return
	}
	m.embeddingCacheMisses.Inc()
}

// RecordAmbiguityLLMTierYield records what the LLM ambiguity tier produced.
func (m *Metrics) RecordAmbiguityLLMTierYield(outcome string) {
	if m == nil {
		return
	}
	cleanOutcome := BoundLabel(outcome, ambiguityLLMTierYieldOutcomes, "error")
	m.ambiguityLLMTierYield.WithLabelValues(cleanOutcome).Inc()
}

// RecordAmbiguityClarificationRound records the round number returned to the user.
func (m *Metrics) RecordAmbiguityClarificationRound(round int) {
	if m == nil {
		return
	}
	m.ambiguityClarificationRounds.Observe(float64(round))
}

// RecordAmbiguityResolution records whether clarification was resolved or abandoned.
func (m *Metrics) RecordAmbiguityResolution(outcome string) {
	if m == nil {
		return
	}
	cleanOutcome := BoundLabel(outcome, ambiguityResolutionOutcomes, "abandoned")
	m.ambiguityResolutionTotal.WithLabelValues(cleanOutcome).Inc()
}

// RecordLLMResponseCacheHit records an LLM response cache hit.
func (m *Metrics) RecordLLMResponseCacheHit() {
	if m == nil {
		return
	}
	m.llmResponseCacheHits.Inc()
}

// RecordLLMResponseCacheMiss records an LLM response cache miss.
func (m *Metrics) RecordLLMResponseCacheMiss() {
	if m == nil {
		return
	}
	m.llmResponseCacheMisses.Inc()
}

// RecordEnrichContextSuggestionsGenerated records suggestions count.
func (m *Metrics) RecordEnrichContextSuggestionsGenerated(count int) {
	if m == nil || count <= 0 {
		return
	}
	m.enrichContextSuggestionsGenerated.Add(float64(count))
}

// RecordEnrichContextSuggestLatency records suggestion latency.
func (m *Metrics) RecordEnrichContextSuggestLatency(duration time.Duration) {
	if m == nil {
		return
	}
	m.enrichContextSuggestLatency.Observe(duration.Seconds())
}

// RecordEnrichContextApplyErrors records apply errors count.
func (m *Metrics) RecordEnrichContextApplyErrors(count int) {
	if m == nil || count <= 0 {
		return
	}
	m.enrichContextApplyErrors.Add(float64(count))
}
