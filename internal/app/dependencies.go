// Package app wires together application dependencies.
package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

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
	platformdb "github.com/biqly/biqly/internal/platform/db"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/security"
	"github.com/biqly/biqly/internal/semantic"
	"github.com/biqly/biqly/pkg/catalogclient"
	"github.com/biqly/biqly/pkg/queryclient"
	"github.com/biqly/biqly/internal/queue"
	_ "github.com/jackc/pgx/v5/stdlib" // PostgreSQL driver registration
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
	AIClient      ai.Provider
	// AIQueryClient powers the NL→LogicalQuery path. Aliases AIClient when no
	// BI_AI_QUERY_* overrides are set; otherwise points at a separate provider
	// (typically a smarter model) so describe/metadata work can keep using the
	// cheaper local model on AIClient.
	AIQueryClient ai.Provider
	AIDescriber   *ai.DescribeService
	Encryptor     *security.Encryption
	EvalRepo      *ai.EvalRepository
	AuditLogger   *audit.Logger
	// Embedder is the embeddings provider used for vector-based table
	// retrieval. nil when no API key is configured — callers MUST tolerate
	// nil (the table router falls back to keyword scoring).
	Embedder    ai.Embedder
	AIEmbedMeta *ai.EmbedMetadataService
	TimeGrains   ai.TimeGrainStore
	Jobs         config.JobsConfig
	AIJobQueue   queue.AIJobPublisher
	AIJobService AIJobRunner
	AIJobsHTTP   AIJobsHTTPHandler
	// PoolCache holds *sql.DB pools for external datasources. Closed during
	// Dependencies.Close().
	PoolCache *datasource.PoolCache
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
}

// NewDependencies wires up all dependencies.
func NewDependencies(ctx context.Context, cfg *config.Config) (*Dependencies, error) {
	// Connect to metadata database. Uses the platform/db helper so pool
	// limits and connection lifetimes are configured in one place.
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

	// Setup driver registry
	reg := datasource.NewRegistry()
	reg.Register(postgres.NewDriver())
	reg.Register(mysql.NewDriver())
	reg.Register(sqlserver.NewDriver())
	reg.Register(clickhouse.NewDriver())

	// Setup repositories
	metaRepo := metadata.NewRepository(db)
	semanticRepo := semantic.NewRepository(db)

	// Encryption for sensitive fields (DSNs, etc.). Falls back to nil if
	// BI_ENCRYPTION_KEY is not set — datasource handler tolerates nil and
	// stores plaintext with a warning.
	var encryptor *security.Encryption
	enc, err := security.NewEncryption()
	if err != nil {
		slog.Warn("encryption disabled; DSNs will be stored in plaintext", "detail", err)
	} else {
		encryptor = enc
		// Migrate any existing plaintext DSNs to encrypted format on startup.
		if migrateErr := migratePlaintextDSNs(ctx, db, encryptor); migrateErr != nil {
			slog.Warn("failed to migrate existing plaintext DSNs to encrypted format", "error", migrateErr)
		}
	}

	// Setup query components
	validator := query.NewValidator(cfg.Query.MaxRows)
	executor := query.NewExecutor(cfg.Query.MaxRows, cfg.QueryTimeout())
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

	// AI provider (OpenAI / Anthropic / OpenAI-compatible) + metadata describe
	// service. Failing here is fatal so misconfigured deployments stop early
	// instead of crashing on first request.
	aiClient, err := ai.NewProvider(cfg.AI)
	if err != nil {
		return nil, fmt.Errorf("ai provider: %w", err)
	}
	// Dedicated provider for NL→LogicalQuery when BI_AI_QUERY_* overrides are
	// set; otherwise reuse the base client. Fatal on misconfiguration so
	// startup fails fast.
	aiQueryClient := aiClient
	if cfg.AI.HasQueryOverride() {
		queryClient, err := ai.NewProvider(cfg.AI.EffectiveQueryConfig())
		if err != nil {
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

	// Embeddings are optional: BI_AI_EMBEDDING_MODEL plus resolvable URL and API key.
	var embedder ai.Embedder
	var embedMeta *ai.EmbedMetadataService
	if cfg.AI.EmbeddingsConfigured() {
		embedder = ai.NewOpenAIEmbedder(cfg.AI)
		embedMeta = ai.NewEmbedMetadataService(embedder, metaRepo)
	}

	// Eval repository for persistent golden test results and regression reports.
	evalRepo := ai.NewEvalRepository(db)

	if err := ai.InitRouting(cfg.AI.RoutingLexiconPath, cfg.AI.RoutingWeightsPath); err != nil {
		return nil, fmt.Errorf("routing config: %w", err)
	}

	ai.SetPromptTemplateStore(ai.NewDBPromptTemplateStore(metaRepo))
	if err := ai.SeedPromptTemplatesFromEmbed(ctx, metaRepo); err != nil {
		return nil, fmt.Errorf("seed prompt templates: %w", err)
	}

	timeGrainsStore := ai.NewDBTimeGrainStore(metaRepo)
	if err := ai.SeedTimeGrains(ctx, metaRepo); err != nil {
		return nil, fmt.Errorf("seed time grains: %w", err)
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
		AIClient:      aiClient,
		AIQueryClient: aiQueryClient,
		AIDescriber:   describer,
		Encryptor:     encryptor,
		EvalRepo:      evalRepo,
		AuditLogger:   audit.NewLogger(slog.Default()),
		Embedder:      embedder,
		AIEmbedMeta:   embedMeta,
		TimeGrains:    timeGrainsStore,
		PoolCache:     poolCache,
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
