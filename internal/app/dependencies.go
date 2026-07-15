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
	"github.com/biqly/biqly/internal/ai/abtest"
	evalpkg "github.com/biqly/biqly/internal/ai/eval"
	"github.com/biqly/biqly/internal/ai/prompt"
	providerpkg "github.com/biqly/biqly/internal/ai/provider"
	"github.com/biqly/biqly/internal/ai/routing"
	"github.com/biqly/biqly/internal/audit"
	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/core"
	"github.com/biqly/biqly/internal/dashboard"
	"github.com/biqly/biqly/internal/datasource"
	"github.com/biqly/biqly/internal/datasource/clickhouse"
	"github.com/biqly/biqly/internal/datasource/databricks"
	"github.com/biqly/biqly/internal/datasource/mysql"
	"github.com/biqly/biqly/internal/datasource/oracle"
	"github.com/biqly/biqly/internal/datasource/postgres"
	"github.com/biqly/biqly/internal/datasource/snowflake"
	"github.com/biqly/biqly/internal/datasource/sqlite"
	"github.com/biqly/biqly/internal/datasource/sqlserver"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/biqly/biqly/internal/mail"
	"github.com/biqly/biqly/internal/metadata"
	platformdb "github.com/biqly/biqly/internal/platform/db"
	"github.com/biqly/biqly/internal/platform/observability"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/queue"
	"github.com/biqly/biqly/internal/security"
	"github.com/biqly/biqly/internal/semantic"
	"github.com/biqly/biqly/internal/semantic/drift"
	"github.com/biqly/biqly/pkg/catalogclient"
	"github.com/biqly/biqly/pkg/queryclient"
	_ "github.com/jackc/pgx/v5/stdlib" // PostgreSQL driver registration
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
)

// Dependencies holds all application dependencies.
type Dependencies struct {
	Config        *config.Config
	MetadataDB    *sql.DB
	DriverReg     *datasource.Registry
	MetaRepo      *metadata.Repository
	SemanticRepo  *semantic.Repository
	CompositeRepo *semantic.CompositeRepository
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
	// Translator localizes metadata/semantic free-text into the configured
	// target language. nil when the translation layer is not configured.
	Translator  *ai.TranslationService
	Encryptor   *security.Encryption
	EvalRepo    *evalpkg.EvalRepository
	AuditLogger *audit.Logger
	PIIPolicies *core.PIIPolicyService
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
	SpendLimiter    *ai.SpendLimiter
	AIRedis         *redis.Client
	Jobs            config.JobsConfig
	AIJobQueue      queue.AIJobPublisher
	AIJobService    AIJobRunner
	AIJobsHTTP      AIJobsHTTPHandler
	// PoolCache holds *sql.DB pools for external datasources. Closed during
	// Dependencies.Close().
	PoolCache          *datasource.PoolCache
	DashboardRepo      *dashboard.Repository
	DashboardShareRepo *dashboard.ShareRepository
	PublicResolver     *dashboard.PublicResolver
	// PublicShareRedis backs the (future) rate limiter and cache for the
	// anonymous public dashboard endpoint. nil when Redis is not configured
	// or unreachable — callers MUST tolerate nil.
	PublicShareRedis *redis.Client
	DriftRepo        *drift.Repository
	DriftDetector    *drift.Detector
	DriftNotifier    *drift.Notifier
	DriftScheduler   *drift.Scheduler
	ABRouter         *abtest.TrafficRouter
}

// CatalogDeps holds the subset of dependencies needed for the Catalog service
// (datasources, schemas, tables, columns, relations, semantic models).
type CatalogDeps struct {
	Config             *config.Config
	DriverReg          *datasource.Registry
	MetaRepo           *metadata.Repository
	SemanticRepo       *semantic.Repository
	CompositeRepo      *semantic.CompositeRepository
	Encryptor          *security.Encryption
	PoolCache          *datasource.PoolCache
	QueryService       *core.QueryService
	DashboardRepo      *dashboard.Repository
	DashboardShareRepo *dashboard.ShareRepository
	PublicResolver     *dashboard.PublicResolver
	PublicShareRedis   *redis.Client
	AuditLogger        *audit.Logger
	PIIPolicies        *core.PIIPolicyService
	DriftRepo          *drift.Repository
	DriftDetector      *drift.Detector
	DriftNotifier      *drift.Notifier
}

// CatalogDeps returns a structured copy of dependencies for the Catalog subsystem.
func (d *Dependencies) CatalogDeps() *CatalogDeps {
	return &CatalogDeps{
		Config:             d.Config,
		DriverReg:          d.DriverReg,
		MetaRepo:           d.MetaRepo,
		SemanticRepo:       d.SemanticRepo,
		CompositeRepo:      d.CompositeRepo,
		Encryptor:          d.Encryptor,
		PoolCache:          d.PoolCache,
		QueryService:       d.QueryService,
		DashboardRepo:      d.DashboardRepo,
		DashboardShareRepo: d.DashboardShareRepo,
		PublicResolver:     d.PublicResolver,
		PublicShareRedis:   d.PublicShareRedis,
		AuditLogger:        d.AuditLogger,
		PIIPolicies:        d.PIIPolicies,
		DriftRepo:          d.DriftRepo,
		DriftDetector:      d.DriftDetector,
		DriftNotifier:      d.DriftNotifier,
	}
}

// AIDeps holds the subset of dependencies needed for the AI service
// (NL to LogicalQuery, AI jobs, evaluation, descriptions, feedback, few-shot examples).
type AIDeps struct {
	Config          *config.Config
	DriverReg       *datasource.Registry
	MetaRepo        *metadata.Repository
	SemanticRepo    *semantic.Repository
	CompositeRepo   *semantic.CompositeRepository
	Validator       *query.Validator
	QueryService    *core.QueryService
	CatalogClient   *catalogclient.Client
	QueryClient     *queryclient.Client
	AIClient        providerpkg.Provider
	AIQueryClient   providerpkg.Provider
	AIDescriber     *ai.DescribeService
	Translator      *ai.TranslationService
	Encryptor       *security.Encryption
	EvalRepo        *evalpkg.EvalRepository
	AuditLogger     *audit.Logger
	Embedder        ai.Embedder
	AIEmbedMeta     *ai.EmbedMetadataService
	TimeGrains      routing.TimeGrainStore
	AIProviderStore *ai.ProviderStore
	ResponseCache   ai.ResponseCache
	SpendLimiter    *ai.SpendLimiter
	AIRedis         *redis.Client
	Jobs            config.JobsConfig
	AIJobQueue      queue.AIJobPublisher
	AIJobService    AIJobRunner
	AIJobsHTTP      AIJobsHTTPHandler
	PoolCache       *datasource.PoolCache
	Executor        *query.Executor
	ABRouter        *abtest.TrafficRouter
}

// AIDeps returns a structured copy of dependencies for the AI subsystem.
func (d *Dependencies) AIDeps() *AIDeps {
	return &AIDeps{
		Config:          d.Config,
		DriverReg:       d.DriverReg,
		MetaRepo:        d.MetaRepo,
		SemanticRepo:    d.SemanticRepo,
		CompositeRepo:   d.CompositeRepo,
		Validator:       d.Validator,
		QueryService:    d.QueryService,
		CatalogClient:   d.CatalogClient,
		QueryClient:     d.QueryClient,
		AIClient:        d.AIClient,
		AIQueryClient:   d.AIQueryClient,
		AIDescriber:     d.AIDescriber,
		Translator:      d.Translator,
		Encryptor:       d.Encryptor,
		EvalRepo:        d.EvalRepo,
		AuditLogger:     d.AuditLogger,
		Embedder:        d.Embedder,
		AIEmbedMeta:     d.AIEmbedMeta,
		TimeGrains:      d.TimeGrains,
		AIProviderStore: d.AIProviderStore,
		ResponseCache:   d.ResponseCache,
		SpendLimiter:    d.SpendLimiter,
		AIRedis:         d.AIRedis,
		Jobs:            d.Jobs,
		AIJobQueue:      d.AIJobQueue,
		AIJobService:    d.AIJobService,
		AIJobsHTTP:      d.AIJobsHTTP,
		PoolCache:       d.PoolCache,
		Executor:        d.Executor,
		ABRouter:        d.ABRouter,
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
	AuditReader  *audit.Reader
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
		AuditReader:  audit.NewReader(d.MetadataDB),
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
	AdminList(http.ResponseWriter, *http.Request)
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
	compositeRepo := semantic.NewCompositeRepository(db).
		WithResolvedCache(provideCompositeCache(ctx, cfg)).
		WithLimits(provideCompositeLimits(cfg))
	dashboardRepo := dashboard.NewRepository(db)
	dashboardShareRepo := dashboard.NewShareRepository(db)
	publicResolver := dashboard.NewPublicResolver(db)
	publicShareRedis := providePublicShareRedis(ctx, cfg)

	encryptor := provideEncryptor(ctx, db, true)

	validator, executor := provideQueryEngine(cfg)
	poolCache := datasource.NewPoolCache()
	observability.RegisterDBPoolMetrics(prometheus.DefaultRegisterer, "datasource", func() observability.DBPoolSnapshot {
		open, inUse, idle, waitCount, waitDuration := poolCache.AggregatedStats()
		return observability.DBPoolSnapshot{
			OpenConnections: open,
			InUse:           inUse,
			Idle:            idle,
			WaitCount:       waitCount,
			WaitDuration:    waitDuration,
		}
	})
	auditLogger := audit.NewLogger(slog.Default()).WithDBWriter(audit.NewDBWriter(ctx, db, slog.Default()))

	driftRepo := drift.NewRepository(db)
	driftDetector := drift.NewDetector()
	mailClient := mail.NewAPIClient(cfg.Mail.ServiceURL, cfg.Mail.InternalToken, nil)
	driftNotifier := drift.NewNotifier(mailClient, nil)

	piiPolicies := providePIIPolicyService(cfg, metaRepo, auditLogger)
	queryService := core.NewQueryService(&core.QueryServiceDeps{
		Models:      semanticRepo,
		Composites:  compositeRepo,
		Datasources: metaRepo,
		Drivers:     reg,
		Validator:   validator,
		Executor:    executor,
		History:     metaRepo,
		Encryptor:   encryptor,
		Pools:       poolCache,
		PIIPolicies: piiPolicies,
		Audit:       auditLogger,
		Identity:    jwtIdentity,
	})

	aiBits, err := setupAI(ctx, cfg, db, metaRepo, reg, encryptor, poolCache)
	if err != nil {
		return nil, err
	}

	driftScheduler := drift.NewScheduler(
		metaRepo,
		semanticRepo,
		driftDetector,
		driftRepo,
		driftNotifier,
		cfg.Drift.CheckInterval,
		cfg.Mail.FrontendURL,
	)

	return &Dependencies{
		Config:             cfg,
		MetadataDB:         db,
		DriverReg:          reg,
		MetaRepo:           metaRepo,
		SemanticRepo:       semanticRepo,
		CompositeRepo:      compositeRepo,
		Validator:          validator,
		Executor:           executor,
		QueryService:       queryService,
		AIClient:           aiBits.client,
		AIQueryClient:      aiBits.queryClient,
		AIDescriber:        aiBits.describer,
		Translator:         aiBits.translator,
		Encryptor:          encryptor,
		EvalRepo:           aiBits.evalRepo,
		AuditLogger:        auditLogger,
		PIIPolicies:        piiPolicies,
		Embedder:           aiBits.embedder,
		AIEmbedMeta:        aiBits.embedMeta,
		TimeGrains:         aiBits.timeGrains,
		AIProviderStore:    aiBits.providerStore,
		ResponseCache:      aiBits.responseCache,
		SpendLimiter:       aiBits.spendLimiter,
		AIRedis:            aiBits.redis,
		PoolCache:          poolCache,
		Jobs:               cfg.Jobs,
		DashboardRepo:      dashboardRepo,
		DashboardShareRepo: dashboardShareRepo,
		PublicResolver:     publicResolver,
		PublicShareRedis:   publicShareRedis,
		DriftRepo:          driftRepo,
		DriftDetector:      driftDetector,
		DriftNotifier:      driftNotifier,
		DriftScheduler:     driftScheduler,
		ABRouter:           aiBits.abRouter,
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
	observability.RegisterDBPoolMetrics(prometheus.DefaultRegisterer, "metadata", observability.DBPoolStatsFromDB(db))
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
	reg.Register(sqlite.NewDriver())
	reg.Register(snowflake.NewDriver())
	reg.Register(databricks.NewDriver())
	reg.Register(oracle.NewDriver())
	return reg
}

// aiBundle groups every AI-related dependency returned by setupAI so the
// main constructor stays readable.
type aiBundle struct {
	client        providerpkg.Provider
	queryClient   providerpkg.Provider
	describePP    *ai.PurposeProvider
	queryPP       *ai.PurposeProvider
	describer     *ai.DescribeService
	translator    *ai.TranslationService
	embedder      ai.Embedder
	embedMeta     *ai.EmbedMetadataService
	evalRepo      *evalpkg.EvalRepository
	timeGrains    routing.TimeGrainStore
	providerStore *ai.ProviderStore
	responseCache ai.ResponseCache
	spendLimiter  *ai.SpendLimiter
	redis         *redis.Client
	abRouter      *abtest.TrafficRouter
}

// provideProviderStore builds the DB-backed AI provider/model store and loads
// the per-purpose defaults into the in-memory cache. Provider/model selection
// is sourced exclusively from the ai_providers / ai_models tables — there is no
// environment seed or fallback. A refresh failure is non-fatal: the service
// still starts so providers can be configured via the admin API, and requests
// for unconfigured purposes return a clear "no model configured" error.
func provideProviderStore(ctx context.Context, cfg *config.Config, db *sql.DB, encryptor *security.Encryption) *ai.ProviderStore {
	store := ai.NewProviderStore(db, encryptor, &cfg.AI)
	if err := store.RefreshCache(ctx); err != nil {
		slog.Warn("ai provider cache refresh failed; configure providers under Administration → AI Providers", "error", err)
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
	poolCache *datasource.PoolCache,
) (aiBundle, error) {
	providerStore := provideProviderStore(ctx, cfg, db, encryptor)

	// Provider/model selection is DB-only (no env fallback). The per-purpose
	// providers resolve their backend from the ProviderStore on every call and
	// return a clear "no model configured" error when nothing is set.
	effectiveCfg := providerStore.EffectiveConfig()
	describePP := ai.NewPurposeProvider(providerStore, ai.PurposeDescribe, nil, nil)
	queryPP := ai.NewPurposeProvider(providerStore, ai.PurposeQuery, nil, nil)
	client := providerpkg.Provider(describePP)
	queryClient := providerpkg.Provider(queryPP)
	describeModel := ""
	if describeCfg, ok := providerStore.ChatConfigForPurpose(ai.PurposeDescribe); ok {
		describeModel = describeCfg.Connection.Model
	}

	translator := ai.NewTranslationServiceFromProviderStore(providerStore, effectiveCfg)
	describer := ai.NewDescribeService(client, metaRepo, reg, translator, 10, cfg.AI.Describe.MaxCellRunes, cfg.AI.Describe.MaxSampleRows, encryptor).
		WithModel(describeModel).
		WithPoolCache(poolCache)

	var embedder ai.Embedder
	var embedMeta *ai.EmbedMetadataService
	if effectiveCfg.ResolvedEmbedding().Configured() {
		embedder = providerpkg.NewOpenAIEmbedder(effectiveCfg)
		embedMeta = ai.NewEmbedMetadataService(embedder, metaRepo).
			WithDeniedSchemas(cfg.AI.Embedding.DenySchemas).
			WithDeniedTables(cfg.AI.Embedding.DenyTables)
	}

	evalRepo := evalpkg.NewEvalRepository(db)

	if err := routing.InitRouting(cfg.AI.Routing.LexiconPath, cfg.AI.Routing.WeightsPath); err != nil {
		return aiBundle{}, fmt.Errorf("routing config: %w", err)
	}

	prompt.SetPromptTemplateStore(prompt.NewDBPromptTemplateStore(metaRepo))
	abRepo := abtest.NewRepository(db)
	abRouter := abtest.NewTrafficRouter(abRepo)
	prompt.SetVariantResolver(abRouter)

	if err := prompt.SeedPromptTemplatesFromEmbed(ctx, metaRepo); err != nil {
		return aiBundle{}, fmt.Errorf("seed prompt templates: %w", err)
	}

	timeGrains := routing.NewDBTimeGrainStore(metaRepo)
	if err := routing.SeedTimeGrains(ctx, metaRepo); err != nil {
		return aiBundle{}, fmt.Errorf("seed time grains: %w", err)
	}

	if err := wireNLRuntime(ctx, metaRepo); err != nil {
		return aiBundle{}, err
	}

	aiRedis := newAIRedisClient(ctx, cfg.Redis.DSN)
	var responseCache ai.ResponseCache
	if aiRedis != nil {
		responseCache = ai.NewRedisResponseCache(aiRedis)
	}
	spendLimiter := ai.NewSpendLimiter(aiRedis, cfg.AI.Generation.WorkspaceDailyTokenBudget)

	return aiBundle{
		client:        client,
		queryClient:   queryClient,
		describePP:    describePP,
		queryPP:       queryPP,
		describer:     describer,
		translator:    translator,
		embedder:      embedder,
		embedMeta:     embedMeta,
		evalRepo:      evalRepo,
		timeGrains:    timeGrains,
		providerStore: providerStore,
		responseCache: responseCache,
		spendLimiter:  spendLimiter,
		redis:         aiRedis,
		abRouter:      abRouter,
	}, nil
}

// newAIRedisClient builds the shared Redis client for AI-side stores (LLM
// response cache and the per-workspace spend limiter). Returns nil when the DSN
// is empty/invalid or the server is unreachable, so callers degrade gracefully.
func newAIRedisClient(ctx context.Context, dsn string) *redis.Client {
	if dsn == "" {
		return nil
	}
	opt, err := redis.ParseURL(dsn)
	if err != nil {
		slog.Warn("AI Redis DSN parse failed; response cache and spend limiter disabled", "error", err)
		return nil
	}
	redisClient := redis.NewClient(opt)
	if instrErr := observability.InstrumentRedis(redisClient, "biqly-dragonfly"); instrErr != nil {
		slog.Warn("AI Redis tracing instrumentation failed", "error", instrErr)
	}
	if pingErr := redisClient.Ping(ctx).Err(); pingErr != nil {
		slog.Warn("AI Redis ping failed; response cache and spend limiter disabled", "error", pingErr)
		return nil
	}
	slog.Info("AI Redis initialized", "addr", opt.Addr)
	return redisClient
}

// WireAIUserResolver attaches per-user model selection when auth is enabled.
func (d *Dependencies) WireAIUserResolver(auth *bimw.AuthClient) {
	if auth == nil || !d.Config.Auth.Enabled || d.AIProviderStore == nil {
		return
	}
	resolver := NewAIUserConfigResolver(d.AIProviderStore, auth)
	if d.AIClient != nil {
		if pp, ok := d.AIClient.(*ai.PurposeProvider); ok {
			pp.SetResolver(resolver)
		}
	}
	if d.AIQueryClient != nil {
		if pp, ok := d.AIQueryClient.(*ai.PurposeProvider); ok {
			pp.SetResolver(resolver)
		}
	}
}

// WireDriftNotifier attaches the auth client to the drift notifier.
func (d *Dependencies) WireDriftNotifier(auth *bimw.AuthClient) {
	if d.DriftNotifier != nil {
		d.DriftNotifier.SetAuthClient(auth)
	}
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

	if d.DriftScheduler != nil {
		d.DriftScheduler.Stop()
	}

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
