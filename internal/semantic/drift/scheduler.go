package drift

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/semantic"
)

// Scheduler runs periodic schema drift checks.
type Scheduler struct {
	metaRepo     *metadata.Repository
	semanticRepo *semantic.Repository
	detector     *Detector
	repo         *Repository
	notifier     *Notifier
	interval     time.Duration
	wg           sync.WaitGroup
	cancel       context.CancelFunc
	frontendURL  string
}

// NewScheduler constructs a new Scheduler.
func NewScheduler(
	metaRepo *metadata.Repository,
	semanticRepo *semantic.Repository,
	detector *Detector,
	repo *Repository,
	notifier *Notifier,
	interval time.Duration,
	frontendURL string,
) *Scheduler {
	if interval <= 0 {
		interval = 6 * time.Hour
	}
	return &Scheduler{
		metaRepo:     metaRepo,
		semanticRepo: semanticRepo,
		detector:     detector,
		repo:         repo,
		notifier:     notifier,
		interval:     interval,
		frontendURL:  frontendURL,
	}
}

// Start spawns the background drift recheck worker.
func (s *Scheduler) Start(ctx context.Context) {
	runCtx, cancel := context.WithCancel(ctx)
	s.cancel = cancel

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		slog.Info("schema drift check scheduler started", "interval", s.interval)

		ticker := time.NewTicker(s.interval)
		defer ticker.Stop()

		for {
			select {
			case <-runCtx.Done():
				slog.Info("schema drift check scheduler stopping")
				return
			case <-ticker.C:
				s.RunCheck(runCtx)
			}
		}
	}()
}

// Stop stops the background worker gracefully.
func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
	slog.Info("schema drift check scheduler stopped")
}

// RunCheck executes the drift check across all datasources.
func (s *Scheduler) RunCheck(ctx context.Context) {
	slog.Info("running scheduled schema drift check")

	datasources, err := s.metaRepo.ListDatasources(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "scheduler failed to list datasources", "error", err)
		return
	}

	for _, ds := range datasources {
		if !ds.IsActive {
			continue
		}

		if ctx.Err() != nil {
			return
		}

		s.checkDatasourceDrift(ctx, ds.ID)
	}
}

func (s *Scheduler) checkDatasourceDrift(ctx context.Context, dsID string) {
	columns, err := s.metaRepo.ListColumns(ctx, dsID, "", "")
	if err != nil {
		slog.ErrorContext(ctx, "scheduler failed to list columns", "datasource_id", dsID, "error", err)
		return
	}

	tables, err := s.metaRepo.ListTables(ctx, dsID, "")
	if err != nil {
		slog.ErrorContext(ctx, "scheduler failed to list tables", "datasource_id", dsID, "error", err)
		return
	}

	models, err := s.semanticRepo.ListModels(ctx, dsID)
	if err != nil {
		slog.ErrorContext(ctx, "scheduler failed to list models", "datasource_id", dsID, "error", err)
		return
	}

	for _, model := range models {
		if !model.IsActive {
			continue
		}

		fullModel, err := s.semanticRepo.GetFullModel(ctx, model.ID)
		if err != nil {
			slog.ErrorContext(ctx, "scheduler failed to fetch full model", "model_id", model.ID, "error", err)
			continue
		}

		report, err := s.detector.Compare(ctx, *fullModel, columns, tables)
		if err != nil {
			slog.ErrorContext(ctx, "scheduler drift comparison failed", "model_id", model.ID, "error", err)
			continue
		}

		if report == nil || len(report.Drifts) == 0 {
			continue
		}

		latest, err := s.repo.GetLatestByModel(ctx, model.ID)
		if errors.Is(err, ErrNoDriftReport) {
			latest = nil
		} else if err != nil {
			slog.ErrorContext(ctx, "scheduler failed to fetch latest drift report", "model_id", model.ID, "error", err)
			continue
		}

		if latest != nil && !latest.Resolved && s.driftsMatch(latest.Drifts, report.Drifts) {
			continue
		}

		if err := s.repo.InsertReport(ctx, report); err != nil {
			slog.ErrorContext(ctx, "scheduler failed to insert drift report", "model_id", model.ID, "error", err)
			continue
		}

		if report.Severity == SeverityCritical || report.Severity == SeverityWarning {
			go func(r *DriftReport, mName string, creator *string, parent context.Context) {
				notifyCtx, cancel := context.WithTimeout(context.WithoutCancel(parent), 15*time.Second)
				defer cancel()

				if err := s.notifier.NotifyOwner(notifyCtx, r, mName, creator, s.frontendURL); err != nil {
					slog.Error("scheduler failed to notify owner about schema drift", "model_id", r.ModelID, "error", err)
				}
			}(report, fullModel.Name, fullModel.CreatedBy, ctx)
		}
	}
}

func (*Scheduler) driftsMatch(a, b []DriftItem) bool {
	if len(a) != len(b) {
		return false
	}
	aMap := make(map[string]bool)
	for _, item := range a {
		key := fmt.Sprintf("%s:%s:%s", item.Type, item.Field, item.ColumnRef)
		aMap[key] = true
	}
	for _, item := range b {
		key := fmt.Sprintf("%s:%s:%s", item.Type, item.Field, item.ColumnRef)
		if !aMap[key] {
			return false
		}
	}
	return true
}
