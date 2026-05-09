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

	// Query metrics
	TotalQueries   int64
	QueryErrors    int64
	QueryDuration  time.Duration
	CacheHits      int64
	CacheMisses    int64

	// AI metrics
	AITotalRequests  int64
	AIErrors         int64
	AILatency        time.Duration

	// Datasource metrics
	ConnectionErrors int64

	// Validation metrics
	ValidationFailures int64

	// Per-status histograms (simplified as counters)
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

	// Bucket
	bucket := durationBucket(durationMs)
	m.QueryDurationBuckets[bucket]++
}

// RecordAIRequest records an AI request.
func (m *Metrics) RecordAIRequest(latencyMs int64, success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.AITotalRequests++
	m.AILatency += time.Duration(latencyMs) * time.Millisecond

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

	fmt.Fprintf(w, "# HELP bi_queries_total Total number of queries\n")
	fmt.Fprintf(w, "# TYPE bi_queries_total counter\n")
	fmt.Fprintf(w, "bi_queries_total %d\n", m.TotalQueries)

	fmt.Fprintf(w, "# HELP bi_query_errors Total number of query errors\n")
	fmt.Fprintf(w, "# TYPE bi_query_errors counter\n")
	fmt.Fprintf(w, "bi_query_errors %d\n", m.QueryErrors)

	fmt.Fprintf(w, "# HELP bi_cache_hits Total cache hits\n")
	fmt.Fprintf(w, "# TYPE bi_cache_hits counter\n")
	fmt.Fprintf(w, "bi_cache_hits %d\n", m.CacheHits)

	fmt.Fprintf(w, "# HELP bi_cache_misses Total cache misses\n")
	fmt.Fprintf(w, "# TYPE bi_cache_misses counter\n")
	fmt.Fprintf(w, "bi_cache_misses %d\n", m.CacheMisses)

	fmt.Fprintf(w, "# HELP bi_ai_requests_total Total AI requests\n")
	fmt.Fprintf(w, "# TYPE bi_ai_requests_total counter\n")
	fmt.Fprintf(w, "bi_ai_requests_total %d\n", m.AITotalRequests)

	fmt.Fprintf(w, "# HELP bi_ai_errors Total AI errors\n")
	fmt.Fprintf(w, "# TYPE bi_ai_errors counter\n")
	fmt.Fprintf(w, "bi_ai_errors %d\n", m.AIErrors)

	fmt.Fprintf(w, "# HELP bi_validation_failures Total validation failures\n")
	fmt.Fprintf(w, "# TYPE bi_validation_failures counter\n")
	fmt.Fprintf(w, "bi_validation_failures %d\n", m.ValidationFailures)

	fmt.Fprintf(w, "# HELP bi_connection_errors Total connection errors\n")
	fmt.Fprintf(w, "# TYPE bi_connection_errors counter\n")
	fmt.Fprintf(w, "bi_connection_errors %d\n", m.ConnectionErrors)

	fmt.Fprintf(w, "# HELP bi_query_duration_seconds Total query duration\n")
	fmt.Fprintf(w, "# TYPE bi_query_duration_seconds counter\n")
	fmt.Fprintf(w, "bi_query_duration_seconds %.3f\n", m.QueryDuration.Seconds())

	for bucket, count := range m.QueryDurationBuckets {
		fmt.Fprintf(w, "bi_query_duration_bucket{le=\"%s\"} %d\n", bucket, count)
	}
}
