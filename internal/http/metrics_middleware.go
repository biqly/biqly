package http

import (
	"net/http"
	"time"

	"github.com/biqly/biqly/internal/http/response"
	"github.com/go-chi/chi/v5"
)

// HTTPMetricsMiddleware records biqly_http_* Prometheus metrics for every request.
func HTTPMetricsMiddleware(metrics *Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if metrics == nil {
				next.ServeHTTP(w, r)
				return
			}
			rec := response.NewStatusRecorder(w)
			start := time.Now()
			next.ServeHTTP(rec, r)
			routePattern := r.URL.Path
			if rc := chi.RouteContext(r.Context()); rc != nil && rc.RoutePattern() != "" {
				routePattern = rc.RoutePattern()
			}
			metrics.RecordHTTPRequest(r.Method, routePattern, rec.Status(), time.Since(start).Milliseconds())
		})
	}
}
