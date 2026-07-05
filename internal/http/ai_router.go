package http

import (
	"context"
	"net/http"
	"time"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/http/handlers"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/go-chi/chi/v5"
)

// AIRouter sets up only the routes owned by the AI Service.
func AIRouter(deps *app.Dependencies) http.Handler {
	r := chi.NewRouter()

	ApplyBaseMiddleware(r, BaseMiddlewareConfig{
		Metrics: GetMetrics(),
		Timeout: aiServiceRequestTimeout(deps),
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
	aiHistoryPagination := bimw.Paginate(bimw.PaginationConfig{DefaultPage: 1, DefaultPageSize: 10, MaxPageSize: 100})
	queryHistoryPagination := bimw.Paginate(bimw.PaginationConfig{DefaultPage: 1, DefaultPageSize: 10, MaxPageSize: 50})
	staleJobsPagination := bimw.Paginate(bimw.PaginationConfig{DefaultPage: 1, DefaultPageSize: 100, MaxPageSize: 500})
	adminJobsPagination := bimw.Paginate(bimw.PaginationConfig{DefaultPage: 1, DefaultPageSize: 25, MaxPageSize: 100})
	usageBreakdownPagination := bimw.Paginate(bimw.PaginationConfig{DefaultPage: 1, DefaultPageSize: 25, MaxPageSize: 100})
	confirmedQueriesPagination := bimw.Paginate(bimw.PaginationConfig{DefaultPage: 1, DefaultPageSize: 10, MaxPageSize: 100})
	if deps.Jobs.Enabled && deps.AIJobsHTTP != nil {
		r.With(dsAccess).Post("/ai/jobs", deps.AIJobsHTTP.Create)
		r.Get("/ai/jobs/describe-batch/conflict", deps.AIJobsHTTP.DescribeBatchConflict)
		r.Get("/ai/jobs", deps.AIJobsHTTP.List)
		r.Get("/ai/jobs/queue/status", deps.AIJobsHTTP.QueueStatus)
		r.With(staleJobsPagination).Get("/ai/jobs/stale", deps.AIJobsHTTP.ListStale)
		r.Post("/ai/jobs/cancel-active", deps.AIJobsHTTP.CancelActive)
		r.Post("/ai/jobs/cancel-batch", deps.AIJobsHTTP.CancelBatch)
		r.Get("/ai/jobs/{id}", deps.AIJobsHTTP.Get)
		r.Delete("/ai/jobs/{id}", deps.AIJobsHTTP.Cancel)
	}
	r.With(aiHistoryPagination).Get("/ai/history", aiHandler.AIHistory)
	r.Get("/ai/history/detail", aiHandler.AIHistoryDetail)
	// Replay resolves the history entry to its owning datasource before the
	// access check (the {id} is a history row, not a datasource). Enforced
	// unconditionally like the glossary routes: the monolith proxy cannot
	// resolve history ids, so the check must live here.
	historyDS := func(ctx context.Context, id string) (string, error) {
		row, err := deps.MetaRepo.GetAIQueryHistoryByID(ctx, id)
		if err != nil {
			return "", err
		}
		return row.DatasourceID, nil
	}
	r.With(aiUserMW, bimw.RequireResolvedDatasourceAccess(authClient, "read", historyDS)).Post("/ai/history/{id}/replay", aiHandler.ReplayAIHistory)
	r.With(queryHistoryPagination).Get("/ai/query/history", aiHandler.QueryHistory)
	registerAIConversationRoutes(r, aiHandler, aiUserMW, dsAccess)
	r.With(aiUserMW, dsAccess).Post("/ai/query", aiHandler.Query)
	r.With(aiUserMW, dsAccess).Post("/ai/query/preview", aiHandler.Preview)
	r.With(aiUserMW, dsAccess).Post("/ai/query/run", aiHandler.Run)
	r.With(aiUserMW, dsAccess).Post("/ai/metadata/describe", aiHandler.Describe)
	r.With(aiUserMW, dsAccess).Post("/ai/metadata/embed", aiHandler.EmbedMetadata)
	// Backfills locale translations for a semantic model's label/description and
	// those of its dimensions/metrics into entity_translations (LLM only when
	// missing); the catalog model-read overlays them. Authenticated via the
	// /api group's authMW; aiUserMW keeps it consistent with the AI-user route
	// family. No per-model datasource ACL — matches the unguarded GetModel read
	// posture, and the endpoint discloses no model data (returns only a count)
	// while writes are an idempotent, bounded TR rendering of the model's own text.
	r.With(aiUserMW).Post("/ai/semantic/models/{id}/translate", aiHandler.TranslateSemanticModel)
	r.Get("/ai/settings", aiHandler.RuntimeSettings)
	r.Get("/ai/user-models", aiHandler.UserAIModels)
	r.Put("/ai/user-preferences", aiHandler.PutUserAIPreferences)
	r.Delete("/ai/user-preferences/{purpose}", aiHandler.DeleteUserAIPreference)
	r.Group(func(r chi.Router) {
		// Admin endpoints accept super_admin JWTs, the shared admin key
		// (machine-to-machine), or JWTs whose RBAC grants ai:settings.
		r.Use(handlers.AdminAccessMiddleware(deps.Config.Security.AdminAPIKey, authClient, "ai:settings"))
		if deps.Jobs.Enabled && deps.AIJobsHTTP != nil {
			r.With(adminJobsPagination).Get("/ai/jobs/admin", deps.AIJobsHTTP.AdminList)
			r.With(adminJobsPagination).Get("/ai/jobs/admin/stale", deps.AIJobsHTTP.AdminListStale)
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

		registerAIAdminConfigRoutes(r, aiHandler, confirmedQueriesPagination)
	})

	registerAIExamplesGlossaryAndTemplatesRoutes(r, deps, authClient, usageBreakdownPagination)
	registerAISkillsRoutes(r, deps, authClient)
}

// registerAISkillsRoutes wires the skills library: saved parameterized
// LogicalQuery templates re-run through the governed query path. Mutating and
// run routes resolve the skill {id} to its owning datasource before the
// access check, like the glossary and replay routes: the monolith proxy
// cannot resolve skill ids, so the check must live here.
func registerAISkillsRoutes(r chi.Router, deps *app.AIDeps, authClient *bimw.AuthClient) {
	skillsHandler := handlers.NewAISkillsHandler(deps)
	skillDS := func(ctx context.Context, id string) (string, error) {
		return deps.MetaRepo.DatasourceForSavedQuery(ctx, id)
	}
	skillAccess := bimw.RequireResolvedDatasourceAccess(authClient, "read", skillDS)
	aiUserMW := bimw.InjectAIUserContext
	r.Get("/ai/skills", skillsHandler.List)
	r.With(bimw.RequireDatasourceAccess(authClient, "read")).Post("/ai/skills", skillsHandler.Create)
	r.With(skillAccess).Get("/ai/skills/{id}", skillsHandler.Get)
	r.With(skillAccess).Put("/ai/skills/{id}", skillsHandler.Update)
	r.With(skillAccess).Delete("/ai/skills/{id}", skillsHandler.Delete)
	r.With(aiUserMW, skillAccess).Post("/ai/skills/{id}/run", skillsHandler.Run)

	// Report schedules run skills unattended and mail results to arbitrary
	// recipients — admin surface only.
	reportsHandler := handlers.NewReportSchedulesHandler(deps)
	requireReportAdmin := bimw.RequirePermission(authClient, "ai:settings")
	r.With(requireReportAdmin).Get("/ai/reports/schedules", reportsHandler.List)
	r.With(requireReportAdmin).Post("/ai/reports/schedules", reportsHandler.Create)
	r.With(requireReportAdmin).Get("/ai/reports/schedules/{id}", reportsHandler.Get)
	r.With(requireReportAdmin).Put("/ai/reports/schedules/{id}", reportsHandler.Update)
	r.With(requireReportAdmin).Delete("/ai/reports/schedules/{id}", reportsHandler.Delete)

	// Memory entries are owner-scoped (workspace + user from JWT); no admin
	// permission — every user manages only their own remembered facts.
	memoryHandler := handlers.NewMemoryEntriesHandler(deps)
	r.Get("/ai/memory/entries", memoryHandler.List)
	r.Post("/ai/memory/entries", memoryHandler.Create)
	r.Put("/ai/memory/entries/{id}", memoryHandler.Update)
	r.Delete("/ai/memory/entries/{id}", memoryHandler.Delete)
}

func registerAIExamplesGlossaryAndTemplatesRoutes(
	r chi.Router,
	deps *app.AIDeps,
	authClient *bimw.AuthClient,
	usageBreakdownPagination func(http.Handler) http.Handler,
) {
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
	r.With(usageBreakdownPagination).Get("/ai/usage/breakdown", examplesHandler.GetAIUsageBreakdown)
	r.Get("/ai/example-ids", examplesHandler.GetExampleIDs)
	r.Get("/ai/stats/models", examplesHandler.GetModelSuccessRates)

	glossaryHandler := handlers.NewAIGlossaryHandler(deps)
	// Glossary terms are datasource-scoped content. Create carries datasource_id
	// in the body; update/delete carry only the term {id}, so resolve it to the
	// owning datasource and check access — otherwise any ai:query user could edit
	// another tenant's glossary.
	glossaryDS := func(ctx context.Context, id string) (string, error) {
		return deps.MetaRepo.DatasourceForBusinessGlossary(ctx, id)
	}
	r.Get("/ai/glossary", glossaryHandler.ListGlossary)
	r.With(bimw.RequireDatasourceAccess(authClient, "write")).Post("/ai/glossary", glossaryHandler.CreateGlossary)
	r.With(bimw.RequireResolvedDatasourceAccess(authClient, "write", glossaryDS)).Put("/ai/glossary/{id}", glossaryHandler.UpdateGlossary)
	r.With(bimw.RequireResolvedDatasourceAccess(authClient, "write", glossaryDS)).Delete("/ai/glossary/{id}", glossaryHandler.DeleteGlossary)

	// Free-form business rules ("instructions") are datasource-scoped content
	// injected into the prompt as a "## Business Rules" block. Create carries
	// datasource_id in the body; update/delete carry only the instruction {id},
	// so resolve it to the owning datasource and check write access — mirroring
	// the glossary routes above.
	instructionsHandler := handlers.NewAIInstructionsHandler(deps)
	instructionDS := func(ctx context.Context, id string) (string, error) {
		return deps.MetaRepo.DatasourceForInstruction(ctx, id)
	}
	r.Get("/ai/instructions", instructionsHandler.List)
	r.With(bimw.RequireDatasourceAccess(authClient, "write")).Post("/ai/instructions", instructionsHandler.Create)
	r.With(bimw.RequireResolvedDatasourceAccess(authClient, "write", instructionDS)).Put("/ai/instructions/{id}", instructionsHandler.Update)
	r.With(bimw.RequireResolvedDatasourceAccess(authClient, "write", instructionDS)).Delete("/ai/instructions/{id}", instructionsHandler.Delete)

	// System prompt templates and time grains steer text-to-SQL for every
	// user, and reseed/restore are destructive — admin surface only.
	requireAISettings := bimw.RequirePermission(authClient, "ai:settings")

	promptTemplatesHandler := handlers.NewAIPromptTemplatesHandler(deps)
	r.With(requireAISettings).Get("/ai/prompt-templates", promptTemplatesHandler.ListPromptTemplates)
	r.With(requireAISettings).Put("/ai/prompt-templates/{name}/{locale}", promptTemplatesHandler.UpdatePromptTemplate)
	r.With(requireAISettings).Post("/ai/prompt-templates/restore", promptTemplatesHandler.RestorePromptTemplate)
	r.With(requireAISettings).Post("/ai/prompt-templates/reseed", promptTemplatesHandler.ReseedPromptTemplates)

	timeGrainsHandler := handlers.NewAITimeGrainsHandler(deps)
	r.With(requireAISettings).Get("/ai/settings/time-grains", timeGrainsHandler.ListTimeGrains)
	r.With(requireAISettings).Put("/ai/settings/time-grains/{grain}", timeGrainsHandler.UpdateTimeGrain)
}

func registerAIConversationRoutes(
	r chi.Router,
	aiHandler *handlers.AIHandler,
	aiUserMW func(http.Handler) http.Handler,
	dsAccess func(http.Handler) http.Handler,
) {
	r.With(aiUserMW).Get("/ai/conversations", aiHandler.ListConversations)
	r.With(aiUserMW, dsAccess).Post("/ai/conversations", aiHandler.CreateConversation)
	r.With(aiUserMW).Delete("/ai/conversations/{id}", aiHandler.DeleteConversation)
}

// registerAIAdminConfigRoutes wires the admin-key-protected runtime knobs:
// NL→SQL memory store administration (P3), runtime config (P5), and the
// locale-dimensioned NL lexicon (ADR-0001, DİL-1).
func registerAIAdminConfigRoutes(
	r chi.Router,
	aiHandler *handlers.AIHandler,
	confirmedQueriesPagination func(http.Handler) http.Handler,
) {
	r.With(confirmedQueriesPagination).Get("/ai/confirmed-queries", aiHandler.AdminListConfirmedQueries)
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
