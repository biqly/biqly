package app

import (
	"context"
	"log/slog"

	evalpkg "github.com/biqly/biqly/internal/ai/eval"
	"github.com/biqly/biqly/internal/audit"
	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/dashboard"
	"github.com/biqly/biqly/internal/mail"
	"github.com/biqly/biqly/internal/semantic"
	"github.com/biqly/biqly/internal/semantic/drift"
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

	driftRepo := drift.NewRepository(db)
	driftDetector := drift.NewDetector()
	mailClient := mail.NewAPIClient(cfg.Mail.ServiceURL, cfg.Mail.InternalToken, nil)
	driftNotifier := drift.NewNotifier(mailClient, nil)

	return &Dependencies{
		Config:        cfg,
		MetadataDB:    db,
		DriverReg:     reg,
		MetaRepo:      metaRepo,
		SemanticRepo:  semanticRepo,
		CompositeRepo: compositeRepo,
		Encryptor:     encryptor,
		EvalRepo:      evalpkg.NewEvalRepository(db),
		AuditLogger:   audit.NewLogger(slog.Default()).WithDBWriter(audit.NewDBWriter(ctx, db, slog.Default())),
		DashboardRepo: dashboardRepo,
		DriftRepo:     driftRepo,
		DriftDetector: driftDetector,
		DriftNotifier: driftNotifier,
	}, nil
}
