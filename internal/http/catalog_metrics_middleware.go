package http

import (
	"net/http"
	"time"

	"github.com/biqly/biqly/internal/http/response"
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
			rec := response.NewStatusRecorder(w)
			start := time.Now()
			next.ServeHTTP(rec, r)
			metrics.RecordCatalogDBQuery(time.Since(start).Milliseconds(), rec.Status() < http.StatusInternalServerError)
		})
	}
}
