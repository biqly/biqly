package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/biqly/biqly/internal/config"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	slog.Info("worker started",
		"query_timeout", cfg.Query.TimeoutSeconds,
		"max_rows", cfg.Query.MaxRows,
	)

	// TODO: Add background job processing
	// - Stale cache cleanup
	// - Query history archival
	// - Datasource health checks
	// - AI query history persistence

	// Placeholder: periodic health check ticker
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case <-ticker.C:
			slog.Info("worker heartbeat")
		case <-ctx.Done():
			slog.Info("worker shutting down")
			return
		case <-quit:
			slog.Info("received shutdown signal")
			cancel()
		}
	}
}
