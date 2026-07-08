package http

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/audit"
	"github.com/biqly/biqly/internal/http/handlers"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// shortAPITimeout caps non-AI /api/* requests (CRUD, metadata, history list).
// Long-running AI generation lives under /api/ai/* with the dedicated AI
// timeout applied below.
const shortAPITimeout = 30 * time.Second

// healthCheckBody is the static JSON payload served by the /health endpoint;
// kept at package scope so the byte slice is allocated once at init.
var healthCheckBody = []byte(`{"status":"ok"}`)

// Router sets up all HTTP routes.
func Router(deps *app.Dependencies) http.Handler {
	r := setupRouter(deps)
	return OTELHTTPHandler("biqly-api", r)
}

// ChiRouter builds the same routes as Router but returns the raw chi.Router
// without the OTEL wrapper. Used by code-generation tools (cmd/gen-openapi)
// that need to Walk the route tree.
func ChiRouter(deps *app.Dependencies) chi.Router {
	return setupRouter(deps)
}

func setupRouter(deps *app.Dependencies) chi.Router {
	r := chi.NewRouter()

	// CORS — restrict to explicitly configured origins. Empty list = no
	// cross-origin requests; the legacy {"https://*", "http://*"} wildcard
	// was removed because it combined poorly with AllowCredentials=true.
	corsOrigins := deps.Config.HTTP.CORSAllowedOrigins
	if len(corsOrigins) == 0 {
		slog.Warn("CORS allowed origins is empty — cross-origin requests will be blocked. Set BI_CORS_ALLOWED_ORIGINS (comma-separated) to allow specific frontend domains.")
	}
	ApplyBaseMiddleware(r, BaseMiddlewareConfig{
		Metrics:   GetMetrics(),
		ChiLogger: true,
		Locale:    true,
		CORS: cors.Handler(cors.Options{
			AllowedOrigins:   corsOrigins,
			AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Accept-Language", "Authorization", "Content-Type", "X-API-Key", "X-Biqly-Channel", "X-CSRF-Token", "X-Locale"},
			ExposedHeaders:   []string{"Link"},
			AllowCredentials: true,
			MaxAge:           300,
		}),
		SecurityHeaders: bimw.SecurityHeadersConfig{
			HSTSEnabled:           deps.Config.HTTP.HSTSEnabled,
			HSTSIncludeSubdomains: true,
			ContentSecurityPolicy: "default-src 'self'; frame-ancestors 'none'",
		},
	})

	// Health check
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(healthCheckBody)
	})
	readyUpstreams := map[string]string{
		"catalog": deps.Config.Services.CatalogURL,
		"query":   deps.Config.Services.QueryURL,
		"ai":      deps.Config.Services.AIURL,
	}
	if deps.Config.Auth.Enabled {
		readyUpstreams["auth"] = deps.Config.Auth.ServiceURL
	}
	r.Get("/ready", ReadinessHandler(deps, readyUpstreams))

	// Metrics — optionally gated by BI_METRICS_API_KEY. The handler is
	// wrapped through the same APIKeyAuth middleware as /api/*, so scrapers
	// authenticate via `X-API-Key` or `Authorization: Bearer`.
	r.With(bimw.APIKeyAuth(deps.Config.Security.MetricsAPIKey)).Get("/metrics", MetricsHandler)

	// API routes
	authMW := buildAPIAuthMiddleware(deps)
	authClient := NewAuthClient(deps)
	deps.WireAIUserResolver(authClient)

	r.Route("/api", func(r chi.Router) {
		r.Use(authMW)
		r.Use(bimw.ChannelTag())

		// Default API timeout for CRUD / metadata / history endpoints. AI
		// sub-routes opt into the longer AIRequestTimeout below; query exec
		// routes manage their own context timeout in the executor layer.
		r.Group(func(r chi.Router) {
			r.Use(middleware.Timeout(shortAPITimeout))
			if deps.Config.Services.CatalogURL != "" {
				registerCatalogProxyRoutes(r, deps.Config.Services.CatalogURL)
			} else {
				r.Group(func(r chi.Router) {
					r.Use(CatalogMetricsMiddleware(GetMetrics()))
					registerCatalogAPIRoutes(r, deps.CatalogDeps(), authClient)
				})
			}

			if deps.Config.Services.QueryURL != "" {
				registerQueryProxyRoutes(r, deps.Config.Services.QueryURL)
			} else {
				r.With(bimw.RequirePermission(authClient, "query:execute")).Group(func(r chi.Router) {
					registerQueryAPIRoutes(r, deps.QueryDeps(), authClient, bimw.RequireDatasourceAccess(authClient, "read"))
				})
			}
		})

		// AI NL→SQL and catalog embedding can be slow with local models — they
		// need their own timeout budget. Routes are mounted in a separate
		// chi.Group so the short timeout above does not apply to them.
		//
		// Authorization is enforced here at the monolith edge in BOTH proxy and
		// in-process modes: ai:query permission, plus a datasource-access check
		// on the routes that carry a datasource_id (query/preview/run, metadata
		// describe/embed, job submit). The downstream AI service trusts the
		// network and does no JWT verification of its own.
		r.Group(func(r chi.Router) {
			r.Use(middleware.Timeout(deps.Config.AI.RequestTimeout()))
			r.Use(bimw.RequirePermission(authClient, "ai:query"))
			dsAccess := bimw.RequireDatasourceAccess(authClient, "read")
			if deps.Config.Services.AIURL != "" {
				registerAIProxyRoutesWithDatasourceGuard(r, deps.Config.Services.AIURL, dsAccess)
			} else {
				registerAIAPIRoutes(r, deps.AIDeps(), authClient, true)
			}
		})
	})

	// MCP server — governed programmatic access for external agents. Mounted
	// behind the same auth middleware as /api; every tool call loops back
	// through the /api router with the caller's credentials, so permissions,
	// datasource access, RLS/PII policy, spend caps and audit (channel=mcp)
	// apply exactly as they do for UI and API callers.
	r.With(authMW, middleware.Timeout(deps.Config.AI.RequestTimeout())).
		Handle("/mcp", mcpHandler(r))

	// Internal API routes (Phase 1 of microservice decomposition).
	// These endpoints are NOT part of the public API and MUST NOT be reached
	// from outside the cluster — they are the wire protocol the future AI /
	// Query Engine binaries will speak to the Catalog (today: this monolith).
	// In production they are fronted by a NetworkPolicy / Cilium policy that
	// only allows peer-service service accounts. See docs/microservice-decomposition.md.
	r.Route("/internal", func(r chi.Router) {
		r.Use(handlers.InternalAuditMiddleware(deps.AuditLogger))
		r.Use(handlers.InternalTokenMiddleware(deps.Config.Security.InternalAPIToken))
		r.Use(bimw.ChannelStatic(audit.ChannelInternal))

		registerCatalogInternalRoutes(r, deps.CatalogDeps(), "biqly-monolith")

		// Internal query endpoints — same compile/run pipeline as /api/query/*,
		// minus the user-facing concerns (auth, RBAC project scoping, etc.).
		// Today they delegate to the monolith's core.QueryService; in Phase 3
		// they move into the standalone Query Engine binary unchanged.
		registerQueryInternalRoutes(r, deps.QueryDeps())
	})

	return r
}
