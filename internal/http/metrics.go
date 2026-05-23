// Package http provides HTTP handlers, middleware, and metrics.
package http

import (
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"sync/atomic"
	"time"
)

var processStart = time.Now()

// durationBucketCount is the number of fixed latency buckets exposed via
// /metrics. Kept here so the array length and durationBucketIndex stay in
// sync — adding a bucket means bumping this constant.
const durationBucketCount = 6

var durationBucketLabels = [durationBucketCount]string{
	"0-10ms", "10-50ms", "50-100ms", "100-500ms", "500-1000ms", "1000ms+",
}

// Metrics exposes Prometheus-style metrics (simplified).
//
// Counters use atomic.Int64 so /metrics scrapes don't serialize behind a
// process-wide mutex while hot paths increment counters. time.Duration
// counters are stored as nanoseconds inside atomic.Int64 to keep all of them
// uniform.
type Metrics struct {
	TotalQueries  atomic.Int64
	QueryErrors   atomic.Int64
	QueryDuration atomic.Int64 // ns
	CacheHits     atomic.Int64
	CacheMisses   atomic.Int64

	AITotalRequests     atomic.Int64
	AIErrors            atomic.Int64
	AILatency           atomic.Int64 // ns
	AITotalRetries      atomic.Int64
	AIClarifications    atomic.Int64
	LLMRequestDuration  atomic.Int64 // ns
	LLMTokensUsed       atomic.Int64
	PromptBuildDuration atomic.Int64 // ns

	ConnectionErrors atomic.Int64

	ValidationFailures atomic.Int64

	QueryDurationBuckets [durationBucketCount]atomic.Int64

	QueryCompiles        atomic.Int64
	QueryCompileErrors   atomic.Int64
	QueryCompileDuration atomic.Int64 // ns
	QueryExecutions      atomic.Int64
	QueryExecutionErrors atomic.Int64
	QueryExecuteDuration atomic.Int64 // ns
	QueryRowsReturned    atomic.Int64

	CatalogDBQueries  atomic.Int64
	CatalogDBErrors   atomic.Int64
	CatalogDBDuration atomic.Int64 // ns

	ModelPublishes       atomic.Int64
	ModelPublishErrors   atomic.Int64
	ModelPublishDuration atomic.Int64 // ns
}

var globalMetrics = &Metrics{}

// GetMetrics returns the global metrics instance.
func GetMetrics() *Metrics {
	return globalMetrics
}

// RecordQuery records a query execution.
func (m *Metrics) RecordQuery(durationMs int64, success bool, cacheHit bool) {
	m.TotalQueries.Add(1)
	m.QueryDuration.Add(durationMs * int64(time.Millisecond))
	if !success {
		m.QueryErrors.Add(1)
	}
	if cacheHit {
		m.CacheHits.Add(1)
	} else {
		m.CacheMisses.Add(1)
	}
	m.QueryDurationBuckets[durationBucketIndex(durationMs)].Add(1)
}

// RecordAIRequest records an AI text-to-query attempt for /metrics and ops dashboards.
func (m *Metrics) RecordAIRequest(latencyMs int64, success bool, retryCount int, clarification bool) {
	m.AITotalRequests.Add(1)
	m.AILatency.Add(latencyMs * int64(time.Millisecond))
	if retryCount > 0 {
		m.AITotalRetries.Add(int64(retryCount))
	}
	if clarification {
		m.AIClarifications.Add(1)
	}
	if !success {
		m.AIErrors.Add(1)
	}
}

// RecordLLMRequest records AI Service LLM-facing latency and token usage.
func (m *Metrics) RecordLLMRequest(latencyMs int64, tokensUsed int, promptBuildMs int64) {
	m.LLMRequestDuration.Add(latencyMs * int64(time.Millisecond))
	if tokensUsed > 0 {
		m.LLMTokensUsed.Add(int64(tokensUsed))
	}
	m.PromptBuildDuration.Add(promptBuildMs * int64(time.Millisecond))
}

// RecordQueryCompile records query compile latency.
func (m *Metrics) RecordQueryCompile(durationMs int64, success bool) {
	m.QueryCompiles.Add(1)
	m.QueryCompileDuration.Add(durationMs * int64(time.Millisecond))
	if !success {
		m.QueryCompileErrors.Add(1)
	}
}

// RecordQueryExecution records query execution latency and returned rows.
func (m *Metrics) RecordQueryExecution(durationMs int64, success bool, rows int) {
	m.QueryExecutions.Add(1)
	m.QueryExecuteDuration.Add(durationMs * int64(time.Millisecond))
	if rows > 0 {
		m.QueryRowsReturned.Add(int64(rows))
	}
	if !success {
		m.QueryExecutionErrors.Add(1)
	}
}

// RecordCatalogDBQuery records a Catalog-owned handler call that performs
// metadata DB work. Until repositories expose lower-level hooks, handler
// latency is the closest process-local proxy for catalog DB time.
func (m *Metrics) RecordCatalogDBQuery(durationMs int64, success bool) {
	m.CatalogDBQueries.Add(1)
	m.CatalogDBDuration.Add(durationMs * int64(time.Millisecond))
	if !success {
		m.CatalogDBErrors.Add(1)
	}
}

// RecordModelPublish records semantic model publish latency.
func (m *Metrics) RecordModelPublish(durationMs int64, success bool) {
	m.ModelPublishes.Add(1)
	m.ModelPublishDuration.Add(durationMs * int64(time.Millisecond))
	if !success {
		m.ModelPublishErrors.Add(1)
	}
}

// RecordValidationFailure records a validation failure.
func (m *Metrics) RecordValidationFailure() {
	m.ValidationFailures.Add(1)
}

// RecordConnectionError records a connection error.
func (m *Metrics) RecordConnectionError() {
	m.ConnectionErrors.Add(1)
}

func durationBucketIndex(ms int64) int {
	switch {
	case ms < 10:
		return 0
	case ms < 50:
		return 1
	case ms < 100:
		return 2
	case ms < 500:
		return 3
	case ms < 1000:
		return 4
	default:
		return 5
	}
}

// memStatsCache memoizes runtime.ReadMemStats for a short interval so a
// burst of /metrics scrapes does not trigger one stop-the-world snapshot
// per request. The TTL is generous enough to amortize cost yet short
// enough that operators still see fresh data.
var memStatsCache struct {
	mu     sync.Mutex
	stats  runtime.MemStats
	loaded bool
	at     time.Time
}

const memStatsTTL = 5 * time.Second

func readMemStatsCached() runtime.MemStats {
	memStatsCache.mu.Lock()
	defer memStatsCache.mu.Unlock()
	if memStatsCache.loaded && time.Since(memStatsCache.at) < memStatsTTL {
		return memStatsCache.stats
	}
	runtime.ReadMemStats(&memStatsCache.stats)
	memStatsCache.loaded = true
	memStatsCache.at = time.Now()
	return memStatsCache.stats
}

// MetricsHandler serves the /metrics endpoint.
func MetricsHandler(w http.ResponseWriter, _ *http.Request) {
	m := GetMetrics()

	w.Header().Set("Content-Type", "text/plain")

	writeMetricsLine := func(format string, args ...any) {
		_, _ = fmt.Fprintf(w, format, args...)
	}

	mem := readMemStatsCached()
	writeMetricsLine("# HELP go_goroutines Number of goroutines that currently exist\n")
	writeMetricsLine("# TYPE go_goroutines gauge\n")
	writeMetricsLine("go_goroutines %d\n", runtime.NumGoroutine())

	writeMetricsLine("# HELP go_memstats_alloc_bytes Bytes of allocated heap objects\n")
	writeMetricsLine("# TYPE go_memstats_alloc_bytes gauge\n")
	writeMetricsLine("go_memstats_alloc_bytes %d\n", mem.Alloc)

	writeMetricsLine("# HELP process_uptime_seconds Process uptime in seconds\n")
	writeMetricsLine("# TYPE process_uptime_seconds counter\n")
	writeMetricsLine("process_uptime_seconds %.3f\n", time.Since(processStart).Seconds())

	aiTotal := m.AITotalRequests.Load()
	aiErrors := m.AIErrors.Load()
	aiRetries := m.AITotalRetries.Load()

	writeMetricsLine("# HELP bi_queries_total Total number of queries\n")
	writeMetricsLine("# TYPE bi_queries_total counter\n")
	writeMetricsLine("bi_queries_total %d\n", m.TotalQueries.Load())

	writeMetricsLine("# HELP bi_query_errors Total number of query errors\n")
	writeMetricsLine("# TYPE bi_query_errors counter\n")
	writeMetricsLine("bi_query_errors %d\n", m.QueryErrors.Load())

	writeMetricsLine("# HELP bi_cache_hits Total cache hits\n")
	writeMetricsLine("# TYPE bi_cache_hits counter\n")
	writeMetricsLine("bi_cache_hits %d\n", m.CacheHits.Load())

	writeMetricsLine("# HELP bi_cache_misses Total cache misses\n")
	writeMetricsLine("# TYPE bi_cache_misses counter\n")
	writeMetricsLine("bi_cache_misses %d\n", m.CacheMisses.Load())

	writeMetricsLine("# HELP bi_ai_requests_total Total AI requests\n")
	writeMetricsLine("# TYPE bi_ai_requests_total counter\n")
	writeMetricsLine("bi_ai_requests_total %d\n", aiTotal)

	writeMetricsLine("# HELP bi_ai_errors Total AI errors\n")
	writeMetricsLine("# TYPE bi_ai_errors counter\n")
	writeMetricsLine("bi_ai_errors %d\n", aiErrors)

	writeMetricsLine("# HELP bi_ai_retries_total Sum of LLM retry attempts\n")
	writeMetricsLine("# TYPE bi_ai_retries_total counter\n")
	writeMetricsLine("bi_ai_retries_total %d\n", aiRetries)

	writeMetricsLine("# HELP bi_ai_clarifications_total Clarification responses\n")
	writeMetricsLine("# TYPE bi_ai_clarifications_total counter\n")
	writeMetricsLine("bi_ai_clarifications_total %d\n", m.AIClarifications.Load())

	writeMetricsLine("# HELP llm_request_duration_seconds Total LLM request duration observed by AI handlers\n")
	writeMetricsLine("# TYPE llm_request_duration_seconds counter\n")
	writeMetricsLine("llm_request_duration_seconds %.3f\n", nsToSeconds(m.LLMRequestDuration.Load()))

	writeMetricsLine("# HELP llm_tokens_used_total Total LLM tokens used or estimated\n")
	writeMetricsLine("# TYPE llm_tokens_used_total counter\n")
	writeMetricsLine("llm_tokens_used_total %d\n", m.LLMTokensUsed.Load())

	writeMetricsLine("# HELP prompt_build_duration_seconds Total prompt build duration\n")
	writeMetricsLine("# TYPE prompt_build_duration_seconds counter\n")
	writeMetricsLine("prompt_build_duration_seconds %.3f\n", nsToSeconds(m.PromptBuildDuration.Load()))

	if aiTotal > 0 {
		failureRate := float64(aiErrors) / float64(aiTotal)
		avgRetries := float64(aiRetries) / float64(aiTotal)
		writeMetricsLine("# HELP bi_ai_failure_rate Approximate failure rate (process lifetime)\n")
		writeMetricsLine("# TYPE bi_ai_failure_rate gauge\n")
		writeMetricsLine("bi_ai_failure_rate %.6f\n", failureRate)
		writeMetricsLine("# HELP bi_ai_avg_retry_count Approximate avg retries per request (process lifetime)\n")
		writeMetricsLine("# TYPE bi_ai_avg_retry_count gauge\n")
		writeMetricsLine("bi_ai_avg_retry_count %.6f\n", avgRetries)
	}

	writeMetricsLine("# HELP bi_validation_failures Total validation failures\n")
	writeMetricsLine("# TYPE bi_validation_failures counter\n")
	writeMetricsLine("bi_validation_failures %d\n", m.ValidationFailures.Load())

	writeMetricsLine("# HELP bi_connection_errors Total connection errors\n")
	writeMetricsLine("# TYPE bi_connection_errors counter\n")
	writeMetricsLine("bi_connection_errors %d\n", m.ConnectionErrors.Load())

	writeMetricsLine("# HELP query_compile_total Total query compile attempts\n")
	writeMetricsLine("# TYPE query_compile_total counter\n")
	writeMetricsLine("query_compile_total %d\n", m.QueryCompiles.Load())

	writeMetricsLine("# HELP query_compile_errors_total Total failed query compile attempts\n")
	writeMetricsLine("# TYPE query_compile_errors_total counter\n")
	writeMetricsLine("query_compile_errors_total %d\n", m.QueryCompileErrors.Load())

	writeMetricsLine("# HELP query_compile_duration_seconds Total query compile duration\n")
	writeMetricsLine("# TYPE query_compile_duration_seconds counter\n")
	writeMetricsLine("query_compile_duration_seconds %.3f\n", nsToSeconds(m.QueryCompileDuration.Load()))

	writeMetricsLine("# HELP query_execute_total Total query execution attempts\n")
	writeMetricsLine("# TYPE query_execute_total counter\n")
	writeMetricsLine("query_execute_total %d\n", m.QueryExecutions.Load())

	writeMetricsLine("# HELP query_execute_errors_total Total failed query execution attempts\n")
	writeMetricsLine("# TYPE query_execute_errors_total counter\n")
	writeMetricsLine("query_execute_errors_total %d\n", m.QueryExecutionErrors.Load())

	writeMetricsLine("# HELP query_execute_duration_seconds Total query execution duration\n")
	writeMetricsLine("# TYPE query_execute_duration_seconds counter\n")
	writeMetricsLine("query_execute_duration_seconds %.3f\n", nsToSeconds(m.QueryExecuteDuration.Load()))

	writeMetricsLine("# HELP query_rows_returned Total rows returned by query execution\n")
	writeMetricsLine("# TYPE query_rows_returned counter\n")
	writeMetricsLine("query_rows_returned %d\n", m.QueryRowsReturned.Load())

	writeMetricsLine("# HELP catalog_db_queries_total Total Catalog DB-bound handler calls\n")
	writeMetricsLine("# TYPE catalog_db_queries_total counter\n")
	writeMetricsLine("catalog_db_queries_total %d\n", m.CatalogDBQueries.Load())

	writeMetricsLine("# HELP catalog_db_query_errors_total Total failed Catalog DB-bound handler calls\n")
	writeMetricsLine("# TYPE catalog_db_query_errors_total counter\n")
	writeMetricsLine("catalog_db_query_errors_total %d\n", m.CatalogDBErrors.Load())

	writeMetricsLine("# HELP catalog_db_query_duration_seconds Total Catalog DB-bound handler duration\n")
	writeMetricsLine("# TYPE catalog_db_query_duration_seconds counter\n")
	writeMetricsLine("catalog_db_query_duration_seconds %.3f\n", nsToSeconds(m.CatalogDBDuration.Load()))

	writeMetricsLine("# HELP model_publish_total Total semantic model publish attempts\n")
	writeMetricsLine("# TYPE model_publish_total counter\n")
	writeMetricsLine("model_publish_total %d\n", m.ModelPublishes.Load())

	writeMetricsLine("# HELP model_publish_errors_total Total failed semantic model publish attempts\n")
	writeMetricsLine("# TYPE model_publish_errors_total counter\n")
	writeMetricsLine("model_publish_errors_total %d\n", m.ModelPublishErrors.Load())

	writeMetricsLine("# HELP model_publish_duration_seconds Total semantic model publish duration\n")
	writeMetricsLine("# TYPE model_publish_duration_seconds counter\n")
	writeMetricsLine("model_publish_duration_seconds %.3f\n", nsToSeconds(m.ModelPublishDuration.Load()))

	writeMetricsLine("# HELP bi_query_duration_seconds Total query duration\n")
	writeMetricsLine("# TYPE bi_query_duration_seconds counter\n")
	writeMetricsLine("bi_query_duration_seconds %.3f\n", nsToSeconds(m.QueryDuration.Load()))

	for i, label := range durationBucketLabels {
		writeMetricsLine("bi_query_duration_bucket{le=\"%s\"} %d\n", label, m.QueryDurationBuckets[i].Load())
	}
}

func nsToSeconds(ns int64) float64 {
	return time.Duration(ns).Seconds()
}
