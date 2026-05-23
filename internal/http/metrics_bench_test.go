package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// BenchmarkMetricsRecordConcurrent measures the cost of the hot-path Record*
// calls under contention. With the old sync.Mutex implementation, every
// counter increment serialized through one lock; the atomic.Int64 refactor
// should keep ns/op flat as parallelism increases.
//
// Run with:
//
//	go test -bench=BenchmarkMetricsRecord -benchmem ./internal/http/...
func BenchmarkMetricsRecordConcurrent(b *testing.B) {
	old := globalMetrics
	b.Cleanup(func() { globalMetrics = old })
	globalMetrics = &Metrics{}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			globalMetrics.RecordQuery(42, true, false)
			globalMetrics.RecordAIRequest(120, true, 0, false)
			globalMetrics.RecordLLMRequest(95, 512, 8)
		}
	})
}

// BenchmarkMetricsHandlerConcurrent measures /metrics throughput while
// Record* calls happen in parallel. With the mutex implementation the
// handler serialized behind every increment; with atomic counters they
// share no critical section.
func BenchmarkMetricsHandlerConcurrent(b *testing.B) {
	old := globalMetrics
	b.Cleanup(func() { globalMetrics = old })
	globalMetrics = &Metrics{}

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
					globalMetrics.RecordQuery(10, true, false)
				}
			}
		}()
	}
	b.Cleanup(func() {
		close(stop)
		wg.Wait()
	})

	b.ResetTimer()
	for range b.N {
		rec := httptest.NewRecorder()
		req := httptest.NewRequestWithContext(context.Background(), http.MethodGet, "/metrics", nil)
		MetricsHandler(rec, req)
	}
}
