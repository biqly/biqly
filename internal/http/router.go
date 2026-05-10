package http

import (
	"net/http"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/http/handlers"
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

	// CORS
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://*", "http://*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
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
		r.Get("/datasources/{id}", dsHandler.Get)
		r.Delete("/datasources/{id}", dsHandler.Delete)
		r.Post("/datasources/{id}/test", dsHandler.Test)
		r.Post("/datasources/{id}/sync-metadata", dsHandler.SyncMetadata)

		// Semantic layer routes
		semHandler := handlers.NewSemanticHandler(deps)
		r.Post("/semantic/models", semHandler.CreateModel)
		r.Get("/semantic/models", semHandler.ListModels)
		r.Get("/semantic/models/{id}", semHandler.GetModel)
		r.Put("/semantic/models/{id}", semHandler.UpdateModel)
		r.Delete("/semantic/models/{id}", semHandler.DeleteModel)
		r.Post("/semantic/models/{id}/dimensions", semHandler.CreateDimension)
		r.Post("/semantic/models/{id}/metrics", semHandler.CreateMetric)
		r.Post("/semantic/models/{id}/joins", semHandler.CreateJoin)

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

		// AI routes
		aiHandler := handlers.NewAIHandler(deps)
		r.Post("/ai/query", aiHandler.Query)
		r.Post("/ai/query/preview", aiHandler.Preview)
		r.Post("/ai/query/run", aiHandler.Run)
		r.Post("/ai/metadata/describe", aiHandler.Describe)
		r.Post("/ai/metadata/embed", aiHandler.EmbedMetadata)
		r.Get("/ai/settings", aiHandler.RuntimeSettings)
		r.Post("/ai/eval/run", aiHandler.EvalRun)
		r.Get("/ai/eval/run/stream", aiHandler.EvalRunStream)

		// AI examples & feedback routes
		examplesHandler := handlers.NewAIExamplesHandler(deps)
		r.Get("/ai/examples", examplesHandler.ListExamples)
		r.Post("/ai/examples", examplesHandler.CreateExample)
		r.Put("/ai/examples/{id}", examplesHandler.UpdateExample)
		r.Delete("/ai/examples/{id}", examplesHandler.DeleteExample)
		r.Post("/ai/feedback", examplesHandler.SubmitFeedback)
		r.Get("/ai/usage", examplesHandler.GetAIUsage)
		r.Get("/ai/example-ids", examplesHandler.GetExampleIDs)
	})

	return r
}
