package app

import (
	"context"
	"log/slog"

	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/audit"
	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/security"
	"github.com/biqly/biqly/internal/semantic"
)

// NewCatalogDependencies wires only the Catalog Service dependency graph.
// Unlike NewDependencies it deliberately does not construct AI providers,
// query validators/executors, or user-query services.
func NewCatalogDependencies(ctx context.Context, cfg *config.Config) (*Dependencies, error) {
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
