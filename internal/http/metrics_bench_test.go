package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// BenchmarkMetricsRecordConcurrent measures the cost of the hot-path Record*
// calls under contention. The collectors are Prometheus counters/histograms,
// which update lock-free on the hot path, so ns/op should stay flat as
// parallelism increases.
//
// Run with:
//
//	go test -bench=BenchmarkMetricsRecord -benchmem ./internal/http/...
func BenchmarkMetricsRecordConcurrent(b *testing.B) {
	m := GetMetrics()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			m.RecordQuery(42, true, false)
			m.RecordAIRequest(120, true, 0, false)
			m.RecordLLMRequest(95, 512, 8)
		}
	})
}

// BenchmarkMetricsHandlerConcurrent measures /metrics throughput while Record*
// calls happen in parallel.
func BenchmarkMetricsHandlerConcurrent(b *testing.B) {
	m := GetMetrics()

	// Background writers warm the counters.
	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(4)
	for range 4 {
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
					m.RecordQuery(10, true, false)
				}
			}
		}()
	}
	b.Cleanup(func() {
		close(stop)
		wg.Wait()
	})

	for b.Loop() {
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", nil)
		MetricsHandler(rec, req)
	}
}
