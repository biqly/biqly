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

// AIRouter sets up only the routes owned by the AI Service.
func AIRouter(deps *app.Dependencies) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(requestIDPropagationMiddleware)
	r.Use(traceContextPropagationMiddleware)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(aiServiceRequestTimeout(deps)))
	r.Use(bimw.Locale)
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
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
		"query":   deps.Config.Services.QueryURL,
	}))
	r.Get("/metrics", MetricsHandler)

	r.Route("/api", func(r chi.Router) {
		registerAIAPIRoutes(r, deps)
	})

	r.Route("/internal", func(r chi.Router) {
		r.Use(handlers.InternalAuditMiddleware(deps.AuditLogger))
		r.Use(handlers.InternalTokenMiddleware(deps.Config.Security.InternalAPIToken))

		internalHandler := handlers.NewInternalHandlerWithService(deps, "biqly-ai")
		r.Get("/health", internalHandler.Health)
	})

	return r
}

func aiServiceRequestTimeout(deps *app.Dependencies) time.Duration {
	return deps.Config.AI.AIRequestTimeout()
}

func registerAIAPIRoutes(r chi.Router, deps *app.Dependencies) {
	aiHandler := handlers.NewAIHandler(deps)
	aiHandler.SetAIMetricsRecorder(GetMetrics())
	if deps.Jobs.Enabled && deps.AIJobsHTTP != nil {
		r.Post("/ai/jobs", deps.AIJobsHTTP.Create)
		r.Get("/ai/jobs", deps.AIJobsHTTP.List)
		r.Get("/ai/jobs/{id}", deps.AIJobsHTTP.Get)
	}
	r.Post("/ai/query", aiHandler.Query)
	r.Post("/ai/query/preview", aiHandler.Preview)
	r.Post("/ai/query/run", aiHandler.Run)
	r.Post("/ai/metadata/describe", aiHandler.Describe)
	r.Post("/ai/metadata/embed", aiHandler.EmbedMetadata)
	r.Get("/ai/settings", aiHandler.RuntimeSettings)
	r.Group(func(r chi.Router) {
		r.Use(handlers.AdminKeyMiddleware(deps.Config.Security.AdminAPIKey))
		r.Post("/ai/eval/run", aiHandler.EvalRun)
		r.Get("/ai/eval/run/stream", aiHandler.EvalRunStream)
		r.Get("/ai/eval/runs", aiHandler.EvalListRuns)
		r.Get("/ai/eval/runs/{id}", aiHandler.EvalGetRun)
		r.Get("/ai/eval/regression", aiHandler.EvalRegression)
	})

	examplesHandler := handlers.NewAIExamplesHandler(deps)
	r.Get("/ai/examples", examplesHandler.ListExamples)
	r.Post("/ai/examples", examplesHandler.CreateExample)
	r.Put("/ai/examples/{id}", examplesHandler.UpdateExample)
	r.Delete("/ai/examples/{id}", examplesHandler.DeleteExample)
	r.Post("/ai/feedback", examplesHandler.SubmitFeedback)
	r.Get("/ai/usage", examplesHandler.GetAIUsage)
	r.Get("/ai/example-ids", examplesHandler.GetExampleIDs)
	r.Get("/ai/stats/models", examplesHandler.GetModelSuccessRates)

	glossaryHandler := handlers.NewAIGlossaryHandler(deps)
	r.Get("/ai/glossary", glossaryHandler.ListGlossary)
	r.Post("/ai/glossary", glossaryHandler.CreateGlossary)
	r.Put("/ai/glossary/{id}", glossaryHandler.UpdateGlossary)
	r.Delete("/ai/glossary/{id}", glossaryHandler.DeleteGlossary)
}
