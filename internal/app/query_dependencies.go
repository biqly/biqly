package app

import (
	"context"
	"log/slog"

	"github.com/biqly/biqly/internal/audit"
	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/core"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/security"
	"github.com/biqly/biqly/internal/semantic"
	"github.com/biqly/biqly/pkg/catalogclient"
)

// NewQueryDependencies wires only the Query Engine dependency graph.
func NewQueryDependencies(ctx context.Context, cfg *config.Config) (*Dependencies, error) {
	db, err := openMetadataDB(ctx, cfg)
	if err != nil {
		return nil, err
	}

	reg := newDriverRegistry()

	metaRepo := metadata.NewRepository(db)
	semanticRepo := semantic.NewRepository(db)

	var encryptor *security.Encryption
	enc, err := security.NewEncryption()
	if err != nil {
		slog.Warn("encryption disabled; DSNs will be read as plaintext", "detail", err)
	} else {
		encryptor = enc
	}

	validator := query.NewValidator(cfg.Query.MaxRows)
	executor := query.NewExecutor(cfg.Query.MaxRows, cfg.QueryTimeout())
	models := core.ModelLoader(semanticRepo)
	datasources := core.DatasourceLoader(metaRepo)
	history := core.HistoryRecorder(metaRepo)
	if cfg.Services.CatalogURL != "" {
		catalog := catalogclient.New(
			cfg.Services.CatalogURL,
			catalogclient.WithAuthToken(cfg.Security.InternalAPIToken),
			catalogclient.WithCaller("query"),
		)
		adapter := newQueryCatalogAdapter(catalog)
		models = adapter
		datasources = adapter
		history = adapter
		slog.Info("query engine using Catalog Service for model/datasource/history",
			"catalog_url", catalog.BaseURL())
	}
	queryService := core.NewQueryService(core.QueryServiceDeps{
		Models:      models,
		Datasources: datasources,
		Drivers:     reg,
		Validator:   validator,
		Executor:    executor,
		History:     history,
		Encryptor:   encryptor,
	})

	return &Dependencies{
		Config:       cfg,
		MetadataDB:   db,
		DriverReg:    reg,
		MetaRepo:     metaRepo,
		SemanticRepo: semanticRepo,
		Validator:    validator,
		Executor:     executor,
		QueryService: queryService,
		Encryptor:    encryptor,
		AuditLogger:  audit.NewLogger(slog.Default()),
	}, nil
}
