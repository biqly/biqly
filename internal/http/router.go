package http

import (
	"net/http"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/http/handlers"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
)

// Router sets up all HTTP routes.
func Router(deps *app.Dependencies) http.Handler {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	// AI NL→SQL and catalog embedding can be slow with local models.
	r.Use(middleware.Timeout(deps.Config.AI.AIRequestTimeout()))

	// Resolve user locale from Accept-Language / X-Locale / ?lang= and store on context.
	r.Use(bimw.Locale)

	// CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Accept-Language", "Authorization", "Content-Type", "X-CSRF-Token", "X-Locale"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	// Health check
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// Metrics
	r.Get("/metrics", MetricsHandler)

	// API routes
	r.Route("/api", func(r chi.Router) {
		// Datasource routes
		dsHandler := handlers.NewDatasourceHandler(deps)
		r.Post("/datasources", dsHandler.Create)
		r.Get("/datasources", dsHandler.List)
		r.Post("/datasources/test-connection", dsHandler.TestDraft)
		r.Get("/datasources/{id}", dsHandler.Get)
		r.Put("/datasources/{id}", dsHandler.Update)
		r.Delete("/datasources/{id}", dsHandler.Delete)
		r.Post("/datasources/{id}/test", dsHandler.Test)
		r.Post("/datasources/{id}/sync-metadata", dsHandler.SyncMetadata)

		// Semantic layer routes
		semHandler := handlers.NewSemanticHandler(deps)
		r.Post("/semantic/models", semHandler.CreateModel)
		r.Post("/semantic/models/generate", semHandler.GenerateModel)
		r.Get("/semantic/models", semHandler.ListModels)
		r.Get("/semantic/models/{id}", semHandler.GetModel)
		r.Put("/semantic/models/{id}", semHandler.UpdateModel)
		r.Delete("/semantic/models/{id}", semHandler.DeleteModel)
		r.Post("/semantic/models/{id}/validate", semHandler.ValidateModel)
		r.Post("/semantic/models/{id}/publish", semHandler.PublishModel)
		r.Post("/semantic/models/{id}/rollback", semHandler.RollbackModel)
		r.Post("/semantic/models/{id}/dimensions", semHandler.CreateDimension)
		r.Delete("/semantic/models/{id}/dimensions/{dimension_id}", semHandler.DeleteDimension)
		r.Put("/semantic/models/{id}/dimensions/{dimension_id}", semHandler.UpdateDimension)
		r.Post("/semantic/models/{id}/metrics", semHandler.CreateMetric)
		r.Delete("/semantic/models/{id}/metrics/{metric_id}", semHandler.DeleteMetric)
		r.Put("/semantic/models/{id}/metrics/{metric_id}", semHandler.UpdateMetric)
		r.Post("/semantic/models/{id}/tables/remove", semHandler.RemoveTable)
		r.Post("/semantic/models/{id}/joins", semHandler.CreateJoin)
		r.Delete("/semantic/models/{id}/joins/{join_id}", semHandler.DeleteJoin)
		r.Put("/semantic/models/{id}/joins/{join_id}", semHandler.UpdateJoin)
		r.Get("/semantic/models/{id}/suggested-joins", semHandler.SuggestedJoins)

		// Query routes
		queryHandler := handlers.NewQueryHandler(deps)
		r.Post("/query/compile", queryHandler.Compile)
		r.Post("/query/run", queryHandler.Run)
		r.Post("/query/explain", queryHandler.Explain)
		r.Get("/query/history", queryHandler.History)
		r.Get("/query/history/{id}", queryHandler.GetHistory)

		// Metadata routes
		metaHandler := handlers.NewMetadataHandler(deps)
		r.Get("/datasources/{id}/tables", metaHandler.ListTables)
		r.Get("/datasources/{id}/columns", metaHandler.ListColumns)
		r.Get("/metadata/columns/search", metaHandler.SearchColumns)
		r.Get("/metadata/tables/search", metaHandler.SearchTables)
		r.Patch("/metadata/tables/{id}", metaHandler.UpdateTableDescription)
		r.Patch("/metadata/columns/{id}", metaHandler.UpdateColumnDescription)
		r.Get("/metadata/tables/{id}/translations", metaHandler.GetTableTranslations)
		r.Put("/metadata/tables/{id}/translations", metaHandler.PutTableTranslations)
		r.Get("/metadata/columns/{id}/translations", metaHandler.GetColumnTranslations)
		r.Put("/metadata/columns/{id}/translations", metaHandler.PutColumnTranslations)

		// AI routes
		aiHandler := handlers.NewAIHandler(deps)
		aiHandler.SetAIMetricsRecorder(GetMetrics())
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

		// AI examples & feedback routes
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
	})

	return r
}
