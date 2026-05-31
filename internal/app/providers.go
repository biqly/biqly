package app

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/metadata"
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
