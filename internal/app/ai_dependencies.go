package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/audit"
	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/core"
	"github.com/biqly/biqly/internal/datasource"
	"github.com/biqly/biqly/internal/datasource/clickhouse"
	"github.com/biqly/biqly/internal/datasource/mysql"
	"github.com/biqly/biqly/internal/datasource/postgres"
	"github.com/biqly/biqly/internal/datasource/sqlserver"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/security"
	"github.com/biqly/biqly/internal/semantic"
	"github.com/biqly/biqly/pkg/catalogclient"
	"github.com/biqly/biqly/pkg/queryclient"
)

// NewAIDependencies wires the standalone AI Service dependency graph.
func NewAIDependencies(ctx context.Context, cfg *config.Config) (*Dependencies, error) {
	db, err := sql.Open("pgx", cfg.Metadata.DSN)
	if err != nil {
		return nil, fmt.Errorf("open metadata db: %w", err)
	}
	if pingErr := db.PingContext(ctx); pingErr != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping metadata db: %w", pingErr)
	}

	lims := datasource.DefaultPoolLimits()
	db.SetMaxOpenConns(lims.MaxOpen)
	db.SetMaxIdleConns(lims.MaxIdle)

	reg := datasource.NewRegistry()
	reg.Register(postgres.NewDriver())
	reg.Register(mysql.NewDriver())
	reg.Register(sqlserver.NewDriver())
	reg.Register(clickhouse.NewDriver())

	metaRepo := metadata.NewRepository(db)
	semanticRepo := semantic.NewRepository(db)

	var encryptor *security.Encryption
	enc, err := security.NewEncryption()
	if err != nil {
		slog.Warn("encryption disabled; datasource DSNs will be read as plaintext", "detail", err)
	} else {
		encryptor = enc
	}

	validator := query.NewValidator(cfg.Query.MaxRows)
	executor := query.NewExecutor(cfg.Query.MaxRows, cfg.QueryTimeout())
	queryService := core.NewQueryService(core.QueryServiceDeps{
		Models:      semanticRepo,
		Datasources: metaRepo,
		Drivers:     reg,
		Validator:   validator,
		Executor:    executor,
		History:     metaRepo,
		Encryptor:   encryptor,
	})

	aiClient, err := ai.NewProvider(cfg.AI)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ai provider: %w", err)
	}
	aiQueryClient := aiClient
	if cfg.AI.HasQueryOverride() {
		queryClient, err := ai.NewProvider(cfg.AI.EffectiveQueryConfig())
		if err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("ai query provider: %w", err)
		}
		aiQueryClient = queryClient
		slog.Info("AI query provider overridden",
			"model", cfg.AI.EffectiveQueryConfig().Model,
			"base_url", cfg.AI.EffectiveQueryConfig().BaseURL,
			"describe_model", cfg.AI.Model)
	}

	translator := ai.NewTranslationServiceFromConfig(cfg.AI)
	describer := ai.NewDescribeService(aiClient, metaRepo, reg, translator, 10, cfg.AI.DescribeMaxCellRunes, cfg.AI.DescribeMaxSampleRows, encryptor).WithModel(cfg.AI.Model)

	var embedder ai.Embedder
	var embedMeta *ai.EmbedMetadataService
	if cfg.AI.EmbeddingsConfigured() {
		embedder = ai.NewOpenAIEmbedder(cfg.AI)
		embedMeta = ai.NewEmbedMetadataService(embedder, metaRepo)
	}

	if err := ai.InitRouting(cfg.AI.RoutingLexiconPath, cfg.AI.RoutingWeightsPath); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("routing config: %w", err)
	}

	ai.SetPromptTemplateStore(ai.NewDBPromptTemplateStore(metaRepo))
	if err := ai.SeedPromptTemplatesFromEmbed(ctx, metaRepo); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("seed prompt templates: %w", err)
	}

	timeGrainsStore := ai.NewDBTimeGrainStore(metaRepo)
	if err := ai.SeedTimeGrains(ctx, metaRepo); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("seed time grains: %w", err)
	}

	var catalogHTTPClient *catalogclient.Client
	if cfg.Services.CatalogURL != "" {
		catalogHTTPClient = catalogclient.New(
			cfg.Services.CatalogURL,
			catalogclient.WithAuthToken(cfg.Security.InternalAPIToken),
			catalogclient.WithCaller("ai"),
		)
		slog.Info("ai service using Catalog Service for metadata/history", "catalog_url", catalogHTTPClient.BaseURL())
	}

	var queryHTTPClient *queryclient.Client
	if cfg.Services.QueryURL != "" {
		queryHTTPClient = queryclient.New(
			cfg.Services.QueryURL,
			queryclient.WithAuthToken(cfg.Security.InternalAPIToken),
			queryclient.WithCaller("ai"),
		)
		slog.Info("ai service using Query Engine for compile/run/dry-run", "query_url", queryHTTPClient.BaseURL())
	}

	return &Dependencies{
		Config:        cfg,
		MetadataDB:    db,
		DriverReg:     reg,
		MetaRepo:      metaRepo,
		SemanticRepo:  semanticRepo,
		Validator:     validator,
		Executor:      executor,
		QueryService:  queryService,
		CatalogClient: catalogHTTPClient,
		QueryClient:   queryHTTPClient,
		AIClient:      aiClient,
		AIQueryClient: aiQueryClient,
		AIDescriber:   describer,
		Encryptor:     encryptor,
		EvalRepo:      ai.NewEvalRepository(db),
		AuditLogger:   audit.NewLogger(slog.Default()),
		Embedder:      embedder,
		AIEmbedMeta:   embedMeta,
		Jobs:          cfg.Jobs,
		TimeGrains:    timeGrainsStore,
	}, nil
}
