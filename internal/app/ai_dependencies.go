package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/biqly/biqly/internal/ai"
	evalpkg "github.com/biqly/biqly/internal/ai/eval"
	"github.com/biqly/biqly/internal/ai/prompt"
	providerpkg "github.com/biqly/biqly/internal/ai/provider"
	"github.com/biqly/biqly/internal/ai/routing"
	"github.com/biqly/biqly/internal/audit"
	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/core"
	"github.com/biqly/biqly/pkg/catalogclient"
	"github.com/biqly/biqly/pkg/queryclient"
)

// NewAIDependencies wires the standalone AI Service dependency graph.
func NewAIDependencies(ctx context.Context, cfg *config.Config) (*Dependencies, error) {
	db, err := openMetadataDB(ctx, cfg)
	if err != nil {
		return nil, err
	}

	reg := newDriverRegistry()

	metaRepo, semanticRepo := provideRepositories(db)

	encryptor := provideEncryptor(ctx, db, false)

	validator, executor := provideQueryEngine(cfg)
	queryService := core.NewQueryService(core.QueryServiceDeps{
		Models:      semanticRepo,
		Datasources: metaRepo,
		Drivers:     reg,
		Validator:   validator,
		Executor:    executor,
		History:     metaRepo,
		Encryptor:   encryptor,
	})

	aiClient, err := providerpkg.NewProvider(cfg.AI)
	if err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ai provider: %w", err)
	}
	aiQueryClient := aiClient
	if cfg.AI.HasQueryOverride() {
		queryClient, err := providerpkg.NewProvider(cfg.AI.EffectiveQueryConfig())
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
		embedder = providerpkg.NewOpenAIEmbedder(cfg.AI)
		embedMeta = ai.NewEmbedMetadataService(embedder, metaRepo)
	}

	if err := routing.InitRouting(cfg.AI.RoutingLexiconPath, cfg.AI.RoutingWeightsPath); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("routing config: %w", err)
	}

	prompt.SetPromptTemplateStore(prompt.NewDBPromptTemplateStore(metaRepo))
	if err := prompt.SeedPromptTemplatesFromEmbed(ctx, metaRepo); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("seed prompt templates: %w", err)
	}

	timeGrainsStore := routing.NewDBTimeGrainStore(metaRepo)
	if err := routing.SeedTimeGrains(ctx, metaRepo); err != nil {
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
		EvalRepo:      evalpkg.NewEvalRepository(db),
		AuditLogger:   audit.NewLogger(slog.Default()),
		Embedder:      embedder,
		AIEmbedMeta:   embedMeta,
		Jobs:          cfg.Jobs,
		TimeGrains:    timeGrainsStore,
	}, nil
}
