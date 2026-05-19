package http

import (
	"net/http"
	"time"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/http/handlers"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// QueryRouter sets up only the routes owned by the Query Engine.
func QueryRouter(deps *app.Dependencies) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(requestIDPropagationMiddleware)
	r.Use(traceContextPropagationMiddleware)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(deps.Config.MaxQueryRuntime() + 10*time.Second))
	r.Use(bimw.Locale)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Accept-Language", "Authorization", "Content-Type", "X-CSRF-Token", "X-Locale"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(healthCheckBody)
	})
	r.Get("/ready", ReadinessHandler(deps, map[string]string{
		"catalog": deps.Config.Services.CatalogURL,
	}))
	r.Get("/metrics", MetricsHandler)

	r.Route("/api", func(r chi.Router) {
		registerQueryAPIRoutes(r, deps)
	})

	r.Route("/internal", func(r chi.Router) {
		r.Use(handlers.InternalAuditMiddleware(deps.AuditLogger))
		r.Use(handlers.InternalTokenMiddleware(deps.Config.Security.InternalAPIToken))
		registerQueryInternalRoutes(r, deps)
	})

	return r
}

func registerQueryAPIRoutes(r chi.Router, deps *app.Dependencies) {
	queryHandler := handlers.NewQueryHandler(deps)
	queryHandler.SetQueryMetricsRecorder(GetMetrics())
	r.Post("/query/compile", queryHandler.Compile)
	r.Post("/query/run", queryHandler.Run)
	r.Post("/query/explain", queryHandler.Explain)
	r.Get("/query/history", queryHandler.History)
	r.Get("/query/history/{id}", queryHandler.GetHistory)
}

func registerQueryInternalRoutes(r chi.Router, deps *app.Dependencies) {
	internalQueryHandler := handlers.NewInternalQueryHandler(deps)
	internalQueryHandler.SetQueryMetricsRecorder(GetMetrics())
	r.Post("/query/compile", internalQueryHandler.Compile)
	r.Post("/query/run", internalQueryHandler.Run)
	r.Post("/query/dry-run", internalQueryHandler.DryRun)
}
