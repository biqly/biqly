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

// CatalogRouter sets up only the routes owned by the Catalog Service.
func CatalogRouter(deps *app.Dependencies) http.Handler {
	r := chi.NewRouter()

	ApplyBaseMiddleware(r, BaseMiddlewareConfig{
		Metrics: GetMetrics(),
		Timeout: 60 * time.Second,
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
	r.Get("/ready", ReadinessHandler(deps, nil))
	r.With(bimw.APIKeyAuth(deps.Config.Security.MetricsAPIKey)).Get("/metrics", MetricsHandler)

	// /api is the catalog's user-facing surface, reached directly through the
	// gateway — it must enforce auth itself (BI_AUTH_ENABLED=true → JWT with
	// admin-key bypass; otherwise legacy API key). The auth client additionally
	// wires per-datasource access checks on routes that carry a datasource_id.
	authMW := buildAPIAuthMiddleware(deps)
	authClient := NewAuthClient(deps)
	r.Route("/api", func(r chi.Router) {
		// Anonymous public embed surface — a sibling of the authed group inside
		// the same /api mount (chi forbids overlapping top-level wildcards).
		// Registered BEFORE any Use on the /api mux so no route is bound to the
		// bare mux; the public group carries NO authMW.
		r.Route("/public", func(r chi.Router) {
			r.Use(bimw.PublicEmbedHeaders)
			registerPublicDashboardRoutes(r, deps.CatalogDeps(), authClient)
		})
		r.Group(func(r chi.Router) {
			r.Use(authMW)
			r.Use(CatalogMetricsMiddleware(GetMetrics()))
			registerCatalogAPIRoutes(r, deps.CatalogDeps(), authClient)
		})
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
	registerCatalogSemanticRoutes(r, deps, authClient)
	registerCatalogCompositeRoutes(r, deps, authClient)
	registerCatalogMetadataRoutes(r, deps, authClient)
	registerCatalogPermissionRoutes(r, deps, authClient)
	registerCatalogDashboardRoutes(r, deps, authClient)
}

func registerCatalogDatasourceRoutes(r chi.Router, deps *app.CatalogDeps, authClient *bimw.AuthClient) {
	dsHandler := handlers.NewDatasourceHandler(deps)
	// Create and test-connection are gated on datasource:create — otherwise any
	// authenticated user could create datasources and, via test-connection, make
	// the server open a connection to an arbitrary attacker-supplied host:port.
	r.With(bimw.RequirePermission(authClient, "datasource:create")).Post("/datasources", dsHandler.Create)
	r.Get("/datasources", dsHandler.List)
	r.With(bimw.RequirePermission(authClient, "datasource:create")).Post("/datasources/test-connection", dsHandler.TestDraft)
	r.With(bimw.RequireDatasourceAccess(authClient, "read")).Get("/datasources/{id}", dsHandler.Get)
	r.With(bimw.RequireDatasourceAccess(authClient, "write")).Put("/datasources/{id}", dsHandler.Update)
	r.With(bimw.RequireDatasourceAccess(authClient, "admin")).Delete("/datasources/{id}", dsHandler.Delete)
	r.With(bimw.RequireDatasourceAccess(authClient, "admin")).Get("/datasources/{id}/function-blocklist", dsHandler.GetFunctionBlocklist)
	r.With(bimw.RequireDatasourceAccess(authClient, "admin")).Put("/datasources/{id}/function-blocklist", dsHandler.ReplaceFunctionBlocklist)
	r.With(bimw.RequireDatasourceAccess(authClient, "read")).Post("/datasources/{id}/test", dsHandler.Test)
	r.With(bimw.RequireDatasourceAccess(authClient, "write")).Post("/datasources/{id}/sync-metadata", dsHandler.SyncMetadata)

	piiHandler := handlers.NewPIIHandler(deps)
	r.With(bimw.RequireDatasourceAccess(authClient, "write")).Post("/datasources/{id}/scan-pii", piiHandler.Scan)
	r.With(bimw.RequireDatasourceAccess(authClient, "read")).Get("/datasources/{id}/pii-columns", piiHandler.ListColumns)
}

func registerCatalogSemanticRoutes(r chi.Router, deps *app.CatalogDeps, authClient *bimw.AuthClient) {
	semHandler := handlers.NewSemanticHandler(deps)
	semHandler.SetCatalogMetricsRecorder(GetMetrics())

	// Model routes carry a model {id}; resolve it to the owning datasource and
	// check per-datasource access so a user can't read/mutate another tenant's
	// model. Create/generate carry datasource_id in the body instead.
	modelDS := func(ctx context.Context, id string) (string, error) {
		m, err := deps.SemanticRepo.GetModel(ctx, id)
		if err != nil {
			return "", err
		}
		return m.DatasourceID, nil
	}
	modelRead := bimw.RequireResolvedDatasourceAccess(authClient, "read", modelDS)
	modelWrite := bimw.RequireResolvedDatasourceAccess(authClient, "write", modelDS)
	dsWrite := bimw.RequireDatasourceAccess(authClient, "write")
	dsRead := bimw.RequireDatasourceAccess(authClient, "read")

	r.With(dsWrite).Post("/semantic/models", semHandler.CreateModel)
	r.With(dsWrite).Post("/semantic/models/generate", semHandler.GenerateModel)
	r.With(dsWrite).Post("/semantic/models/import", semHandler.ImportModel)
	r.With(dsWrite).Post("/catalog/dbt/import", semHandler.ImportDbtProject)
	r.Get("/semantic/models", semHandler.ListModels)
	r.With(modelRead).Get("/semantic/models/{id}", semHandler.GetModel)
	r.With(modelRead).Get("/semantic/models/{id}/export", semHandler.ExportModel)
	r.With(modelRead).Get("/semantic/models/{id}/versions", semHandler.ListModelVersions)
	r.With(modelRead).Get("/semantic/models/{id}/versions/{version}/export", semHandler.ExportModelVersion)
	r.With(modelRead).Get("/semantic/models/{id}/fields", semHandler.ListModelFields)
	r.With(modelRead).Get("/semantic/models/{id}/lineage", semHandler.GetModelLineage)
	r.With(modelWrite).Put("/semantic/models/{id}", semHandler.UpdateModel)
	r.With(modelWrite).Delete("/semantic/models/{id}", semHandler.DeleteModel)
	r.With(modelRead).Post("/semantic/models/{id}/validate", semHandler.ValidateModel)
	r.With(modelRead).Post("/semantic/models/{id}/compile-expression", semHandler.CompileExpression)
	r.With(modelWrite).Post("/semantic/models/{id}/publish", semHandler.PublishModel)
	r.With(modelWrite).Post("/semantic/models/{id}/rollback", semHandler.RollbackModel)
	r.With(modelWrite).Post("/semantic/models/{id}/sync-dimensions", semHandler.SyncDimensions)
	r.With(modelWrite).Post("/semantic/models/{id}/dimensions", semHandler.CreateDimension)
	r.With(modelWrite).Delete("/semantic/models/{id}/dimensions/{dimension_id}", semHandler.DeleteDimension)
	r.With(modelWrite).Put("/semantic/models/{id}/dimensions/{dimension_id}", semHandler.UpdateDimension)
	r.With(modelRead).Get("/semantic/models/{id}/dimensions/{dimension_id}/enums", semHandler.GetDimensionEnums)
	r.With(modelWrite).Put("/semantic/models/{id}/dimensions/{dimension_id}/enums", semHandler.ReplaceDimensionEnums)
	r.With(modelWrite).Post("/semantic/models/{id}/metrics", semHandler.CreateMetric)
	r.With(modelWrite).Delete("/semantic/models/{id}/metrics/{metric_id}", semHandler.DeleteMetric)
	r.With(modelWrite).Put("/semantic/models/{id}/metrics/{metric_id}", semHandler.UpdateMetric)
	r.With(modelWrite).Post("/semantic/models/{id}/tables/remove", semHandler.RemoveTable)
	r.With(modelWrite).Post("/semantic/models/{id}/schemas/remove", semHandler.RemoveSchema)
	r.With(modelWrite).Post("/semantic/models/{id}/joins", semHandler.CreateJoin)
	r.With(modelWrite).Delete("/semantic/models/{id}/joins/{join_id}", semHandler.DeleteJoin)
	r.With(modelWrite).Put("/semantic/models/{id}/joins/{join_id}", semHandler.UpdateJoin)
	r.With(modelRead).Get("/semantic/models/{id}/suggested-joins", semHandler.SuggestedJoins)

	driftHandler := handlers.NewDriftHandler(deps)
	driftDS := func(ctx context.Context, id string) (string, error) {
		return deps.DriftRepo.DatasourceForReport(ctx, id)
	}
	r.With(modelRead).Get("/semantic/models/{id}/drift", driftHandler.ListForModel)
	r.With(dsRead).Get("/datasources/{id}/drift", driftHandler.ListForDatasource)
	r.With(bimw.RequireResolvedDatasourceAccess(authClient, "write", driftDS)).Post("/drift/{id}/resolve", driftHandler.Resolve)
}

func registerCatalogCompositeRoutes(r chi.Router, deps *app.CatalogDeps, authClient *bimw.AuthClient) {
	compHandler := handlers.NewCompositeHandler(deps)

	compDS := func(ctx context.Context, id string) (string, error) {
		c, err := deps.CompositeRepo.GetComposite(ctx, id)
		if err != nil {
			return "", err
		}
		return c.DatasourceID, nil
	}
	compRead := bimw.RequireResolvedDatasourceAccess(authClient, "read", compDS)
	compWrite := bimw.RequireResolvedDatasourceAccess(authClient, "write", compDS)

	r.With(bimw.RequireDatasourceAccess(authClient, "write")).Post("/semantic/composites", compHandler.CreateComposite)
	r.Get("/semantic/composites", compHandler.ListComposites)
	r.With(compRead).Get("/semantic/composites/{id}", compHandler.GetComposite)
	r.With(compWrite).Put("/semantic/composites/{id}", compHandler.UpdateComposite)
	r.With(compWrite).Delete("/semantic/composites/{id}", compHandler.DeleteComposite)
	r.With(compWrite).Post("/semantic/composites/{id}/components", compHandler.AddComponent)
	r.With(compWrite).Delete("/semantic/composites/{id}/components/{model_id}", compHandler.RemoveComponent)
	r.With(compWrite).Post("/semantic/composites/{id}/cross-joins", compHandler.AddCrossJoin)
	r.With(compWrite).Put("/semantic/composites/{id}/cross-joins/{join_id}", compHandler.UpdateCrossJoin)
	r.With(compWrite).Delete("/semantic/composites/{id}/cross-joins/{join_id}", compHandler.RemoveCrossJoin)
	r.With(compWrite).Put("/semantic/composites/{id}/canonical-date", compHandler.SetCanonicalDate)
	r.With(compWrite).Put("/semantic/composites/{id}/dimension-resolutions", compHandler.SetDimensionResolutions)
	r.With(compRead).Post("/semantic/composites/{id}/validate", compHandler.ValidateComposite)
	r.With(compWrite).Post("/semantic/composites/{id}/publish", compHandler.PublishComposite)
	r.With(compWrite).Post("/semantic/composites/{id}/rollback", compHandler.RollbackComposite)
	r.With(compRead).Get("/semantic/composites/{id}/suggested-joins", compHandler.SuggestedJoins)
}

func registerCatalogMetadataRoutes(r chi.Router, deps *app.CatalogDeps, authClient *bimw.AuthClient) {
	metaHandler := handlers.NewMetadataHandler(deps)
	metaHandler.SetDatasourceAccessChecker(authClient)
	piiHandler := handlers.NewPIIHandler(deps)
	r.With(bimw.RequireDatasourceAccess(authClient, "read")).Get("/datasources/{id}/tables", metaHandler.ListTables)
	r.With(bimw.RequireDatasourceAccess(authClient, "read")).Get("/datasources/{id}/tables/{schema}/{table}/sample", metaHandler.GetTableSample)
	r.With(bimw.RequireDatasourceAccess(authClient, "read")).Post("/datasources/{id}/tables/{schema}/{table}/rows", metaHandler.BrowseTableRows)
	r.With(bimw.RequireDatasourceAccess(authClient, "read")).Get("/datasources/{id}/columns", metaHandler.ListColumns)
	r.With(bimw.RequireDatasourceAccess(authClient, "read")).Get("/datasources/{id}/relations", metaHandler.ListRelations)
	r.With(bimw.RequireDatasourceAccess(authClient, "read")).Get("/metadata/columns/search", metaHandler.SearchColumns)
	r.With(bimw.RequireDatasourceAccess(authClient, "read")).Get("/metadata/tables/search", metaHandler.SearchTables)
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

func registerCatalogDashboardRoutes(r chi.Router, deps *app.CatalogDeps, authClient *bimw.AuthClient) {
	dashHandler := handlers.NewDashboardHandler(deps.DashboardRepo)
	shareHandler := handlers.NewDashboardShareHandler(deps.DashboardShareRepo, deps.DashboardRepo, authClient, deps.AuditLogger)
	r.Route("/dashboards", func(r chi.Router) {
		r.Post("/", dashHandler.Create)
		r.Get("/", dashHandler.List)
		r.Get("/{id}", dashHandler.Get)
		r.Put("/{id}", dashHandler.Update)
		r.Delete("/{id}", dashHandler.Delete)
		r.Post("/{id}/public-share", shareHandler.Create)
		r.Get("/{id}/public-share", shareHandler.Status)
		r.Delete("/{id}/public-share", shareHandler.Revoke)
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
	r.Get("/resource-datasource", internalHandler.ResolveResourceDatasource)

	r.Post("/history/ai", internalHandler.CreateAIHistory)
	r.Post("/history/query", internalHandler.CreateQueryHistory)
	r.Post("/eval-results", internalHandler.CreateEvalResults)
}
