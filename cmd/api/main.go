// Package main is the main API server for the BI query engine.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	_ "net/http/pprof" // #nosec G108 — local interface only, used for diagnostics
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/config"
	httprouter "github.com/biqly/biqly/internal/http"
	"github.com/biqly/biqly/internal/http/handlers"
	"github.com/biqly/biqly/internal/platform/observability"
	"github.com/biqly/biqly/internal/queue"
	"github.com/biqly/biqly/internal/security"
)

func main() {
	// Start pprof server on localhost:6060 for local profiling/debugging
	go func() {
		slog.Info("starting pprof server", "addr", "localhost:6060")
		pprofSrv := &http.Server{
			Addr:         "localhost:6060",
			Handler:      nil, // uses http.DefaultServeMux
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 10 * time.Second,
			IdleTimeout:  30 * time.Second,
		}
		if err := pprofSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("pprof server failed", "error", err)
		}
	}()

	// Load config
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}
	observability.SetupLogging(cfg.Logging.Level, cfg.Logging.Format)

	ctx := context.Background()

	shutdownTracing, err := observability.SetupTracing(ctx, "api")
	if err != nil {
		slog.Warn("tracing setup failed, continuing without traces", "error", err)
	}
	defer func() {
		if shutdownErr := shutdownTracing(context.Background()); shutdownErr != nil {
			slog.Warn("trace provider shutdown error", "error", shutdownErr)
		}
	}()

	// Wire dependencies
	deps, err := app.NewDependencies(ctx, cfg)
	if err != nil {
		slog.Error("failed to initialize dependencies", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := deps.Close(); err != nil {
			slog.Error("failed to close dependencies", "error", err)
		}
	}()

	slog.Info("dependencies initialized",
		"metadata_db", security.RedactDSN(cfg.Metadata.DSN),
	)

	authClient := httprouter.NewAuthClient(deps)
	deps.WireAIUserResolver(authClient)
	deps.WireDriftNotifier(authClient)

	deps.DriftScheduler.Start(ctx)

	if deps.Jobs.Enabled {
		pub, qerr := app.NewAIJobQueue(cfg)
		if qerr != nil {
			slog.Error("failed to create ai job queue", "error", qerr)
			os.Exit(1)
		}
		deps.AIJobQueue = pub
		aiHandler := handlers.NewAIHandler(deps.AIDeps())
		aiHandler.SetAuthClient(authClient)
		jobSvc := handlers.NewAIJobService(deps.MetaRepo, pub, aiHandler)
		deps.AIJobService = jobSvc
		deps.AIJobsHTTP = handlers.NewAIJobsHandler(jobSvc)
		if _, ok := pub.(queue.AIJobConsumer); ok {
			consumerCtx := context.Background()
			go func() {
				if err := jobSvc.StartConsumer(consumerCtx, cfg.NATS.ConsumerGroup); err != nil {
					slog.Error("ai job consumer stopped", "error", err)
				}
			}()
			slog.Info("ai job consumer started", "nats_url", cfg.NATS.URL)
		}
	}

	// Setup router
	router := httprouter.Router(deps)

	// Create server
	srv := &http.Server{
		Addr:         cfg.HTTPAddr(),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: cfg.AI.RequestTimeout(),
		IdleTimeout:  120 * time.Second,
	}

	// Start server
	go func() {
		slog.Info("starting HTTP server", "addr", cfg.HTTPAddr())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for interrupt
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped gracefully")
}
