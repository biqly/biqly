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
		// AI microservice trusts the network: JWT verification, permission, and
		// datasource access checks all happen at the monolith proxy. Pass a nil
		// AuthClient so the per-route middlewares degrade to pass-through.
		registerAIAPIRoutes(r, deps.AIDeps(), nil)
	})

	r.Route("/internal", func(r chi.Router) {
		r.Use(handlers.InternalAuditMiddleware(deps.AuditLogger))
		r.Use(handlers.InternalTokenMiddleware(deps.Config.Security.InternalAPIToken))

		internalHandler := handlers.NewInternalHandlerWithService(deps.CatalogDeps(), "biqly-ai")
		r.Get("/health", internalHandler.Health)
	})

	return r
}

func aiServiceRequestTimeout(deps *app.Dependencies) time.Duration {
	return deps.Config.AI.AIRequestTimeout()
}

// registerAIAPIRoutes mounts the AI API routes. When authClient is non-nil,
// datasource-consuming routes (query/preview/run, describe, embed, job submit)
// are wrapped with RequireDatasourceAccess("read") so users cannot target
// datasources they have not been granted. Pass nil from the AI microservice,
// which is fronted by the monolith proxy that enforces the same checks.
func registerAIAPIRoutes(r chi.Router, deps *app.AIDeps, authClient *bimw.AuthClient) {
	aiHandler := handlers.NewAIHandler(deps)
	aiHandler.SetAIMetricsRecorder(GetMetrics())
	dsAccess := bimw.RequireDatasourceAccess(authClient, "read")
	if deps.Jobs.Enabled && deps.AIJobsHTTP != nil {
		r.With(dsAccess).Post("/ai/jobs", deps.AIJobsHTTP.Create)
		r.Get("/ai/jobs/describe-batch/conflict", deps.AIJobsHTTP.DescribeBatchConflict)
		r.Get("/ai/jobs", deps.AIJobsHTTP.List)
		r.Get("/ai/jobs/queue/status", deps.AIJobsHTTP.QueueStatus)
		r.Get("/ai/jobs/stale", deps.AIJobsHTTP.ListStale)
		r.Post("/ai/jobs/cancel-active", deps.AIJobsHTTP.CancelActive)
		r.Post("/ai/jobs/cancel-batch", deps.AIJobsHTTP.CancelBatch)
		r.Get("/ai/jobs/{id}", deps.AIJobsHTTP.Get)
		r.Delete("/ai/jobs/{id}", deps.AIJobsHTTP.Cancel)
	}
	r.Get("/ai/history", aiHandler.AIHistory)
	r.Get("/ai/history/detail", aiHandler.AIHistoryDetail)
	r.Get("/ai/query/history", aiHandler.QueryHistory)
	r.With(dsAccess).Post("/ai/query", aiHandler.Query)
	r.With(dsAccess).Post("/ai/query/preview", aiHandler.Preview)
	r.With(dsAccess).Post("/ai/query/run", aiHandler.Run)
	r.With(dsAccess).Post("/ai/metadata/describe", aiHandler.Describe)
	r.With(dsAccess).Post("/ai/metadata/embed", aiHandler.EmbedMetadata)
	r.Get("/ai/settings", aiHandler.RuntimeSettings)
	r.Group(func(r chi.Router) {
		r.Use(handlers.AdminKeyMiddleware(deps.Config.Security.AdminAPIKey))
		if deps.Jobs.Enabled && deps.AIJobsHTTP != nil {
			r.Get("/ai/jobs/admin/stale", deps.AIJobsHTTP.AdminListStale)
			r.Post("/ai/jobs/admin/cancel-all-stale", deps.AIJobsHTTP.AdminCancelAllStale)
			r.Post("/ai/jobs/admin/cancel-batch", deps.AIJobsHTTP.CancelBatch)
		}
		r.Post("/ai/eval/run", aiHandler.EvalRun)
		r.Get("/ai/eval/run/stream", aiHandler.EvalRunStream)
		r.Get("/ai/eval/runs", aiHandler.EvalListRuns)
		r.Get("/ai/eval/runs/{id}", aiHandler.EvalGetRun)
		r.Get("/ai/eval/regression", aiHandler.EvalRegression)

		// AI provider/model runtime management (DB-managed, hot-reloaded).
		providersHandler := handlers.NewAIProvidersHandler(deps)
		r.Get("/ai/providers", providersHandler.ListProviders)
		r.Post("/ai/providers", providersHandler.CreateProvider)
		r.Get("/ai/providers/active-models", providersHandler.ActiveModels)
		r.Get("/ai/providers/{id}", providersHandler.GetProvider)
		r.Put("/ai/providers/{id}", providersHandler.UpdateProvider)
		r.Delete("/ai/providers/{id}", providersHandler.DeleteProvider)
		r.Post("/ai/providers/{id}/test", providersHandler.TestProvider)
		r.Get("/ai/models", providersHandler.ListModels)
		r.Post("/ai/models", providersHandler.CreateModel)
		r.Put("/ai/models/{id}", providersHandler.UpdateModel)
		r.Delete("/ai/models/{id}", providersHandler.DeleteModel)
		r.Post("/ai/models/{id}/default", providersHandler.SetDefaultModel)
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

	promptTemplatesHandler := handlers.NewAIPromptTemplatesHandler(deps)
	r.Get("/ai/prompt-templates", promptTemplatesHandler.ListPromptTemplates)
	r.Put("/ai/prompt-templates/{name}/{locale}", promptTemplatesHandler.UpdatePromptTemplate)
	r.Post("/ai/prompt-templates/restore", promptTemplatesHandler.RestorePromptTemplate)
	r.Post("/ai/prompt-templates/reseed", promptTemplatesHandler.ReseedPromptTemplates)

	timeGrainsHandler := handlers.NewAITimeGrainsHandler(deps)
	r.Get("/ai/settings/time-grains", timeGrainsHandler.ListTimeGrains)
	r.Put("/ai/settings/time-grains/{grain}", timeGrainsHandler.UpdateTimeGrain)
}
