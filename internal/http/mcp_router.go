package http

import (
	"net/http"
	"time"

	"github.com/biqly/biqly/internal/config"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/go-chi/chi/v5"
)

// MCPRouter builds the standalone MCP service. It is a thin governed gateway:
// every MCP tool call is dispatched through a reverse proxy to the internal API
// (BI_API_SERVICE_URL), forwarding the caller's credentials, so the exact same
// authentication, per-datasource access, RLS/PII masking, spend caps, and audit
// (channel=mcp) apply as for the UI/API. There is no second query path here —
// this service owns no database and no query logic.
func MCPRouter(cfg *config.Config) http.Handler {
	r := chi.NewRouter()

	ApplyBaseMiddleware(r, BaseMiddlewareConfig{
		Metrics: GetMetrics(),
		// A tool call may run a long AI/query request upstream; give it the AI
		// request budget plus headroom for the network hop.
		Timeout: cfg.AI.RequestTimeout() + 15*time.Second,
		SecurityHeaders: bimw.SecurityHeadersConfig{
			HSTSEnabled:           cfg.HTTP.HSTSEnabled,
			HSTSIncludeSubdomains: true,
			ContentSecurityPolicy: "default-src 'self'; frame-ancestors 'none'",
		},
	})

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(healthCheckBody)
	})
	r.Get("/ready", func(w http.ResponseWriter, _ *http.Request) {
		// The service is a stateless proxy; it is ready once the upstream API URL
		// is configured. Without it there is nothing to dispatch to.
		if cfg.Services.APIURL == "" {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"unavailable","error":"BI_API_SERVICE_URL is not configured"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(healthCheckBody)
	})
	r.Get("/metrics", MetricsHandler)

	// Dispatch target: the internal API gateway/monolith. The MCP tools synthesize
	// /api/* requests and this proxy forwards them upstream with the caller's
	// Authorization/X-API-Key, where the real policy is enforced.
	if proxy, ok := newUpstreamProxy(cfg.Services.APIURL, "BI_API_SERVICE_URL", "api"); ok {
		handler := mcpHandler(proxy)
		r.Handle("/mcp", handler)
		r.Handle("/mcp/*", handler)
	}

	return OTELHTTPHandler("biqly-mcp", r)
}
