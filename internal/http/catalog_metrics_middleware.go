package http

import (
	"net/http"
	"time"
)

// CatalogMetricsMiddleware records process-local Catalog route latency.
func CatalogMetricsMiddleware(metrics *Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if metrics == nil {
				next.ServeHTTP(w, r)
				return
			}
			if r.URL.Path == "/internal/health" {
				next.ServeHTTP(w, r)
				return
			}
			rec := &metricsStatusRecorder{ResponseWriter: w, status: http.StatusOK}
			start := time.Now()
			next.ServeHTTP(rec, r)
			metrics.RecordCatalogDBQuery(time.Since(start).Milliseconds(), rec.status < http.StatusInternalServerError)
		})
	}
}

type metricsStatusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *metricsStatusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}
