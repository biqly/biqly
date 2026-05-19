package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"

	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/audit"
	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/datasource"
	"github.com/biqly/biqly/internal/datasource/clickhouse"
	"github.com/biqly/biqly/internal/datasource/mysql"
	"github.com/biqly/biqly/internal/datasource/postgres"
	"github.com/biqly/biqly/internal/datasource/sqlserver"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/security"
	"github.com/biqly/biqly/internal/semantic"
)

// NewCatalogDependencies wires only the Catalog Service dependency graph.
// Unlike NewDependencies it deliberately does not construct AI providers,
// query validators/executors, or user-query services.
func NewCatalogDependencies(ctx context.Context, cfg *config.Config) (*Dependencies, error) {
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
		slog.Warn("encryption disabled; DSNs will be stored in plaintext", "detail", err)
	} else {
		encryptor = enc
		if migrateErr := migratePlaintextDSNs(ctx, db, encryptor); migrateErr != nil {
			slog.Warn("failed to migrate existing plaintext DSNs to encrypted format", "error", migrateErr)
		}
	}

	return &Dependencies{
		Config:       cfg,
		MetadataDB:   db,
		DriverReg:    reg,
		MetaRepo:     metaRepo,
		SemanticRepo: semanticRepo,
		Encryptor:    encryptor,
		EvalRepo:     ai.NewEvalRepository(db),
		AuditLogger:  audit.NewLogger(slog.Default()),
	}, nil
}
