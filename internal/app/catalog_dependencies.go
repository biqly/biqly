package app

import (
	"context"
	"log/slog"

	evalpkg "github.com/biqly/biqly/internal/ai/eval"
	"github.com/biqly/biqly/internal/audit"
	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/dashboard"
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

	metaRepo, semanticRepo := provideRepositories(db)
	compositeRepo := semantic.NewCompositeRepository(db).
		WithResolvedCache(provideCompositeCache(ctx, cfg)).
		WithLimits(provideCompositeLimits(cfg))
	dashboardRepo := dashboard.NewRepository(db)

	encryptor := provideEncryptor(ctx, db, true)

	return &Dependencies{
		Config:        cfg,
		MetadataDB:    db,
		DriverReg:     reg,
		MetaRepo:      metaRepo,
		SemanticRepo:  semanticRepo,
		CompositeRepo: compositeRepo,
		Encryptor:     encryptor,
		EvalRepo:      evalpkg.NewEvalRepository(db),
		AuditLogger:   audit.NewLogger(slog.Default()).WithDBWriter(audit.NewDBWriter(db, slog.Default())),
		DashboardRepo: dashboardRepo,
	}, nil
}
