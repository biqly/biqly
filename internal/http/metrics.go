// Package http provides HTTP handlers, middleware, and metrics.
package http

import (
	"fmt"
	"net/http"
	"runtime"
	"sync"
	"time"
)

var processStart = time.Now()

// Metrics exposes Prometheus-style metrics (simplified).
type Metrics struct {
	mu sync.Mutex

	TotalQueries  int64
	QueryErrors   int64
	QueryDuration time.Duration
	CacheHits     int64
	CacheMisses   int64

	AITotalRequests     int64
	AIErrors            int64
	AILatency           time.Duration
	AITotalRetries      int64
	AIClarifications    int64
	LLMRequestDuration  time.Duration
	LLMTokensUsed       int64
	PromptBuildDuration time.Duration

	ConnectionErrors int64

	ValidationFailures int64

	QueryDurationBuckets map[string]int64

	QueryCompiles        int64
	QueryCompileErrors   int64
	QueryCompileDuration time.Duration
	QueryExecutions      int64
	QueryExecutionErrors int64
	QueryExecuteDuration time.Duration
	QueryRowsReturned    int64

	CatalogDBQueries  int64
	CatalogDBErrors   int64
	CatalogDBDuration time.Duration

	ModelPublishes       int64
	ModelPublishErrors   int64
	ModelPublishDuration time.Duration
}

var globalMetrics = &Metrics{
	QueryDurationBuckets: make(map[string]int64),
}

// GetMetrics returns the global metrics instance.
func GetMetrics() *Metrics {
	return globalMetrics
}

// RecordQuery records a query execution.
func (m *Metrics) RecordQuery(durationMs int64, success bool, cacheHit bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.TotalQueries++
	m.QueryDuration += time.Duration(durationMs) * time.Millisecond

	if !success {
		m.QueryErrors++
	}

	if cacheHit {
		m.CacheHits++
	} else {
		m.CacheMisses++
	}

	bucket := durationBucket(durationMs)
	m.QueryDurationBuckets[bucket]++
}

// RecordAIRequest records an AI text-to-query attempt for /metrics and ops dashboards.
func (m *Metrics) RecordAIRequest(latencyMs int64, success bool, retryCount int, clarification bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.AITotalRequests++
	m.AILatency += time.Duration(latencyMs) * time.Millisecond
	if retryCount > 0 {
		m.AITotalRetries += int64(retryCount)
	}
	if clarification {
		m.AIClarifications++
	}
	if !success {
		m.AIErrors++
	}
}

// RecordLLMRequest records AI Service LLM-facing latency and token usage.
func (m *Metrics) RecordLLMRequest(latencyMs int64, tokensUsed int, promptBuildMs int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.LLMRequestDuration += time.Duration(latencyMs) * time.Millisecond
	if tokensUsed > 0 {
		m.LLMTokensUsed += int64(tokensUsed)
	}
	m.PromptBuildDuration += time.Duration(promptBuildMs) * time.Millisecond
}

// RecordQueryCompile records query compile latency.
func (m *Metrics) RecordQueryCompile(durationMs int64, success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.QueryCompiles++
	m.QueryCompileDuration += time.Duration(durationMs) * time.Millisecond
	if !success {
		m.QueryCompileErrors++
	}
}

// RecordQueryExecution records query execution latency and returned rows.
func (m *Metrics) RecordQueryExecution(durationMs int64, success bool, rows int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.QueryExecutions++
	m.QueryExecuteDuration += time.Duration(durationMs) * time.Millisecond
	if rows > 0 {
		m.QueryRowsReturned += int64(rows)
	}
	if !success {
		m.QueryExecutionErrors++
	}
}

// RecordCatalogDBQuery records a Catalog-owned handler call that performs
// metadata DB work. Until repositories expose lower-level hooks, handler
// latency is the closest process-local proxy for catalog DB time.
func (m *Metrics) RecordCatalogDBQuery(durationMs int64, success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.CatalogDBQueries++
	m.CatalogDBDuration += time.Duration(durationMs) * time.Millisecond
	if !success {
		m.CatalogDBErrors++
	}
}

// RecordModelPublish records semantic model publish latency.
func (m *Metrics) RecordModelPublish(durationMs int64, success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.ModelPublishes++
	m.ModelPublishDuration += time.Duration(durationMs) * time.Millisecond
	if !success {
		m.ModelPublishErrors++
	}
}

// RecordValidationFailure records a validation failure.
func (m *Metrics) RecordValidationFailure() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ValidationFailures++
}

// RecordConnectionError records a connection error.
func (m *Metrics) RecordConnectionError() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.ConnectionErrors++
}

func durationBucket(ms int64) string {
	switch {
	case ms < 10:
		return "0-10ms"
	case ms < 50:
		return "10-50ms"
	case ms < 100:
		return "50-100ms"
	case ms < 500:
		return "100-500ms"
	case ms < 1000:
		return "500-1000ms"
	default:
		return "1000ms+"
	}
}

// MetricsHandler serves the /metrics endpoint.
func MetricsHandler(w http.ResponseWriter, r *http.Request) {
	m := GetMetrics()
	m.mu.Lock()
	defer m.mu.Unlock()

	w.Header().Set("Content-Type", "text/plain")

	writeMetricsLine := func(format string, args ...any) {
		_, _ = fmt.Fprintf(w, format, args...)
	}

	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	writeMetricsLine("# HELP go_goroutines Number of goroutines that currently exist\n")
	writeMetricsLine("# TYPE go_goroutines gauge\n")
	writeMetricsLine("go_goroutines %d\n", runtime.NumGoroutine())

	writeMetricsLine("# HELP go_memstats_alloc_bytes Bytes of allocated heap objects\n")
	writeMetricsLine("# TYPE go_memstats_alloc_bytes gauge\n")
	writeMetricsLine("go_memstats_alloc_bytes %d\n", mem.Alloc)

	writeMetricsLine("# HELP process_uptime_seconds Process uptime in seconds\n")
	writeMetricsLine("# TYPE process_uptime_seconds counter\n")
	writeMetricsLine("process_uptime_seconds %.3f\n", time.Since(processStart).Seconds())

	writeMetricsLine("# HELP bi_queries_total Total number of queries\n")
	writeMetricsLine("# TYPE bi_queries_total counter\n")
	writeMetricsLine("bi_queries_total %d\n", m.TotalQueries)

	writeMetricsLine("# HELP bi_query_errors Total number of query errors\n")
	writeMetricsLine("# TYPE bi_query_errors counter\n")
	writeMetricsLine("bi_query_errors %d\n", m.QueryErrors)

	writeMetricsLine("# HELP bi_cache_hits Total cache hits\n")
	writeMetricsLine("# TYPE bi_cache_hits counter\n")
	writeMetricsLine("bi_cache_hits %d\n", m.CacheHits)

	writeMetricsLine("# HELP bi_cache_misses Total cache misses\n")
	writeMetricsLine("# TYPE bi_cache_misses counter\n")
	writeMetricsLine("bi_cache_misses %d\n", m.CacheMisses)

	writeMetricsLine("# HELP bi_ai_requests_total Total AI requests\n")
	writeMetricsLine("# TYPE bi_ai_requests_total counter\n")
	writeMetricsLine("bi_ai_requests_total %d\n", m.AITotalRequests)

	writeMetricsLine("# HELP bi_ai_errors Total AI errors\n")
	writeMetricsLine("# TYPE bi_ai_errors counter\n")
	writeMetricsLine("bi_ai_errors %d\n", m.AIErrors)

	writeMetricsLine("# HELP bi_ai_retries_total Sum of LLM retry attempts\n")
	writeMetricsLine("# TYPE bi_ai_retries_total counter\n")
	writeMetricsLine("bi_ai_retries_total %d\n", m.AITotalRetries)

	writeMetricsLine("# HELP bi_ai_clarifications_total Clarification responses\n")
	writeMetricsLine("# TYPE bi_ai_clarifications_total counter\n")
	writeMetricsLine("bi_ai_clarifications_total %d\n", m.AIClarifications)

	writeMetricsLine("# HELP llm_request_duration_seconds Total LLM request duration observed by AI handlers\n")
	writeMetricsLine("# TYPE llm_request_duration_seconds counter\n")
	writeMetricsLine("llm_request_duration_seconds %.3f\n", m.LLMRequestDuration.Seconds())

	writeMetricsLine("# HELP llm_tokens_used_total Total LLM tokens used or estimated\n")
	writeMetricsLine("# TYPE llm_tokens_used_total counter\n")
	writeMetricsLine("llm_tokens_used_total %d\n", m.LLMTokensUsed)

	writeMetricsLine("# HELP prompt_build_duration_seconds Total prompt build duration\n")
	writeMetricsLine("# TYPE prompt_build_duration_seconds counter\n")
	writeMetricsLine("prompt_build_duration_seconds %.3f\n", m.PromptBuildDuration.Seconds())

	if m.AITotalRequests > 0 {
		failureRate := float64(m.AIErrors) / float64(m.AITotalRequests)
		avgRetries := float64(m.AITotalRetries) / float64(m.AITotalRequests)
		writeMetricsLine("# HELP bi_ai_failure_rate Approximate failure rate (process lifetime)\n")
		writeMetricsLine("# TYPE bi_ai_failure_rate gauge\n")
		writeMetricsLine("bi_ai_failure_rate %.6f\n", failureRate)
		writeMetricsLine("# HELP bi_ai_avg_retry_count Approximate avg retries per request (process lifetime)\n")
		writeMetricsLine("# TYPE bi_ai_avg_retry_count gauge\n")
		writeMetricsLine("bi_ai_avg_retry_count %.6f\n", avgRetries)
	}

	writeMetricsLine("# HELP bi_validation_failures Total validation failures\n")
	writeMetricsLine("# TYPE bi_validation_failures counter\n")
	writeMetricsLine("bi_validation_failures %d\n", m.ValidationFailures)

	writeMetricsLine("# HELP bi_connection_errors Total connection errors\n")
	writeMetricsLine("# TYPE bi_connection_errors counter\n")
	writeMetricsLine("bi_connection_errors %d\n", m.ConnectionErrors)

	writeMetricsLine("# HELP query_compile_total Total query compile attempts\n")
	writeMetricsLine("# TYPE query_compile_total counter\n")
	writeMetricsLine("query_compile_total %d\n", m.QueryCompiles)

	writeMetricsLine("# HELP query_compile_errors_total Total failed query compile attempts\n")
	writeMetricsLine("# TYPE query_compile_errors_total counter\n")
	writeMetricsLine("query_compile_errors_total %d\n", m.QueryCompileErrors)

	writeMetricsLine("# HELP query_compile_duration_seconds Total query compile duration\n")
	writeMetricsLine("# TYPE query_compile_duration_seconds counter\n")
	writeMetricsLine("query_compile_duration_seconds %.3f\n", m.QueryCompileDuration.Seconds())

	writeMetricsLine("# HELP query_execute_total Total query execution attempts\n")
	writeMetricsLine("# TYPE query_execute_total counter\n")
	writeMetricsLine("query_execute_total %d\n", m.QueryExecutions)

	writeMetricsLine("# HELP query_execute_errors_total Total failed query execution attempts\n")
	writeMetricsLine("# TYPE query_execute_errors_total counter\n")
	writeMetricsLine("query_execute_errors_total %d\n", m.QueryExecutionErrors)

	writeMetricsLine("# HELP query_execute_duration_seconds Total query execution duration\n")
	writeMetricsLine("# TYPE query_execute_duration_seconds counter\n")
	writeMetricsLine("query_execute_duration_seconds %.3f\n", m.QueryExecuteDuration.Seconds())

	writeMetricsLine("# HELP query_rows_returned Total rows returned by query execution\n")
	writeMetricsLine("# TYPE query_rows_returned counter\n")
	writeMetricsLine("query_rows_returned %d\n", m.QueryRowsReturned)

	writeMetricsLine("# HELP catalog_db_queries_total Total Catalog DB-bound handler calls\n")
	writeMetricsLine("# TYPE catalog_db_queries_total counter\n")
	writeMetricsLine("catalog_db_queries_total %d\n", m.CatalogDBQueries)

	writeMetricsLine("# HELP catalog_db_query_errors_total Total failed Catalog DB-bound handler calls\n")
	writeMetricsLine("# TYPE catalog_db_query_errors_total counter\n")
	writeMetricsLine("catalog_db_query_errors_total %d\n", m.CatalogDBErrors)

	writeMetricsLine("# HELP catalog_db_query_duration_seconds Total Catalog DB-bound handler duration\n")
	writeMetricsLine("# TYPE catalog_db_query_duration_seconds counter\n")
	writeMetricsLine("catalog_db_query_duration_seconds %.3f\n", m.CatalogDBDuration.Seconds())

	writeMetricsLine("# HELP model_publish_total Total semantic model publish attempts\n")
	writeMetricsLine("# TYPE model_publish_total counter\n")
	writeMetricsLine("model_publish_total %d\n", m.ModelPublishes)

	writeMetricsLine("# HELP model_publish_errors_total Total failed semantic model publish attempts\n")
	writeMetricsLine("# TYPE model_publish_errors_total counter\n")
	writeMetricsLine("model_publish_errors_total %d\n", m.ModelPublishErrors)

	writeMetricsLine("# HELP model_publish_duration_seconds Total semantic model publish duration\n")
	writeMetricsLine("# TYPE model_publish_duration_seconds counter\n")
	writeMetricsLine("model_publish_duration_seconds %.3f\n", m.ModelPublishDuration.Seconds())

	writeMetricsLine("# HELP bi_query_duration_seconds Total query duration\n")
	writeMetricsLine("# TYPE bi_query_duration_seconds counter\n")
	writeMetricsLine("bi_query_duration_seconds %.3f\n", m.QueryDuration.Seconds())

	for bucket, count := range m.QueryDurationBuckets {
		writeMetricsLine("bi_query_duration_bucket{le=\"%s\"} %d\n", bucket, count)
	}
}
