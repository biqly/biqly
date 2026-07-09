// Package main runs the standalone AI Service.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/config"
	httprouter "github.com/biqly/biqly/internal/http"
	"github.com/biqly/biqly/internal/http/handlers"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/biqly/biqly/internal/platform/logger"
	"github.com/biqly/biqly/internal/platform/observability"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "service", "ai", "error", err)
		os.Exit(1)
	}
	slog.SetDefault(logger.New(logger.Config{
		Level: logger.LevelFromString(cfg.Logging.Level),
		JSON:  logger.JSONFromString(cfg.Logging.Format),
	}))

	ctx := context.Background()
	shutdownTracing, tracErr := observability.SetupTracing(ctx, "ai")
	if tracErr != nil {
		slog.Warn("tracing setup failed, continuing without traces", "error", tracErr)
	}
	defer func() {
		if err := shutdownTracing(context.Background()); err != nil {
			slog.Warn("trace provider shutdown error", "error", err)
		}
	}()
	shutdownLogExport, logExpErr := observability.SetupLogExport(ctx, "ai")
	if logExpErr != nil {
		slog.Warn("log export setup failed, continuing with stdout only", "error", logExpErr)
	}
	defer func() {
		if err := shutdownLogExport(context.Background()); err != nil {
			slog.Warn("log provider shutdown error", "error", err)
		}
	}()

	deps, err := app.NewAIDependencies(ctx, cfg)
	if err != nil {
		slog.Error("failed to initialize ai dependencies", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := deps.Close(); err != nil {
			slog.Error("failed to close ai dependencies", "error", err)
		}
	}()

	authClient := httprouter.NewAuthClient(deps)
	deps.WireAIUserResolver(authClient)

	if err := wireAIJobs(cfg, deps, authClient); err != nil {
		slog.Error("failed to initialize ai jobs", "error", err)
		os.Exit(1)
	}

	router := httprouter.AIRouter(deps)
	srv := &http.Server{
		Addr:         cfg.HTTPAddr(),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: cfg.HTTPWriteTimeout(),
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		slog.Info("starting AI Service", "addr", cfg.HTTPAddr())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("ai server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down AI Service")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("ai server forced to shutdown", "error", err)
		os.Exit(1)
	}
	slog.Info("AI Service stopped gracefully")
}

func wireAIJobs(cfg *config.Config, deps *app.Dependencies, authClient *bimw.AuthClient) error {
	if !deps.Jobs.Enabled {
		return nil
	}
	pub, err := app.NewAIJobQueue(cfg)
	if err != nil {
		return fmt.Errorf("create ai job queue: %w", err)
	}
	deps.AIJobQueue = pub
	aiHandler := handlers.NewAIHandler(deps.AIDeps())
	aiHandler.SetAuthClient(authClient)
	aiHandler.SetAIMetricsRecorder(httprouter.GetMetrics())
	jobSvc := handlers.NewAIJobService(deps.MetaRepo, pub, aiHandler)
	deps.AIJobService = jobSvc
	deps.AIJobsHTTP = handlers.NewAIJobsHandler(jobSvc, deps.AuditLogger)

	if !cfg.Jobs.ConsumerEnabled {
		slog.Info("ai job consumer disabled", "nats_url", cfg.NATS.URL)
		return nil
	}
	consumerCtx := context.Background()
	if err := jobSvc.StartConsumer(consumerCtx, cfg.NATS.ConsumerGroup); err != nil {
		return fmt.Errorf("start ai job consumer: %w", err)
	}
	slog.Info("ai job consumer started", "nats_url", cfg.NATS.URL)
	return nil
}
