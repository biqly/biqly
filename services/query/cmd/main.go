// Package main runs the standalone Query Engine.
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
	"github.com/biqly/biqly/internal/platform/logger"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "service", "query", "error", err)
		os.Exit(1)
	}
	slog.SetDefault(logger.New(logger.Config{
		Level: logger.LevelFromString(cfg.Logging.Level),
		JSON:  logger.JSONFromString(cfg.Logging.Format),
	}))

	ctx := context.Background()
	deps, err := app.NewQueryDependencies(ctx, cfg)
	if err != nil {
		slog.Error("failed to initialize query dependencies", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := deps.Close(); err != nil {
			slog.Error("failed to close query dependencies", "error", err)
		}
	}()

	router := httprouter.QueryRouter(deps)
	srv := &http.Server{
		Addr:         cfg.HTTPAddr(),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: cfg.MaxQueryRuntime() + 15*time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		slog.Info("starting Query Engine", "addr", cfg.HTTPAddr())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("query server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down Query Engine")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("query server forced to shutdown", "error", err)
		os.Exit(1)
	}
	slog.Info("Query Engine stopped gracefully")
}
