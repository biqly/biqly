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

// AIRouter sets up only the routes owned by the AI Service.
func AIRouter(deps *app.Dependencies) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(requestIDPropagationMiddleware)
	r.Use(bimw.RealIP)
	r.Use(requestLoggerMiddleware)
	r.Use(middleware.Recoverer)
	r.Use(HTTPMetricsMiddleware(GetMetrics()))
	r.Use(middleware.Timeout(aiServiceRequestTimeout(deps)))
	r.Use(bimw.Locale)
	r.Use(serviceCORS(deps))
	r.Use(serviceSecurityHeaders(deps))

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(healthCheckBody)
	})
	r.Get("/ready", ReadinessHandler(deps, map[string]string{
		"catalog": deps.Config.Services.CatalogURL,
		"query":   deps.Config.Services.QueryURL,
	}))
	r.Get("/metrics", MetricsHandler)

	authClient := buildAIAuthClient(deps)
	deps.WireAIUserResolver(authClient)
	authMW := buildAIAuthMiddleware(deps)

	r.Route("/api", func(r chi.Router) {
		r.Use(authMW)
		registerAIAPIRoutes(r, deps.AIDeps(), authClient, false)
	})

	r.Route("/internal", func(r chi.Router) {
		r.Use(handlers.InternalAuditMiddleware(deps.AuditLogger))
		r.Use(handlers.InternalTokenMiddleware(deps.Config.Security.InternalAPIToken))

		internalHandler := handlers.NewInternalHandlerWithService(deps.CatalogDeps(), "biqly-ai")
		r.Get("/health", internalHandler.Health)
	})

	return OTELHTTPHandler("biqly-ai", r)
}

func aiServiceRequestTimeout(deps *app.Dependencies) time.Duration {
	return deps.Config.AI.RequestTimeout()
}

// registerAIAPIRoutes mounts the AI API routes. When authClient is non-nil,
// datasource-consuming routes (query/preview/run, describe, embed, job submit)
// are wrapped with RequireDatasourceAccess("read") so users cannot target
// datasources they have not been granted. Pass nil from the AI microservice,
// which is fronted by the monolith proxy that enforces the same checks.
func registerAIAPIRoutes(r chi.Router, deps *app.AIDeps, authClient *bimw.AuthClient, enforceMiddlewares bool) {
	aiHandler := handlers.NewAIHandler(deps)
	aiHandler.SetAuthClient(authClient)
	aiHandler.SetAIMetricsRecorder(GetMetrics())
	var dsAccess func(http.Handler) http.Handler
	if enforceMiddlewares && authClient != nil {
		dsAccess = bimw.RequireDatasourceAccess(authClient, "read")
	} else {
		dsAccess = func(next http.Handler) http.Handler { return next }
	}
	aiUserMW := bimw.InjectAIUserContext
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
	r.With(aiUserMW, dsAccess).Post("/ai/query", aiHandler.Query)
	r.With(aiUserMW, dsAccess).Post("/ai/query/preview", aiHandler.Preview)
	r.With(aiUserMW, dsAccess).Post("/ai/query/run", aiHandler.Run)
	r.With(aiUserMW, dsAccess).Post("/ai/metadata/describe", aiHandler.Describe)
	r.With(aiUserMW, dsAccess).Post("/ai/metadata/embed", aiHandler.EmbedMetadata)
	r.Get("/ai/settings", aiHandler.RuntimeSettings)
	r.Get("/ai/user-models", aiHandler.UserAIModels)
	r.Put("/ai/user-preferences", aiHandler.PutUserAIPreferences)
	r.Delete("/ai/user-preferences/{purpose}", aiHandler.DeleteUserAIPreference)
	r.Group(func(r chi.Router) {
		// Admin endpoints accept super_admin JWTs, the shared admin key
		// (machine-to-machine), or JWTs whose RBAC grants ai:settings.
		r.Use(handlers.AdminAccessMiddleware(deps.Config.Security.AdminAPIKey, authClient, "ai:settings"))
		if deps.Jobs.Enabled && deps.AIJobsHTTP != nil {
			r.Get("/ai/jobs/admin", deps.AIJobsHTTP.AdminList)
			r.Get("/ai/jobs/admin/stale", deps.AIJobsHTTP.AdminListStale)
			r.Post("/ai/jobs/admin/cancel-all-stale", deps.AIJobsHTTP.AdminCancelAllStale)
			r.Post("/ai/jobs/admin/cancel-batch", deps.AIJobsHTTP.CancelBatch)
		}
		r.Post("/ai/eval/run", aiHandler.EvalRun)
		r.Get("/ai/eval/run/stream", aiHandler.EvalRunStream)
		r.Get("/ai/eval/runs", aiHandler.EvalListRuns)
		r.Get("/ai/eval/runs/{id}", aiHandler.EvalGetRun)
		r.Get("/ai/eval/regression", aiHandler.EvalRegression)
		r.Get("/ai/eval/cases", aiHandler.EvalListCases)
		r.Post("/ai/eval/cases", aiHandler.EvalCreateCase)
		r.Delete("/ai/eval/cases/{id}", aiHandler.EvalDeleteCase)

		// AI provider/model runtime management (DB-managed, hot-reloaded).
		providersHandler := handlers.NewAIProvidersHandler(deps)
		r.Get("/ai/providers", providersHandler.ListProviders)
		r.Post("/ai/providers", providersHandler.CreateProvider)
		r.Get("/ai/providers/active-models", providersHandler.ActiveModels)
		r.Get("/ai/providers/{id}", providersHandler.GetProvider)
		r.Put("/ai/providers/{id}", providersHandler.UpdateProvider)
		r.Delete("/ai/providers/{id}", providersHandler.DeleteProvider)
		r.Post("/ai/providers/{id}/test", providersHandler.TestProvider)
		r.Get("/ai/providers/{id}/remote-models", providersHandler.ListProviderRemoteModels)
		r.Get("/ai/models", providersHandler.ListModels)
		r.Post("/ai/models", providersHandler.CreateModel)
		r.Put("/ai/models/{id}", providersHandler.UpdateModel)
		r.Delete("/ai/models/{id}", providersHandler.DeleteModel)
		r.Post("/ai/models/{id}/default", providersHandler.SetDefaultModel)

		// Prompt A/B testing management endpoints
		abHandler := handlers.NewABExperimentHandler(deps)
		r.Post("/ai/ab-experiments", abHandler.Create)
		r.Get("/ai/ab-experiments", abHandler.List)
		r.Get("/ai/ab-experiments/{id}", abHandler.Get)
		r.Put("/ai/ab-experiments/{id}", abHandler.Update)
		r.Put("/ai/ab-experiments/{id}/status", abHandler.UpdateStatus)
		r.Post("/ai/ab-experiments/{id}/variants", abHandler.AddVariant)
		r.Put("/ai/ab-experiments/{id}/variants/{variantId}", abHandler.UpdateVariant)
		r.Delete("/ai/ab-experiments/{id}/variants/{variantId}", abHandler.DeleteVariant)
		r.Get("/ai/ab-experiments/{id}/metrics", abHandler.GetMetrics)
		r.Get("/ai/ab-experiments/{id}/timeseries", abHandler.GetTimeseries)
		r.Get("/ai/ab-experiments/{id}/recommendation", abHandler.GetRecommendation)

		enrichHandler := handlers.NewEnrichContextHandler(deps)
		enrichHandler.SetAIMetricsRecorder(GetMetrics())
		r.Post("/ai/enrich-context", enrichHandler.Analyze)
		r.Post("/ai/enrich-context/apply", enrichHandler.Apply)

		registerAIAdminConfigRoutes(r, aiHandler)
	})

	examplesHandler := handlers.NewAIExamplesHandler(deps)
	examplesHandler.SetAuthClient(authClient)
	examplesHandler.SetAIMetricsRecorder(GetMetrics())
	r.Get("/ai/examples", examplesHandler.ListExamples)
	r.Get("/ai/examples/favorites", examplesHandler.ListFavorites)
	r.Post("/ai/examples", examplesHandler.CreateExample)
	r.Put("/ai/examples/{id}", examplesHandler.UpdateExample)
	r.Delete("/ai/examples/{id}", examplesHandler.DeleteExample)
	r.Post("/ai/feedback", examplesHandler.SubmitFeedback)
	r.Get("/ai/usage", examplesHandler.GetAIUsage)
	r.Get("/ai/usage/breakdown", examplesHandler.GetAIUsageBreakdown)
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

// registerAIAdminConfigRoutes wires the admin-key-protected runtime knobs:
// NL→SQL memory store administration (P3), runtime config (P5), and the
// locale-dimensioned NL lexicon (ADR-0001, DİL-1).
func registerAIAdminConfigRoutes(r chi.Router, aiHandler *handlers.AIHandler) {
	r.Get("/ai/confirmed-queries", aiHandler.AdminListConfirmedQueries)
	r.Post("/ai/confirmed-queries/{id}/deactivate", aiHandler.AdminDeactivateConfirmedQuery)
	r.Get("/ai/admin/config", aiHandler.AdminRuntimeConfig)
	r.Put("/ai/admin/config", aiHandler.UpdateAdminRuntimeConfig)
	r.Get("/ai/admin/lexicon", aiHandler.AdminListLexicon)
	r.Put("/ai/admin/lexicon", aiHandler.AdminUpsertLexicon)
	r.Post("/ai/admin/lexicon/reset", aiHandler.AdminResetLexiconDomain)
	// Dynamic locale registry + message-bundle overlay (ADR-0001 K8, DİL-3).
	r.Get("/ai/admin/i18n/locales", aiHandler.AdminListI18nLocales)
	r.Put("/ai/admin/i18n/locales", aiHandler.AdminUpsertI18nLocales)
	r.Get("/ai/admin/i18n/bundles/{locale}", aiHandler.AdminGetI18nBundle)
	r.Put("/ai/admin/i18n/bundles/{locale}", aiHandler.AdminUpsertI18nBundle)
	r.Get("/ai/admin/i18n/coverage/{locale}", aiHandler.AdminI18nCoverage)
}
