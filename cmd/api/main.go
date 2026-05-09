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
)

func main() {
	// Setup logger
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	// Load config
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()

	// Wire dependencies
	deps, err := app.NewDependencies(ctx, cfg)
	if err != nil {
		slog.Error("failed to initialize dependencies", "error", err)
		os.Exit(1)
	}
	defer deps.Close()

	slog.Info("dependencies initialized",
		"metadata_db", cfg.Metadata.DSN,
	)

	// Setup router
	router := httprouter.Router(deps)

	// Create server
	srv := &http.Server{
		Addr:         cfg.HTTPAddr(),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 60 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	// Start server
	go func() {
		slog.Info("starting HTTP server", "addr", cfg.HTTPAddr())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped gracefully")
}
