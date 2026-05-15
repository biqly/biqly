// Package http provides HTTP handlers, middleware, and metrics.
package http

import (
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Metrics exposes Prometheus-style metrics (simplified).
type Metrics struct {
	mu sync.Mutex

	TotalQueries   int64
	QueryErrors    int64
	QueryDuration  time.Duration
	CacheHits      int64
	CacheMisses    int64

	AITotalRequests int64
	AIErrors        int64
	AILatency       time.Duration
	AITotalRetries  int64
	AIClarifications int64

	ConnectionErrors int64

	ValidationFailures int64

	QueryDurationBuckets map[string]int64
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

	writeMetricsLine("# HELP bi_query_duration_seconds Total query duration\n")
	writeMetricsLine("# TYPE bi_query_duration_seconds counter\n")
	writeMetricsLine("bi_query_duration_seconds %.3f\n", m.QueryDuration.Seconds())

	for bucket, count := range m.QueryDurationBuckets {
		writeMetricsLine("bi_query_duration_bucket{le=\"%s\"} %d\n", bucket, count)
	}
}
