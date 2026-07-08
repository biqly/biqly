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
	"github.com/biqly/biqly/internal/datasource"
	"github.com/biqly/biqly/internal/semantic"
	"github.com/biqly/biqly/pkg/catalogclient"
	"github.com/biqly/biqly/pkg/queryclient"
)

// NewAIDependencies wires the standalone AI Service dependency graph.
func NewAIDependencies(ctx context.Context, cfg *config.Config) (*Dependencies, error) {
	providerpkg.SetAirgapped(cfg.DeploymentMode == config.DeploymentModeAirgapped)

	db, err := openMetadataDB(ctx, cfg)
	if err != nil {
		return nil, err
	}

	reg := newDriverRegistry()

	metaRepo, semanticRepo := provideRepositories(db)

	encryptor := provideEncryptor(ctx, db, false)

	validator, executor := provideQueryEngine(cfg)
	auditLogger := audit.NewLogger(slog.Default()).WithDBWriter(audit.NewDBWriter(ctx, db, slog.Default()))
	piiPolicies := providePIIPolicyService(cfg, metaRepo, auditLogger)
	queryService := core.NewQueryService(&core.QueryServiceDeps{
		Models:      semanticRepo,
		Composites:  semantic.NewCompositeRepository(db),
		Datasources: metaRepo,
		Drivers:     reg,
		Validator:   validator,
		Executor:    executor,
		History:     metaRepo,
		Encryptor:   encryptor,
		PIIPolicies: piiPolicies,
		Audit:       auditLogger,
		Identity:    jwtIdentity,
	})

	providerStore := provideProviderStore(ctx, cfg, db, encryptor)

	// Provider/model selection is DB-only: the AI clients resolve their backend
	// per purpose from the ProviderStore on every call. There is no environment
	// fallback — when no default model is configured for a purpose the provider
	// returns a clear "no model configured" error rather than silently using a
	// baked-in model.
	effectiveCfg := providerStore.EffectiveConfig()
	aiClient := ai.NewPurposeProvider(providerStore, ai.PurposeDescribe, nil, nil)
	aiQueryClient := ai.NewPurposeProvider(providerStore, ai.PurposeQuery, nil, nil)
	describeModel := ""
	if describeCfg, ok := providerStore.ChatConfigForPurpose(ai.PurposeDescribe); ok {
		describeModel = describeCfg.Connection.Model
	}

	poolCache := datasource.NewPoolCache()

	translator := ai.NewTranslationServiceFromProviderStore(providerStore, effectiveCfg)
	describer := ai.NewDescribeService(aiClient, metaRepo, reg, translator, 10, cfg.AI.Describe.MaxCellRunes, cfg.AI.Describe.MaxSampleRows, encryptor).
		WithModel(describeModel).
		WithPoolCache(poolCache)

	var embedder ai.Embedder
	var embedMeta *ai.EmbedMetadataService
	if effectiveCfg.ResolvedEmbedding().Configured() {
		embedder = providerpkg.NewOpenAIEmbedder(effectiveCfg)
		embedMeta = ai.NewEmbedMetadataService(embedder, metaRepo)
	}

	if err := routing.InitRouting(cfg.AI.Routing.LexiconPath, cfg.AI.Routing.WeightsPath); err != nil {
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

	if err := wireNLRuntime(ctx, metaRepo); err != nil {
		_ = db.Close()
		return nil, err
	}

	catalogHTTPClient, queryHTTPClient := provideServiceClients(cfg)
	aiRedis := newAIRedisClient(ctx, cfg.Redis.DSN)
	var responseCache ai.ResponseCache
	if aiRedis != nil {
		responseCache = ai.NewRedisResponseCache(aiRedis)
	}
	spendLimiter := ai.NewSpendLimiter(aiRedis, cfg.AI.Generation.WorkspaceDailyTokenBudget)

	return &Dependencies{
		Config:          cfg,
		MetadataDB:      db,
		DriverReg:       reg,
		MetaRepo:        metaRepo,
		SemanticRepo:    semanticRepo,
		CompositeRepo:   semantic.NewCompositeRepository(db),
		Validator:       validator,
		Executor:        executor,
		QueryService:    queryService,
		PoolCache:       poolCache,
		CatalogClient:   catalogHTTPClient,
		QueryClient:     queryHTTPClient,
		AIClient:        aiClient,
		AIQueryClient:   aiQueryClient,
		AIDescriber:     describer,
		Encryptor:       encryptor,
		EvalRepo:        evalpkg.NewEvalRepository(db),
		AuditLogger:     auditLogger,
		PIIPolicies:     piiPolicies,
		Embedder:        embedder,
		AIEmbedMeta:     embedMeta,
		Jobs:            cfg.Jobs,
		TimeGrains:      timeGrainsStore,
		AIProviderStore: providerStore,
		ResponseCache:   responseCache,
		SpendLimiter:    spendLimiter,
		AIRedis:         aiRedis,
	}, nil
}

// provideServiceClients builds the optional Catalog and Query Engine HTTP
// clients when their service URLs are configured.
func provideServiceClients(cfg *config.Config) (*catalogclient.Client, *queryclient.Client) {
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
	return catalogHTTPClient, queryHTTPClient
}
