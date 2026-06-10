package observability

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	ambiguityResolutionOutcomes        = []string{"resolved", "abandoned"}
	ambiguityClarificationRoundBuckets = []float64{1, 2, 3, 4, 5}
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
