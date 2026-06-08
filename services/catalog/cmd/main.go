// Package main runs the standalone Catalog Service.
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
	"github.com/biqly/biqly/internal/platform/observability"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "service", "catalog", "error", err)
		os.Exit(1)
	}
	slog.SetDefault(logger.New(logger.Config{
		Level: logger.LevelFromString(cfg.Logging.Level),
		JSON:  logger.JSONFromString(cfg.Logging.Format),
	}))

	ctx := context.Background()
	shutdownTracing, tracErr := observability.SetupTracing(ctx, "catalog")
	if tracErr != nil {
		slog.Warn("tracing setup failed, continuing without traces", "error", tracErr)
	}
	defer func() {
		if err := shutdownTracing(context.Background()); err != nil {
			slog.Warn("trace provider shutdown error", "error", err)
		}
	}()
	deps, err := app.NewCatalogDependencies(ctx, cfg)
	if err != nil {
		slog.Error("failed to initialize catalog dependencies", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := deps.Close(); err != nil {
			slog.Error("failed to close catalog dependencies", "error", err)
		}
	}()

	router := httprouter.CatalogRouter(deps)
	srv := &http.Server{
		Addr:         cfg.HTTPAddr(),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		slog.Info("starting Catalog Service", "addr", cfg.HTTPAddr())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("catalog server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down Catalog Service")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("catalog server forced to shutdown", "error", err)
		os.Exit(1)
	}
	slog.Info("Catalog Service stopped gracefully")
}
