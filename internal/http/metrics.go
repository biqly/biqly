// Package http provides HTTP handlers, middleware, and metrics.
package http

import (
	"net/http"

	"github.com/biqly/biqly/internal/platform/observability"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics is the process-local metrics recorder. It aliases the centralized
// Prometheus-backed collector in internal/platform/observability so the API,
// query, catalog, and AI routers share one consistent /metrics surface with
// the auth service. The Record* methods satisfy the recorder interfaces in
// internal/http/handlers (AIMetricsRecorder, QueryMetricsRecorder,
// CatalogMetricsRecorder).
type Metrics = observability.Metrics

// GetMetrics returns the process-wide metrics singleton.
func GetMetrics() *Metrics {
	return observability.Default()
}

// metricsHandler is the shared Prometheus exposition handler. It gathers the
// default registry, which includes the BI engine collectors plus the standard
// Go runtime and process collectors registered by client_golang.
var metricsHandler = promhttp.Handler()

// MetricsHandler serves the /metrics endpoint in Prometheus text exposition
// format.
func MetricsHandler(w http.ResponseWriter, r *http.Request) {
	metricsHandler.ServeHTTP(w, r)
}
