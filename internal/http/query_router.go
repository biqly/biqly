package http

import (
	"net/http"
	"time"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/audit"
	"github.com/biqly/biqly/internal/http/handlers"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/go-chi/chi/v5"
)

// QueryRouter sets up only the routes owned by the Query Engine.
func QueryRouter(deps *app.Dependencies) http.Handler {
	r := chi.NewRouter()

	ApplyBaseMiddleware(r, BaseMiddlewareConfig{
		Metrics: GetMetrics(),
		Timeout: deps.Config.MaxQueryRuntime() + 10*time.Second,
		Locale:  true,
		CORS:    serviceCORS(deps),
		SecurityHeaders: bimw.SecurityHeadersConfig{
			HSTSEnabled:           deps.Config.HTTP.HSTSEnabled,
			HSTSIncludeSubdomains: true,
			ContentSecurityPolicy: "default-src 'self'; frame-ancestors 'none'",
		},
	})

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(healthCheckBody)
	})
	r.Get("/ready", ReadinessHandler(deps, map[string]string{
		"catalog": deps.Config.Services.CatalogURL,
	}))
	r.Get("/metrics", MetricsHandler)

	authMW := buildAPIAuthMiddleware(deps)
	authClient := NewAuthClient(deps)
	r.Route("/api", func(r chi.Router) {
		r.Use(authMW)
		r.Use(bimw.ChannelTag())
		r.Use(bimw.RequirePermission(authClient, "query:execute"))
		registerQueryAPIRoutes(r, deps.QueryDeps(), authClient, bimw.RequireDatasourceAccess(authClient, "read"))
	})

	r.Route("/internal", func(r chi.Router) {
		r.Use(handlers.InternalAuditMiddleware(deps.AuditLogger))
		r.Use(handlers.InternalTokenMiddleware(deps.Config.Security.InternalAPIToken))
		r.Use(bimw.ChannelStatic(audit.ChannelInternal))
		registerQueryInternalRoutes(r, deps.QueryDeps())
	})

	return OTELHTTPHandler("biqly-query", r)
}

func registerQueryAPIRoutes(r chi.Router, deps *app.QueryDeps, authClient *bimw.AuthClient, dsAccess func(http.Handler) http.Handler) {
	queryHandler := handlers.NewQueryHandler(deps)
	queryHandler.SetQueryMetricsRecorder(GetMetrics())
	// compile/run/explain carry a client-supplied datasource_id in the body and
	// must be gated on per-datasource access, not just the generic query:execute
	// permission — otherwise any user could target another tenant's datasource.
	r.With(dsAccess).Post("/query/compile", queryHandler.Compile)
	r.With(dsAccess).Post("/query/run", queryHandler.Run)
	r.With(dsAccess).Post("/query/explain", queryHandler.Explain)
	r.Get("/query/history", queryHandler.History)
	r.Get("/query/history/{id}", queryHandler.GetHistory)

	// Query-audit prove-ability endpoints: admin-only view of which policy
	// decisions were applied to which executed query.
	auditHandler := handlers.NewAuditQueryHandler(deps)
	r.With(bimw.RequirePermission(authClient, "admin:audit")).Get("/audit/query", auditHandler.List)
	r.With(bimw.RequirePermission(authClient, "admin:audit")).Get("/audit/query/{id}", auditHandler.Detail)
}

func registerQueryInternalRoutes(r chi.Router, deps *app.QueryDeps) {
	internalQueryHandler := handlers.NewInternalQueryHandler(deps)
	internalQueryHandler.SetQueryMetricsRecorder(GetMetrics())
	r.Post("/query/compile", internalQueryHandler.Compile)
	r.Post("/query/run", internalQueryHandler.Run)
	r.Post("/query/dry-run", internalQueryHandler.DryRun)
}
