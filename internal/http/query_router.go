package http

import (
	"net/http"
	"time"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/http/handlers"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// QueryRouter sets up only the routes owned by the Query Engine.
func QueryRouter(deps *app.Dependencies) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(requestIDPropagationMiddleware)
	r.Use(TraceContextPropagationMiddleware)
	r.Use(bimw.RealIP)
	r.Use(requestLoggerMiddleware)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(deps.Config.MaxQueryRuntime() + 10*time.Second))
	r.Use(bimw.Locale)
	r.Use(serviceCORS(deps))
	r.Use(serviceSecurityHeaders(deps))

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
		r.Use(bimw.RequirePermission(authClient, "query:execute"))
		registerQueryAPIRoutes(r, deps.QueryDeps())
	})

	r.Route("/internal", func(r chi.Router) {
		r.Use(handlers.InternalAuditMiddleware(deps.AuditLogger))
		r.Use(handlers.InternalTokenMiddleware(deps.Config.Security.InternalAPIToken))
		registerQueryInternalRoutes(r, deps.QueryDeps())
	})

	return OTELHTTPHandler("biqly-query", r)
}

func registerQueryAPIRoutes(r chi.Router, deps *app.QueryDeps) {
	queryHandler := handlers.NewQueryHandler(deps)
	queryHandler.SetQueryMetricsRecorder(GetMetrics())
	r.Post("/query/compile", queryHandler.Compile)
	r.Post("/query/run", queryHandler.Run)
	r.Post("/query/explain", queryHandler.Explain)
	r.Get("/query/history", queryHandler.History)
	r.Get("/query/history/{id}", queryHandler.GetHistory)
}

func registerQueryInternalRoutes(r chi.Router, deps *app.QueryDeps) {
	internalQueryHandler := handlers.NewInternalQueryHandler(deps)
	internalQueryHandler.SetQueryMetricsRecorder(GetMetrics())
	r.Post("/query/compile", internalQueryHandler.Compile)
	r.Post("/query/run", internalQueryHandler.Run)
	r.Post("/query/dry-run", internalQueryHandler.DryRun)
}
