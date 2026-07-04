// Package main runs the standalone MCP service: a thin governed gateway that
// exposes biqly's query capability over the Model Context Protocol. It owns no
// database — every tool call is proxied to the internal API (BI_API_SERVICE_URL)
// where authentication, per-datasource access, RLS/PII masking, spend caps, and
// audit are enforced. See internal/http/mcp_router.go.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/biqly/biqly/internal/config"
	httprouter "github.com/biqly/biqly/internal/http"
	"github.com/biqly/biqly/internal/platform/logger"
	"github.com/biqly/biqly/internal/platform/observability"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "service", "mcp", "error", err)
		os.Exit(1)
	}
	slog.SetDefault(logger.New(logger.Config{
		Level: logger.LevelFromString(cfg.Logging.Level),
		JSON:  logger.JSONFromString(cfg.Logging.Format),
	}))

	if cfg.Services.APIURL == "" {
		slog.Error("BI_API_SERVICE_URL is required for the MCP service (upstream API gateway)")
		os.Exit(1)
	}

	ctx := context.Background()
	shutdownTracing, tracErr := observability.SetupTracing(ctx, "mcp")
	if tracErr != nil {
		slog.Warn("tracing setup failed, continuing without traces", "error", tracErr)
	}
	defer func() {
		if err := shutdownTracing(context.Background()); err != nil {
			slog.Warn("trace provider shutdown error", "error", err)
		}
	}()
	shutdownLogExport, logExpErr := observability.SetupLogExport(ctx, "mcp")
	if logExpErr != nil {
		slog.Warn("log export setup failed, continuing with stdout only", "error", logExpErr)
	}
	defer func() {
		if err := shutdownLogExport(context.Background()); err != nil {
			slog.Warn("log provider shutdown error", "error", err)
		}
	}()

	router := httprouter.MCPRouter(cfg)
	srv := &http.Server{
		Addr:         cfg.HTTPAddr(),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: cfg.AI.RequestTimeout() + 20*time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		slog.Info("starting MCP service", "addr", cfg.HTTPAddr(), "upstream", cfg.Services.APIURL)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("mcp server failed", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down MCP service")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("mcp server forced to shutdown", "error", err)
		os.Exit(1)
	}
	slog.Info("MCP service stopped gracefully")
}
