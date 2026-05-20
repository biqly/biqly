// Package main is the background worker process for the BI query engine.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/http/handlers"
	"github.com/biqly/biqly/internal/platform/logger"
	"github.com/biqly/biqly/internal/queue"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}
	slog.SetDefault(logger.New(logger.Config{
		Level: logger.LevelFromString(cfg.Logging.Level),
		JSON:  logger.JSONFromString(cfg.Logging.Format),
	}))

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if cfg.NATS.URL == "" {
		slog.Error("BI_NATS_URL is required for worker")
		os.Exit(1)
	}

	deps, err := app.NewAIDependencies(ctx, cfg)
	if err != nil {
		slog.Error("failed to initialize dependencies", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := deps.Close(); err != nil {
			slog.Error("failed to close dependencies", "error", err)
		}
		if q, ok := deps.AIJobQueue.(interface{ Close() error }); ok {
			_ = q.Close()
		}
	}()

	pub, err := app.NewAIJobQueue(cfg)
	if err != nil {
		slog.Error("failed to create ai job queue", "error", err)
		os.Exit(1)
	}
	deps.AIJobQueue = pub
	if _, ok := pub.(queue.AIJobConsumer); !ok {
		slog.Error("job queue does not support consume")
		os.Exit(1)
	}
	aiHandler := handlers.NewAIHandler(deps)
	jobSvc := handlers.NewAIJobService(deps.MetaRepo, pub, aiHandler)

	slog.Info("worker started", "nats_url", cfg.NATS.URL, "group", cfg.NATS.ConsumerGroup)

	go func() {
		if err := jobSvc.StartConsumer(ctx, cfg.NATS.ConsumerGroup); err != nil && ctx.Err() == nil {
			slog.Error("ai job consumer stopped", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	cancel()
	slog.Info("worker shutting down")
}
