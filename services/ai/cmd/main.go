// Package main runs the standalone AI Service.
package main

import (
	"context"
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
	"github.com/biqly/biqly/internal/platform/logger"
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

	if deps.Jobs.Enabled {
		pub, qerr := app.NewAIJobQueue(cfg)
		if qerr != nil {
			slog.Error("failed to create ai job queue", "error", qerr)
			os.Exit(1)
		}
		deps.AIJobQueue = pub
		aiHandler := handlers.NewAIHandler(deps)
		jobSvc := handlers.NewAIJobService(deps.MetaRepo, pub, aiHandler)
		deps.AIJobService = jobSvc
		deps.AIJobsHTTP = handlers.NewAIJobsHandler(jobSvc)
		consumerCtx := context.Background()
		if err := jobSvc.StartConsumer(consumerCtx, cfg.NATS.ConsumerGroup); err != nil {
			slog.Error("failed to start ai job consumer", "error", err)
			os.Exit(1)
		}
		slog.Info("ai job consumer started", "nats_url", cfg.NATS.URL)
	}

	router := httprouter.AIRouter(deps)
	srv := &http.Server{
		Addr:         cfg.HTTPAddr(),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: cfg.MaxQueryRuntime() + 15*time.Second,
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
