package http

import (
	"net/http"
	"strings"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// otelRouteFilter skips high-churn, low-signal paths from HTTP server spans.
func otelRouteFilter(r *http.Request) bool {
	switch r.URL.Path {
	case "/health", "/healthz", "/ready", "/readyz", "/metrics":
		return false
	}
	if strings.HasSuffix(r.URL.Path, "/health") && strings.Contains(r.URL.Path, "/internal") {
		return false
	}
	return true
}

// OTELHTTPHandler wraps handler with otelhttp server instrumentation when a
// non-noop TracerProvider is configured. operation becomes the span name prefix
// (e.g. biqly-query); use one stable name per service binary.
func OTELHTTPHandler(operation string, handler http.Handler) http.Handler {
	return otelhttp.NewHandler(handler, operation, otelhttp.WithFilter(otelRouteFilter))
}
