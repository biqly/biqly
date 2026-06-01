// Package app wires together application dependencies.
package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"

	"github.com/biqly/biqly/internal/ai"
	evalpkg "github.com/biqly/biqly/internal/ai/eval"
	"github.com/biqly/biqly/internal/ai/prompt"
	providerpkg "github.com/biqly/biqly/internal/ai/provider"
	"github.com/biqly/biqly/internal/ai/routing"
	"github.com/biqly/biqly/internal/audit"
	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/core"
	"github.com/biqly/biqly/internal/datasource"
	"github.com/biqly/biqly/internal/datasource/clickhouse"
	"github.com/biqly/biqly/internal/datasource/mysql"
	"github.com/biqly/biqly/internal/datasource/postgres"
	"github.com/biqly/biqly/internal/datasource/sqlserver"
	"github.com/biqly/biqly/internal/dashboard"
	"github.com/biqly/biqly/internal/metadata"
	platformdb "github.com/biqly/biqly/internal/platform/db"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/queue"
	"github.com/biqly/biqly/internal/security"
	"github.com/biqly/biqly/internal/semantic"
	"github.com/biqly/biqly/pkg/catalogclient"
	"github.com/biqly/biqly/pkg/queryclient"
	_ "github.com/jackc/pgx/v5/stdlib" // PostgreSQL driver registration
	"github.com/redis/go-redis/v9"
)

// Dependencies holds all application dependencies.
type Dependencies struct {
	Config        *config.Config
	MetadataDB    *sql.DB
	DriverReg     *datasource.Registry
	MetaRepo      *metadata.Repository
	SemanticRepo  *semantic.Repository
	Validator     *query.Validator
	Executor      *query.Executor
	QueryService  *core.QueryService
	CatalogClient *catalogclient.Client
	QueryClient   *queryclient.Client
	AIClient      providerpkg.Provider
	// AIQueryClient powers the NL→LogicalQuery path. Aliases AIClient when no
	// BI_AI_QUERY_* overrides are set; otherwise points at a separate provider
	// (typically a smarter model) so describe/metadata work can keep using the
	// cheaper local model on AIClient.
	AIQueryClient providerpkg.Provider
	AIDescriber   *ai.DescribeService
	Encryptor     *security.Encryption
	EvalRepo      *evalpkg.EvalRepository
	AuditLogger   *audit.Logger
	// Embedder is the embeddings provider used for vector-based table
	// retrieval. nil when no API key is configured — callers MUST tolerate
	// nil (the table router falls back to keyword scoring).
	Embedder    ai.Embedder
	AIEmbedMeta *ai.EmbedMetadataService
	TimeGrains  routing.TimeGrainStore
	// AIProviderStore is the DB-backed AI provider/model registry. Always
	// non-nil; it falls back to the env config when no DB rows are configured.
	AIProviderStore *ai.ProviderStore
	ResponseCache   ai.ResponseCache
	Jobs            config.JobsConfig
	AIJobQueue      queue.AIJobPublisher
	AIJobService    AIJobRunner
	AIJobsHTTP      AIJobsHTTPHandler
	// PoolCache holds *sql.DB pools for external datasources. Closed during
	// Dependencies.Close().
	PoolCache *datasource.PoolCache
	DashboardRepo *dashboard.Repository
}

// CatalogDeps holds the subset of dependencies needed for the Catalog service
// (datasources, schemas, tables, columns, relations, semantic models).
type CatalogDeps struct {
	Config        *config.Config
	DriverReg     *datasource.Registry
	MetaRepo      *metadata.Repository
	SemanticRepo  *semantic.Repository
	Encryptor     *security.Encryption
	PoolCache     *datasource.PoolCache
	QueryService  *core.QueryService
	DashboardRepo *dashboard.Repository
}

// CatalogDeps returns a structured copy of dependencies for the Catalog subsystem.
func (d *Dependencies) CatalogDeps() *CatalogDeps {
	return &CatalogDeps{
		Config:        d.Config,
		DriverReg:     d.DriverReg,
		MetaRepo:      d.MetaRepo,
		SemanticRepo:  d.SemanticRepo,
		Encryptor:     d.Encryptor,
		PoolCache:     d.PoolCache,
		QueryService:  d.QueryService,
		DashboardRepo: d.DashboardRepo,
	}
}

// AIDeps holds the subset of dependencies needed for the AI service
// (NL to LogicalQuery, AI jobs, evaluation, descriptions, feedback, few-shot examples).
type AIDeps struct {
	Config          *config.Config
	DriverReg       *datasource.Registry
	MetaRepo        *metadata.Repository
	SemanticRepo    *semantic.Repository
	Validator       *query.Validator
	QueryService    *core.QueryService
	CatalogClient   *catalogclient.Client
	QueryClient     *queryclient.Client
	AIClient        providerpkg.Provider
	AIQueryClient   providerpkg.Provider
	AIDescriber     *ai.DescribeService
	Encryptor       *security.Encryption
	EvalRepo        *evalpkg.EvalRepository
	AuditLogger     *audit.Logger
	Embedder        ai.Embedder
	AIEmbedMeta     *ai.EmbedMetadataService
	TimeGrains      routing.TimeGrainStore
	AIProviderStore *ai.ProviderStore
	ResponseCache   ai.ResponseCache
	Jobs            config.JobsConfig
	AIJobQueue      queue.AIJobPublisher
	AIJobService    AIJobRunner
	AIJobsHTTP      AIJobsHTTPHandler
	PoolCache       *datasource.PoolCache
	Executor        *query.Executor
}

// AIDeps returns a structured copy of dependencies for the AI subsystem.
func (d *Dependencies) AIDeps() *AIDeps {
	return &AIDeps{
		Config:          d.Config,
		DriverReg:       d.DriverReg,
		MetaRepo:        d.MetaRepo,
		SemanticRepo:    d.SemanticRepo,
		Validator:       d.Validator,
		QueryService:    d.QueryService,
		CatalogClient:   d.CatalogClient,
		QueryClient:     d.QueryClient,
		AIClient:        d.AIClient,
		AIQueryClient:   d.AIQueryClient,
		AIDescriber:     d.AIDescriber,
		Encryptor:       d.Encryptor,
		EvalRepo:        d.EvalRepo,
		AuditLogger:     d.AuditLogger,
		Embedder:        d.Embedder,
		AIEmbedMeta:     d.AIEmbedMeta,
		TimeGrains:      d.TimeGrains,
		AIProviderStore: d.AIProviderStore,
		ResponseCache:   d.ResponseCache,
		Jobs:            d.Jobs,
		AIJobQueue:      d.AIJobQueue,
		AIJobService:    d.AIJobService,
		AIJobsHTTP:      d.AIJobsHTTP,
		PoolCache:       d.PoolCache,
		Executor:        d.Executor,
	}
}

// QueryDeps holds the subset of dependencies needed for the Query compiling and execution engine
// (query run, compile, query history).
type QueryDeps struct {
	Config       *config.Config
	MetaRepo     *metadata.Repository
	Validator    *query.Validator
	Executor     *query.Executor
	QueryService *core.QueryService
	AuditLogger  *audit.Logger
}

// QueryDeps returns a structured copy of dependencies for the Query subsystem.
func (d *Dependencies) QueryDeps() *QueryDeps {
	return &QueryDeps{
		Config:       d.Config,
		MetaRepo:     d.MetaRepo,
		Validator:    d.Validator,
		Executor:     d.Executor,
		QueryService: d.QueryService,
		AuditLogger:  d.AuditLogger,
	}
}

// AIJobRunner processes queued NL→query jobs (implemented by handlers.AIJobService).
type AIJobRunner interface {
	Process(ctx context.Context, jobID string) error
	StartConsumer(ctx context.Context, group string) error
}

// AIJobsHTTPHandler serves /api/ai/jobs* (implemented by handlers.AIJobsHandler).
type AIJobsHTTPHandler interface {
	Create(http.ResponseWriter, *http.Request)
	Get(http.ResponseWriter, *http.Request)
	List(http.ResponseWriter, *http.Request)
	Cancel(http.ResponseWriter, *http.Request)
	ListStale(http.ResponseWriter, *http.Request)
	CancelBatch(http.ResponseWriter, *http.Request)
	CancelActive(http.ResponseWriter, *http.Request)
	AdminListStale(http.ResponseWriter, *http.Request)
	AdminCancelAllStale(http.ResponseWriter, *http.Request)
	DescribeBatchConflict(http.ResponseWriter, *http.Request)
	QueueStatus(http.ResponseWriter, *http.Request)
}

// NewDependencies wires up all dependencies. The constructor is composed
// from smaller setup helpers (one per subsystem) so each can be reasoned
// about in isolation and replaced in tests if needed.
func NewDependencies(ctx context.Context, cfg *config.Config) (*Dependencies, error) {
	db, err := openMetadataDB(ctx, cfg)
	if err != nil {
		return nil, err
	}

	reg := newDriverRegistry()

	metaRepo, semanticRepo := provideRepositories(db)
	dashboardRepo := dashboard.NewRepository(db)

	encryptor := provideEncryptor(ctx, db, true)

	validator, executor := provideQueryEngine(cfg)
	poolCache := datasource.NewPoolCache()
	queryService := core.NewQueryService(core.QueryServiceDeps{
		Models:      semanticRepo,
		Datasources: metaRepo,
		Drivers:     reg,
		Validator:   validator,
		Executor:    executor,
		History:     metaRepo,
		Encryptor:   encryptor,
		Pools:       poolCache,
	})

	aiBits, err := setupAI(ctx, cfg, db, metaRepo, reg, encryptor)
	if err != nil {
		return nil, err
	}

	return &Dependencies{
		Config:          cfg,
		MetadataDB:      db,
		DriverReg:       reg,
		MetaRepo:        metaRepo,
		SemanticRepo:    semanticRepo,
		Validator:       validator,
		Executor:        executor,
		QueryService:    queryService,
		AIClient:        aiBits.client,
		AIQueryClient:   aiBits.queryClient,
		AIDescriber:     aiBits.describer,
		Encryptor:       encryptor,
		EvalRepo:        aiBits.evalRepo,
		AuditLogger:     audit.NewLogger(slog.Default()).WithDBWriter(audit.NewDBWriter(db, slog.Default())),
		Embedder:        aiBits.embedder,
		AIEmbedMeta:     aiBits.embedMeta,
		TimeGrains:      aiBits.timeGrains,
		AIProviderStore: aiBits.providerStore,
		ResponseCache:   aiBits.responseCache,
		PoolCache:       poolCache,
		Jobs:            cfg.Jobs,
		DashboardRepo:   dashboardRepo,
	}, nil
}

// openMetadataDB constructs the metadata Postgres pool with the project's
// standard pool limits and connection lifetimes.
func openMetadataDB(ctx context.Context, cfg *config.Config) (*sql.DB, error) {
	if strings.Contains(strings.ToLower(cfg.Metadata.DSN), "sslmode=disable") {
		slog.Warn("metadata DSN has sslmode=disable — set sslmode=require (or higher) for production deployments")
	}
	lims := datasource.DefaultPoolLimits()
	db, err := platformdb.NewPool(ctx, platformdb.Config{
		DSN:             cfg.Metadata.DSN,
		MaxOpenConns:    lims.MaxOpen,
		MaxIdleConns:    lims.MaxIdle,
		ConnMaxLifetime: datasource.DefaultConnMaxLifetime,
		ConnMaxIdleTime: datasource.DefaultConnMaxIdleTime,
	})
	if err != nil {
		return nil, fmt.Errorf("open metadata db: %w", err)
	}
	return db, nil
}

// newDriverRegistry returns a registry populated with every datasource
// driver the engine knows how to talk to. Adding a backend is a one-line
// change here.
func newDriverRegistry() *datasource.Registry {
	reg := datasource.NewRegistry()
	reg.Register(postgres.NewDriver())
	reg.Register(mysql.NewDriver())
	reg.Register(sqlserver.NewDriver())
	reg.Register(clickhouse.NewDriver())
	return reg
}

// aiBundle groups every AI-related dependency returned by setupAI so the
// main constructor stays readable.
type aiBundle struct {
	client        providerpkg.Provider
	queryClient   providerpkg.Provider
	describer     *ai.DescribeService
	embedder      ai.Embedder
	embedMeta     *ai.EmbedMetadataService
	evalRepo      *evalpkg.EvalRepository
	timeGrains    routing.TimeGrainStore
	providerStore *ai.ProviderStore
	responseCache ai.ResponseCache
}

// provideProviderStore builds the DB-backed AI provider/model store. When
// BI_AI_DB_MANAGED is true it seeds an empty database from the BI_AI_* env vars
// and loads the per-purpose defaults into the in-memory cache. Failures are
// non-fatal: the store always retains the env config as a fallback.
func provideProviderStore(ctx context.Context, cfg *config.Config, db *sql.DB, encryptor *security.Encryption) *ai.ProviderStore {
	store := ai.NewProviderStore(db, encryptor, cfg.AI)
	if !cfg.AI.DBManaged {
		return store
	}
	if seeded, err := store.SeedFromEnv(ctx); err != nil {
		slog.Warn("ai provider env seed failed; using env fallback", "error", err)
	} else if seeded {
		slog.Info("seeded ai provider configuration from environment")
	}
	if err := store.RefreshCache(ctx); err != nil {
		slog.Warn("ai provider cache refresh failed; using env fallback", "error", err)
	}
	return store
}

// setupAI wires the LLM provider, optional override provider for NL→query,
// embeddings, descriptor, eval repo, routing, prompt templates, and time
// grain seeds. Failing setup is fatal so misconfigured deployments fail at
// startup, not on the first request.
func setupAI(
	ctx context.Context,
	cfg *config.Config,
	db *sql.DB,
	metaRepo *metadata.Repository,
	reg *datasource.Registry,
	encryptor *security.Encryption,
) (aiBundle, error) {
	providerStore := provideProviderStore(ctx, cfg, db, encryptor)

	baseFallback, err := providerpkg.NewProvider(cfg.AI)
	if err != nil {
		return aiBundle{}, fmt.Errorf("ai provider: %w", err)
	}
	queryFallback := baseFallback
	if cfg.AI.HasQueryOverride() {
		qc, qerr := providerpkg.NewProvider(cfg.AI.EffectiveQueryConfig())
		if qerr != nil {
			return aiBundle{}, fmt.Errorf("ai query provider: %w", qerr)
		}
		queryFallback = qc
		slog.Info("AI query provider overridden",
			"model", cfg.AI.EffectiveQueryConfig().Model,
			"base_url", cfg.AI.EffectiveQueryConfig().BaseURL,
			"describe_model", cfg.AI.Model)
	}

	// Effective config carries DB-managed embedding/translation overrides; when
	// DB management is off it is identical to the env config.
	effectiveCfg := cfg.AI
	client := baseFallback
	queryClient := queryFallback
	describeModel := cfg.AI.Model
	if cfg.AI.DBManaged {
		effectiveCfg = providerStore.EffectiveConfig()
		client = ai.NewPurposeProvider(providerStore, ai.PurposeDescribe, baseFallback)
		queryClient = ai.NewPurposeProvider(providerStore, ai.PurposeQuery, queryFallback)
		if describeCfg, ok := providerStore.ChatConfigForPurpose(ai.PurposeDescribe); ok {
			describeModel = describeCfg.Model
		}
	}

	translator := ai.NewTranslationServiceFromConfig(effectiveCfg)
	describer := ai.NewDescribeService(client, metaRepo, reg, translator, 10, cfg.AI.DescribeMaxCellRunes, cfg.AI.DescribeMaxSampleRows, encryptor).WithModel(describeModel)

	var embedder ai.Embedder
	var embedMeta *ai.EmbedMetadataService
	if effectiveCfg.EmbeddingsConfigured() {
		embedder = providerpkg.NewOpenAIEmbedder(effectiveCfg)
		embedMeta = ai.NewEmbedMetadataService(embedder, metaRepo).
			WithDeniedSchemas(cfg.AI.EmbeddingDenySchemas).
			WithDeniedTables(cfg.AI.EmbeddingDenyTables)
	}

	evalRepo := evalpkg.NewEvalRepository(db)

	if err := routing.InitRouting(cfg.AI.RoutingLexiconPath, cfg.AI.RoutingWeightsPath); err != nil {
		return aiBundle{}, fmt.Errorf("routing config: %w", err)
	}

	prompt.SetPromptTemplateStore(prompt.NewDBPromptTemplateStore(metaRepo))
	if err := prompt.SeedPromptTemplatesFromEmbed(ctx, metaRepo); err != nil {
		return aiBundle{}, fmt.Errorf("seed prompt templates: %w", err)
	}

	timeGrains := routing.NewDBTimeGrainStore(metaRepo)
	if err := routing.SeedTimeGrains(ctx, metaRepo); err != nil {
		return aiBundle{}, fmt.Errorf("seed time grains: %w", err)
	}

	var responseCache ai.ResponseCache
	if cfg.Redis.DSN != "" {
		opt, err := redis.ParseURL(cfg.Redis.DSN)
		if err == nil {
			redisClient := redis.NewClient(opt)
			if pingErr := redisClient.Ping(ctx).Err(); pingErr == nil {
				responseCache = ai.NewRedisResponseCache(redisClient)
				slog.Info("LLM Response Cache initialized with Redis", "dsn", cfg.Redis.DSN)
			} else {
				slog.Warn("LLM Response Cache Redis ping failed; cache disabled", "error", pingErr)
			}
		} else {
			slog.Warn("LLM Response Cache Redis DSN parse failed; cache disabled", "error", err)
		}
	}

	return aiBundle{
		client:        client,
		queryClient:   queryClient,
		describer:     describer,
		embedder:      embedder,
		embedMeta:     embedMeta,
		evalRepo:      evalRepo,
		timeGrains:    timeGrains,
		providerStore: providerStore,
		responseCache: responseCache,
	}, nil
}

// migratePlaintextDSNs scans the datasources table for DSNs that do not look
// encrypted and encrypts them in-place. This runs once on startup.
func migratePlaintextDSNs(ctx context.Context, db *sql.DB, enc *security.Encryption) error {
	rows, err := db.QueryContext(ctx, "SELECT id, dsn_encrypted FROM datasources")
	if err != nil {
		return fmt.Errorf("query datasources: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type pair struct {
		id  string
		dsn string
	}
	var toEncrypt []pair
	for rows.Next() {
		var id, dsn string
		if err := rows.Scan(&id, &dsn); err != nil {
			return fmt.Errorf("scan datasource row: %w", err)
		}
		if !enc.IsEncrypted(dsn) {
			toEncrypt = append(toEncrypt, pair{id: id, dsn: dsn})
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate datasources for DSN migration: %w", err)
	}
	if len(toEncrypt) == 0 {
		return nil
	}

	for _, p := range toEncrypt {
		encrypted, err := enc.Encrypt(p.dsn)
		if err != nil {
			slog.Error("encrypt datasource DSN failed", "id", p.id, "error", err)
			continue
		}
		if _, err := db.ExecContext(ctx, "UPDATE datasources SET dsn_encrypted = $1 WHERE id = $2", encrypted, p.id); err != nil {
			slog.Error("update encrypted DSN failed", "id", p.id, "error", err)
		}
	}
	slog.Info("migrated plaintext DSNs", "count", len(toEncrypt))
	return nil
}

// Close cleans up resources in the reverse order they were acquired:
// in-flight datasource pools first, then HTTP-client-backed AI components,
// finally the metadata DB. Any individual error is collected and returned
// joined so callers see every failure instead of just the first.
func (d *Dependencies) Close() error {
	var errs []error

	if d.AuditLogger != nil {
		if err := d.AuditLogger.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close audit logger: %w", err))
		}
	}

	if d.ResponseCache != nil {
		if err := d.ResponseCache.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close response cache: %w", err))
		}
	}

	if d.PoolCache != nil {
		if err := d.PoolCache.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close datasource pool cache: %w", err))
		}
	}

	// AI providers and the embedder hold http.Client instances; closing them
	// drains idle keepalive sockets so the process exits cleanly. Providers
	// implement an optional `Close() error` — anything else is left alone.
	closeIfPossible := func(label string, v any) {
		closer, ok := v.(interface{ Close() error })
		if !ok || closer == nil {
			return
		}
		if err := closer.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close %s: %w", label, err))
		}
	}
	closeIfPossible("ai client", d.AIClient)
	if d.AIQueryClient != nil && d.AIQueryClient != d.AIClient {
		closeIfPossible("ai query client", d.AIQueryClient)
	}
	closeIfPossible("embedder", d.Embedder)

	if d.MetadataDB != nil {
		if err := d.MetadataDB.Close(); err != nil {
			errs = append(errs, fmt.Errorf("close metadata db: %w", err))
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errors.Join(errs...)
}
