// Package observability centralizes process-wide telemetry for the BI engine:
// Prometheus metrics and structured-logging setup. It is the single place
// metric collectors are defined so every binary (api, worker, auth, query,
// catalog) exposes a consistent /metrics surface backed by the same library —
// matching the promauto-based collectors already used by the auth service.
package observability

import (
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// queryLatencyBuckets mirror the fixed latency bands the previous hand-rolled
// /metrics exposition reported (10ms / 50ms / 100ms / 500ms / 1s), expressed in
// seconds for Prometheus histogram convention.
var queryLatencyBuckets = []float64{0.01, 0.05, 0.1, 0.5, 1}
var ambiguityLatencyMSBuckets = []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2000, 5000}

// Metrics holds every process-local Prometheus collector for the BI engine.
// Construct it with NewMetrics (tests, isolated registries) or obtain the
// process singleton with Default. All Record* methods are safe for concurrent
// use; Prometheus collectors update lock-free on the hot path.
type Metrics struct {
	queriesTotal  prometheus.Counter
	queryErrors   prometheus.Counter
	queryDuration prometheus.Histogram
	cacheHits     prometheus.Counter
	cacheMisses   prometheus.Counter

	aiRequestsTotal    prometheus.Counter
	aiErrors           prometheus.Counter
	aiRequestDuration  prometheus.Histogram
	aiRetriesTotal     prometheus.Counter
	aiClarifications   prometheus.Counter
	llmRequestDuration prometheus.Histogram
	llmTokensUsed      prometheus.Counter
	promptBuildSeconds prometheus.Histogram
	ambiguityDetected  prometheus.Counter
	ambiguityClarified prometheus.Counter
	ambiguityLatencyMS prometheus.Histogram
	ambiguityBySource  *prometheus.CounterVec

	validationFailures prometheus.Counter
	connectionErrors   prometheus.Counter

	queryCompiles        prometheus.Counter
	queryCompileErrors   prometheus.Counter
	queryCompileDuration prometheus.Histogram
	queryExecutions      prometheus.Counter
	queryExecutionErrors prometheus.Counter
	queryExecuteDuration prometheus.Histogram
	queryRowsReturned    prometheus.Counter

	catalogDBQueries  prometheus.Counter
	catalogDBErrors   prometheus.Counter
	catalogDBDuration prometheus.Histogram

	modelPublishes       prometheus.Counter
	modelPublishErrors   prometheus.Counter
	modelPublishDuration prometheus.Histogram
}

// NewMetrics registers every collector against reg and returns the bundle.
// Passing a fresh prometheus.NewRegistry() keeps tests isolated from the
// process-wide default registry.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	f := promauto.With(reg)
	return &Metrics{
		queriesTotal: f.NewCounter(prometheus.CounterOpts{
			Name: "bi_queries_total", Help: "Total number of queries.",
		}),
		queryErrors: f.NewCounter(prometheus.CounterOpts{
			Name: "bi_query_errors_total", Help: "Total number of query errors.",
		}),
		queryDuration: f.NewHistogram(prometheus.HistogramOpts{
			Name: "bi_query_duration_seconds", Help: "Query execution duration in seconds.", Buckets: queryLatencyBuckets,
		}),
		cacheHits: f.NewCounter(prometheus.CounterOpts{
			Name: "bi_cache_hits_total", Help: "Total cache hits.",
		}),
		cacheMisses: f.NewCounter(prometheus.CounterOpts{
			Name: "bi_cache_misses_total", Help: "Total cache misses.",
		}),

		aiRequestsTotal: f.NewCounter(prometheus.CounterOpts{
			Name: "bi_ai_requests_total", Help: "Total AI text-to-query requests.",
		}),
		aiErrors: f.NewCounter(prometheus.CounterOpts{
			Name: "bi_ai_errors_total", Help: "Total failed AI text-to-query requests.",
		}),
		aiRequestDuration: f.NewHistogram(prometheus.HistogramOpts{
			Name: "bi_ai_request_duration_seconds", Help: "AI text-to-query end-to-end latency in seconds.", Buckets: prometheus.DefBuckets,
		}),
		aiRetriesTotal: f.NewCounter(prometheus.CounterOpts{
			Name: "bi_ai_retries_total", Help: "Sum of LLM retry attempts across AI requests.",
		}),
		aiClarifications: f.NewCounter(prometheus.CounterOpts{
			Name: "bi_ai_clarifications_total", Help: "Total clarification responses returned to users.",
		}),
		llmRequestDuration: f.NewHistogram(prometheus.HistogramOpts{
			Name: "llm_request_duration_seconds", Help: "LLM request duration observed by AI handlers, in seconds.", Buckets: prometheus.DefBuckets,
		}),
		llmTokensUsed: f.NewCounter(prometheus.CounterOpts{
			Name: "llm_tokens_used_total", Help: "Total LLM tokens used or estimated.",
		}),
		promptBuildSeconds: f.NewHistogram(prometheus.HistogramOpts{
			Name: "prompt_build_duration_seconds", Help: "Prompt build duration in seconds.", Buckets: prometheus.DefBuckets,
		}),
		ambiguityDetected: f.NewCounter(prometheus.CounterOpts{
			Name: "biqly_ambiguity_detected_total", Help: "Total questions where ambiguity was detected.",
		}),
		ambiguityClarified: f.NewCounter(prometheus.CounterOpts{
			Name: "biqly_ambiguity_clarified_total", Help: "Total ambiguity clarifications answered by users.",
		}),
		ambiguityLatencyMS: f.NewHistogram(prometheus.HistogramOpts{
			Name: "biqly_ambiguity_latency_ms", Help: "Ambiguity analysis latency in milliseconds.", Buckets: ambiguityLatencyMSBuckets,
		}),
		ambiguityBySource: f.NewCounterVec(prometheus.CounterOpts{
			Name: "biqly_ambiguity_by_source", Help: "Total detected ambiguities by analyzer source.",
		}, []string{"source"}),

		validationFailures: f.NewCounter(prometheus.CounterOpts{
			Name: "bi_validation_failures_total", Help: "Total validation failures.",
		}),
		connectionErrors: f.NewCounter(prometheus.CounterOpts{
			Name: "bi_connection_errors_total", Help: "Total datasource connection errors.",
		}),

		queryCompiles: f.NewCounter(prometheus.CounterOpts{
			Name: "query_compile_total", Help: "Total query compile attempts.",
		}),
		queryCompileErrors: f.NewCounter(prometheus.CounterOpts{
			Name: "query_compile_errors_total", Help: "Total failed query compile attempts.",
		}),
		queryCompileDuration: f.NewHistogram(prometheus.HistogramOpts{
			Name: "query_compile_duration_seconds", Help: "Query compile duration in seconds.", Buckets: prometheus.DefBuckets,
		}),
		queryExecutions: f.NewCounter(prometheus.CounterOpts{
			Name: "query_execute_total", Help: "Total query execution attempts.",
		}),
		queryExecutionErrors: f.NewCounter(prometheus.CounterOpts{
			Name: "query_execute_errors_total", Help: "Total failed query execution attempts.",
		}),
		queryExecuteDuration: f.NewHistogram(prometheus.HistogramOpts{
			Name: "query_execute_duration_seconds", Help: "Query execution duration in seconds.", Buckets: prometheus.DefBuckets,
		}),
		queryRowsReturned: f.NewCounter(prometheus.CounterOpts{
			Name: "query_rows_returned_total", Help: "Total rows returned by query execution.",
		}),

		catalogDBQueries: f.NewCounter(prometheus.CounterOpts{
			Name: "catalog_db_queries_total", Help: "Total Catalog DB-bound handler calls.",
		}),
		catalogDBErrors: f.NewCounter(prometheus.CounterOpts{
			Name: "catalog_db_query_errors_total", Help: "Total failed Catalog DB-bound handler calls.",
		}),
		catalogDBDuration: f.NewHistogram(prometheus.HistogramOpts{
			Name: "catalog_db_query_duration_seconds", Help: "Catalog DB-bound handler duration in seconds.", Buckets: prometheus.DefBuckets,
		}),

		modelPublishes: f.NewCounter(prometheus.CounterOpts{
			Name: "model_publish_total", Help: "Total semantic model publish attempts.",
		}),
		modelPublishErrors: f.NewCounter(prometheus.CounterOpts{
			Name: "model_publish_errors_total", Help: "Total failed semantic model publish attempts.",
		}),
		modelPublishDuration: f.NewHistogram(prometheus.HistogramOpts{
			Name: "model_publish_duration_seconds", Help: "Semantic model publish duration in seconds.", Buckets: prometheus.DefBuckets,
		}),
	}
}

var (
	defaultOnce    sync.Once
	defaultMetrics *Metrics
)

// Default returns the process-wide Metrics singleton, registered against the
// Prometheus default registry. Collectors are created exactly once so repeated
// calls never trigger duplicate-registration panics.
func Default() *Metrics {
	defaultOnce.Do(func() {
		defaultMetrics = NewMetrics(prometheus.DefaultRegisterer)
	})
	return defaultMetrics
}

func msToSeconds(ms int64) float64 { return float64(ms) / 1000 }

// RecordQuery records a query execution.
func (m *Metrics) RecordQuery(durationMs int64, success, cacheHit bool) {
	m.queriesTotal.Inc()
	m.queryDuration.Observe(msToSeconds(durationMs))
	if !success {
		m.queryErrors.Inc()
	}
	if cacheHit {
		m.cacheHits.Inc()
	} else {
		m.cacheMisses.Inc()
	}
}

// RecordAIRequest records an AI text-to-query attempt.
func (m *Metrics) RecordAIRequest(latencyMs int64, success bool, retryCount int, clarification bool) {
	m.aiRequestsTotal.Inc()
	m.aiRequestDuration.Observe(msToSeconds(latencyMs))
	if retryCount > 0 {
		m.aiRetriesTotal.Add(float64(retryCount))
	}
	if clarification {
		m.aiClarifications.Inc()
	}
	if !success {
		m.aiErrors.Inc()
	}
}

// RecordLLMRequest records LLM-facing latency, token usage, and prompt build time.
func (m *Metrics) RecordLLMRequest(latencyMs int64, tokensUsed int, promptBuildMs int64) {
	m.llmRequestDuration.Observe(msToSeconds(latencyMs))
	if tokensUsed > 0 {
		m.llmTokensUsed.Add(float64(tokensUsed))
	}
	m.promptBuildSeconds.Observe(msToSeconds(promptBuildMs))
}

// RecordAmbiguityAnalysis records one deterministic or LLM-backed ambiguity pass.
func (m *Metrics) RecordAmbiguityAnalysis(latencyMs int64, source string, detected bool) {
	m.ambiguityLatencyMS.Observe(float64(latencyMs))
	if detected {
		m.ambiguityDetected.Inc()
		m.ambiguityBySource.WithLabelValues(source).Inc()
	}
}

// RecordAmbiguityClarified records a user answer to an ambiguity clarification.
func (m *Metrics) RecordAmbiguityClarified() { m.ambiguityClarified.Inc() }

// RecordCatalogDBQuery records a Catalog-owned handler call that performs
// metadata DB work.
func (m *Metrics) RecordCatalogDBQuery(durationMs int64, success bool) {
	m.catalogDBQueries.Inc()
	m.catalogDBDuration.Observe(msToSeconds(durationMs))
	if !success {
		m.catalogDBErrors.Inc()
	}
}

// RecordModelPublish records semantic model publish latency.
func (m *Metrics) RecordModelPublish(durationMs int64, success bool) {
	m.modelPublishes.Inc()
	m.modelPublishDuration.Observe(msToSeconds(durationMs))
	if !success {
		m.modelPublishErrors.Inc()
	}
}

// RecordQueryCompile records a LogicalQuery → SQL compile attempt.
func (m *Metrics) RecordQueryCompile(durationMs int64, success bool) {
	m.queryCompiles.Inc()
	m.queryCompileDuration.Observe(msToSeconds(durationMs))
	if !success {
		m.queryCompileErrors.Inc()
	}
}

// RecordQueryExecution records a SQL execution attempt and rows returned.
func (m *Metrics) RecordQueryExecution(durationMs int64, success bool, rows int) {
	m.queryExecutions.Inc()
	m.queryExecuteDuration.Observe(msToSeconds(durationMs))
	if !success {
		m.queryExecutionErrors.Inc()
	}
	if rows > 0 {
		m.queryRowsReturned.Add(float64(rows))
	}
}

// RecordValidationFailure records a semantic validation failure.
func (m *Metrics) RecordValidationFailure() { m.validationFailures.Inc() }

// RecordConnectionError records a datasource connection error.
func (m *Metrics) RecordConnectionError() { m.connectionErrors.Inc() }
