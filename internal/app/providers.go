package app

import (
	"context"
	"database/sql"
	"log/slog"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/platform/observability"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/security"
	"github.com/biqly/biqly/internal/semantic"
)

// providers.go collects the small constructor ("provider") functions the
// per-service Dependencies builders compose from. This formalizes the
// project's manual constructor-injection approach (roadmap task 4.4): each
// provider owns exactly one concern, takes its inputs as explicit parameters,
// and returns constructed components — so the wiring graph stays visible in
// plain Go and unit-testable, without a reflection-based (dig) or codegen
// (wire) DI framework. The NewDependencies / NewAIDependencies /
// NewQueryDependencies / NewCatalogDependencies constructors are the
// composition roots that assemble these providers for each binary.

// provideRepositories builds the metadata and semantic repositories that share
// the metadata Postgres pool.
func provideRepositories(db *sql.DB) (*metadata.Repository, *semantic.Repository) {
	return metadata.NewRepository(db), semantic.NewRepository(db)
}

// provideEncryptor loads the BI_ENCRYPTION_KEY-backed Encryption helper. It
// returns nil (after warning) when no key is configured; every caller tolerates
// nil and falls back to plaintext DSN storage. When migrate is true, existing
// plaintext DSNs are re-encrypted in place once on startup.
func provideEncryptor(ctx context.Context, db *sql.DB, migrate bool) *security.Encryption {
	enc, err := security.NewEncryption()
	if err != nil {
		slog.Warn("encryption disabled; datasource DSNs will be stored/read as plaintext", "detail", err)
		return nil
	}
	if migrate {
		if migrateErr := migratePlaintextDSNs(ctx, db, enc); migrateErr != nil {
			slog.Warn("failed to migrate existing plaintext DSNs to encrypted format", "error", migrateErr)
		}
	}
	return enc
}

// provideQueryEngine builds the semantic validator and the safe SQL executor
// from the configured row limit and query timeout.
func provideQueryEngine(cfg *config.Config) (*query.Validator, *query.Executor) {
	return query.NewValidator(cfg.Query.MaxRows),
		query.NewExecutor(cfg.Query.MaxRows, cfg.QueryTimeout())
}

// provideCompositeCache builds the Redis-backed cache for published resolved
// composite models. It returns nil (caching disabled) when Redis is not
// configured or unreachable, mirroring the LLM response cache wiring.
func provideCompositeCache(ctx context.Context, cfg *config.Config) semantic.ResolvedCompositeCache {
	if cfg.Redis.DSN == "" {
		return nil
	}
	opt, err := redis.ParseURL(cfg.Redis.DSN)
	if err != nil {
		slog.Warn("composite cache Redis DSN parse failed; cache disabled", "error", err)
		return nil
	}
	client := redis.NewClient(opt)
	if instrErr := observability.InstrumentRedis(client, "biqly-dragonfly"); instrErr != nil {
		slog.Warn("composite cache Redis tracing instrumentation failed", "error", instrErr)
	}
	if pingErr := client.Ping(ctx).Err(); pingErr != nil {
		slog.Warn("composite cache Redis ping failed; cache disabled", "error", pingErr)
		return nil
	}
	slog.Info("Composite model cache initialized with Redis")
	return semantic.NewRedisCompositeCache(client, time.Hour)
}

// providePublicShareRedis builds the shared Redis client backing the public
// dashboard share rate limiter and cache. It returns nil (feature degrades to
// disabled) when Redis is not configured or unreachable, mirroring
// provideCompositeCache's fallback behavior.
func providePublicShareRedis(ctx context.Context, cfg *config.Config) *redis.Client {
	if cfg.Redis.DSN == "" {
		return nil
	}
	opt, err := redis.ParseURL(cfg.Redis.DSN)
	if err != nil {
		slog.Warn("public share Redis DSN parse failed; rate limiter and cache disabled", "error", err)
		return nil
	}
	client := redis.NewClient(opt)
	if instrErr := observability.InstrumentRedis(client, "biqly-dragonfly"); instrErr != nil {
		slog.Warn("public share Redis tracing instrumentation failed", "error", instrErr)
	}
	if pingErr := client.Ping(ctx).Err(); pingErr != nil {
		slog.Warn("public share Redis ping failed; rate limiter and cache disabled", "error", pingErr)
		return nil
	}
	slog.Info("Public share Redis initialized")
	return client
}

func provideCompositeLimits(cfg *config.Config) semantic.CompositeLimits {
	return semantic.CompositeLimits{
		MaxComponents:   cfg.Composite.MaxComponents,
		MaxCrossJoins:   cfg.Composite.MaxCrossJoins,
		MaxMergedFields: cfg.Composite.MaxMergedFields,
	}
}
