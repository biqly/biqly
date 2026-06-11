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

// CatalogRouter sets up only the routes owned by the Catalog Service.
func CatalogRouter(deps *app.Dependencies) http.Handler {
	r := chi.NewRouter()

	r.Use(middleware.RequestID)
	r.Use(requestIDPropagationMiddleware)
	r.Use(bimw.RealIP)
	r.Use(requestLoggerMiddleware)
	r.Use(middleware.Recoverer)
	r.Use(HTTPMetricsMiddleware(GetMetrics()))
	r.Use(middleware.Timeout(60 * time.Second))
	r.Use(bimw.Locale)
	r.Use(serviceCORS(deps))
	r.Use(serviceSecurityHeaders(deps))

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(healthCheckBody)
	})
	r.Get("/ready", ReadinessHandler(deps, nil))
	r.Get("/metrics", MetricsHandler)

	// /api is the catalog's user-facing surface, reached directly through the
	// gateway — it must enforce auth itself (BI_AUTH_ENABLED=true → JWT with
	// admin-key bypass; otherwise legacy API key). The auth client additionally
	// wires per-datasource access checks on routes that carry a datasource_id.
	authMW := buildAPIAuthMiddleware(deps)
	authClient := NewAuthClient(deps)
	r.Route("/api", func(r chi.Router) {
		r.Use(authMW)
		r.Use(CatalogMetricsMiddleware(GetMetrics()))
		registerCatalogAPIRoutes(r, deps.CatalogDeps(), authClient)
	})

	r.Route("/internal", func(r chi.Router) {
		r.Use(handlers.InternalAuditMiddleware(deps.AuditLogger))
		r.Use(handlers.InternalTokenMiddleware(deps.Config.Security.InternalAPIToken))
		r.Use(CatalogMetricsMiddleware(GetMetrics()))
		registerCatalogInternalRoutes(r, deps.CatalogDeps(), "biqly-catalog")
	})

	return OTELHTTPHandler("biqly-catalog", r)
}

func registerCatalogAPIRoutes(r chi.Router, deps *app.CatalogDeps, authClient *bimw.AuthClient) {
	registerCatalogDatasourceRoutes(r, deps, authClient)
	registerCatalogSemanticRoutes(r, deps)
	registerCatalogCompositeRoutes(r, deps)
	registerCatalogMetadataRoutes(r, deps, authClient)
	registerCatalogPermissionRoutes(r, deps, authClient)
	registerCatalogDashboardRoutes(r, deps)
}

func registerCatalogDatasourceRoutes(r chi.Router, deps *app.CatalogDeps, authClient *bimw.AuthClient) {
	dsHandler := handlers.NewDatasourceHandler(deps)
	r.Post("/datasources", dsHandler.Create)
	r.Get("/datasources", dsHandler.List)
	r.Post("/datasources/test-connection", dsHandler.TestDraft)
	r.With(bimw.RequireDatasourceAccess(authClient, "read")).Get("/datasources/{id}", dsHandler.Get)
	r.With(bimw.RequireDatasourceAccess(authClient, "write")).Put("/datasources/{id}", dsHandler.Update)
	r.With(bimw.RequireDatasourceAccess(authClient, "admin")).Delete("/datasources/{id}", dsHandler.Delete)
	r.With(bimw.RequireDatasourceAccess(authClient, "read")).Post("/datasources/{id}/test", dsHandler.Test)
	r.With(bimw.RequireDatasourceAccess(authClient, "write")).Post("/datasources/{id}/sync-metadata", dsHandler.SyncMetadata)

	piiHandler := handlers.NewPIIHandler(deps)
	r.With(bimw.RequireDatasourceAccess(authClient, "write")).Post("/datasources/{id}/scan-pii", piiHandler.Scan)
	r.With(bimw.RequireDatasourceAccess(authClient, "read")).Get("/datasources/{id}/pii-columns", piiHandler.ListColumns)
}

func registerCatalogSemanticRoutes(r chi.Router, deps *app.CatalogDeps) {
	semHandler := handlers.NewSemanticHandler(deps)
	semHandler.SetCatalogMetricsRecorder(GetMetrics())
	r.Post("/semantic/models", semHandler.CreateModel)
	r.Post("/semantic/models/generate", semHandler.GenerateModel)
	r.Get("/semantic/models", semHandler.ListModels)
	r.Get("/semantic/models/{id}", semHandler.GetModel)
	r.Get("/semantic/models/{id}/fields", semHandler.ListModelFields)
	r.Get("/semantic/models/{id}/lineage", semHandler.GetModelLineage)
	r.Put("/semantic/models/{id}", semHandler.UpdateModel)
	r.Delete("/semantic/models/{id}", semHandler.DeleteModel)
	r.Post("/semantic/models/{id}/validate", semHandler.ValidateModel)
	r.Post("/semantic/models/{id}/compile-expression", semHandler.CompileExpression)
	r.Post("/semantic/models/{id}/publish", semHandler.PublishModel)
	r.Post("/semantic/models/{id}/rollback", semHandler.RollbackModel)
	r.Post("/semantic/models/{id}/sync-dimensions", semHandler.SyncDimensions)
	r.Post("/semantic/models/{id}/dimensions", semHandler.CreateDimension)
	r.Delete("/semantic/models/{id}/dimensions/{dimension_id}", semHandler.DeleteDimension)
	r.Put("/semantic/models/{id}/dimensions/{dimension_id}", semHandler.UpdateDimension)
	r.Get("/semantic/models/{id}/dimensions/{dimension_id}/enums", semHandler.GetDimensionEnums)
	r.Put("/semantic/models/{id}/dimensions/{dimension_id}/enums", semHandler.ReplaceDimensionEnums)
	r.Post("/semantic/models/{id}/metrics", semHandler.CreateMetric)
	r.Delete("/semantic/models/{id}/metrics/{metric_id}", semHandler.DeleteMetric)
	r.Put("/semantic/models/{id}/metrics/{metric_id}", semHandler.UpdateMetric)
	r.Post("/semantic/models/{id}/tables/remove", semHandler.RemoveTable)
	r.Post("/semantic/models/{id}/schemas/remove", semHandler.RemoveSchema)
	r.Post("/semantic/models/{id}/joins", semHandler.CreateJoin)
	r.Delete("/semantic/models/{id}/joins/{join_id}", semHandler.DeleteJoin)
	r.Put("/semantic/models/{id}/joins/{join_id}", semHandler.UpdateJoin)
	r.Get("/semantic/models/{id}/suggested-joins", semHandler.SuggestedJoins)

	driftHandler := handlers.NewDriftHandler(deps)
	r.Get("/semantic/models/{id}/drift", driftHandler.ListForModel)
	r.Get("/datasources/{id}/drift", driftHandler.ListForDatasource)
	r.Post("/drift/{id}/resolve", driftHandler.Resolve)
}

func registerCatalogCompositeRoutes(r chi.Router, deps *app.CatalogDeps) {
	compHandler := handlers.NewCompositeHandler(deps)
	r.Post("/semantic/composites", compHandler.CreateComposite)
	r.Get("/semantic/composites", compHandler.ListComposites)
	r.Get("/semantic/composites/{id}", compHandler.GetComposite)
	r.Put("/semantic/composites/{id}", compHandler.UpdateComposite)
	r.Delete("/semantic/composites/{id}", compHandler.DeleteComposite)
	r.Post("/semantic/composites/{id}/components", compHandler.AddComponent)
	r.Delete("/semantic/composites/{id}/components/{model_id}", compHandler.RemoveComponent)
	r.Post("/semantic/composites/{id}/cross-joins", compHandler.AddCrossJoin)
	r.Put("/semantic/composites/{id}/cross-joins/{join_id}", compHandler.UpdateCrossJoin)
	r.Delete("/semantic/composites/{id}/cross-joins/{join_id}", compHandler.RemoveCrossJoin)
	r.Put("/semantic/composites/{id}/canonical-date", compHandler.SetCanonicalDate)
	r.Put("/semantic/composites/{id}/dimension-resolutions", compHandler.SetDimensionResolutions)
	r.Post("/semantic/composites/{id}/validate", compHandler.ValidateComposite)
	r.Post("/semantic/composites/{id}/publish", compHandler.PublishComposite)
	r.Post("/semantic/composites/{id}/rollback", compHandler.RollbackComposite)
	r.Get("/semantic/composites/{id}/suggested-joins", compHandler.SuggestedJoins)
}

func registerCatalogMetadataRoutes(r chi.Router, deps *app.CatalogDeps, authClient *bimw.AuthClient) {
	metaHandler := handlers.NewMetadataHandler(deps)
	piiHandler := handlers.NewPIIHandler(deps)
	r.Get("/datasources/{id}/tables", metaHandler.ListTables)
	r.With(bimw.RequireDatasourceAccess(authClient, "read")).Get("/datasources/{id}/tables/{schema}/{table}/sample", metaHandler.GetTableSample)
	r.With(bimw.RequireDatasourceAccess(authClient, "read")).Post("/datasources/{id}/tables/{schema}/{table}/rows", metaHandler.BrowseTableRows)
	r.Get("/datasources/{id}/columns", metaHandler.ListColumns)
	r.Get("/metadata/columns/search", metaHandler.SearchColumns)
	r.Get("/metadata/tables/search", metaHandler.SearchTables)
	r.Patch("/metadata/tables/{id}", metaHandler.UpdateTableDescription)
	r.Patch("/metadata/columns/{id}", metaHandler.UpdateColumnDescription)
	r.Get("/metadata/tables/{id}/translations", metaHandler.GetTableTranslations)
	r.Put("/metadata/tables/{id}/translations", metaHandler.PutTableTranslations)
	r.Get("/metadata/columns/{id}/translations", metaHandler.GetColumnTranslations)
	r.Put("/metadata/columns/{id}/translations", metaHandler.PutColumnTranslations)
	r.With(bimw.RequirePermission(authClient, "admin:roles")).Patch("/metadata/columns/{id}/pii", piiHandler.UpdateColumn)
	r.With(bimw.RequirePermission(authClient, "admin:roles")).Delete("/metadata/columns/{id}/pii", piiHandler.DeleteColumn)
	r.With(bimw.RequirePermission(authClient, "admin:roles")).Get("/compliance/pii-summary", piiHandler.ComplianceSummary)
}

func registerCatalogPermissionRoutes(r chi.Router, deps *app.CatalogDeps, authClient *bimw.AuthClient) {
	permHandler := handlers.NewPermissionHandler(deps)
	r.With(bimw.RequirePermission(authClient, "admin:roles")).Route("/permissions", func(r chi.Router) {
		r.Get("/", permHandler.List)
		r.Get("/keys", permHandler.GetByKeys)
		r.Put("/", permHandler.Upsert)
		r.Delete("/{id}", permHandler.Delete)
		r.Delete("/keys", permHandler.DeleteByKeys)
	})
}

func registerCatalogDashboardRoutes(r chi.Router, deps *app.CatalogDeps) {
	dashHandler := handlers.NewDashboardHandler(deps.DashboardRepo)
	r.Route("/dashboards", func(r chi.Router) {
		r.Post("/", dashHandler.Create)
		r.Get("/", dashHandler.List)
		r.Get("/{id}", dashHandler.Get)
		r.Put("/{id}", dashHandler.Update)
		r.Delete("/{id}", dashHandler.Delete)
	})
}

func registerCatalogInternalRoutes(r chi.Router, deps *app.CatalogDeps, serviceName string) {
	internalHandler := handlers.NewInternalHandlerWithService(deps, serviceName)
	r.Get("/health", internalHandler.Health)

	r.Get("/datasources", internalHandler.ListDatasources)
	r.Get("/datasources/{id}", internalHandler.GetDatasource)
	r.Get("/models", internalHandler.ListModels)
	r.Get("/models/{id}", internalHandler.GetFullModel)
	r.Get("/datasources/{id}/tables", internalHandler.ListTables)
	r.Get("/datasources/{id}/columns", internalHandler.ListColumns)
	r.Get("/datasources/{id}/relations", internalHandler.ListRelations)
	r.Get("/few-shot", internalHandler.ListFewShot)
	r.Get("/glossary", internalHandler.ListGlossary)

	r.Post("/history/ai", internalHandler.CreateAIHistory)
	r.Post("/history/query", internalHandler.CreateQueryHistory)
	r.Post("/eval-results", internalHandler.CreateEvalResults)
}
