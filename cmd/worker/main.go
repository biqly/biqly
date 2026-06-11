// Package main is the background worker process for the BI query engine.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/http/handlers"
	"github.com/biqly/biqly/internal/platform/observability"
	"github.com/biqly/biqly/internal/queue"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}
	observability.SetupLogging(cfg.Logging.Level, cfg.Logging.Format)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	shutdownTracing, tracErr := observability.SetupTracing(context.Background(), "worker")
	if tracErr != nil {
		slog.Warn("tracing setup failed, continuing without traces", "error", tracErr)
	}
	defer func() { _ = shutdownTracing(context.Background()) }()

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
	aiHandler := handlers.NewAIHandler(deps.AIDeps())
	jobSvc := handlers.NewAIJobService(deps.MetaRepo, pub, aiHandler)

	slog.Info("worker started", "nats_url", cfg.NATS.URL, "group", cfg.NATS.ConsumerGroup)

	metricsSrv := startMetricsServer(cfg.HTTPAddr())

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
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := metricsSrv.Shutdown(shutdownCtx); err != nil {
		slog.Warn("worker metrics server shutdown error", "error", err)
	}
	slog.Info("worker shutting down")
}

func startMetricsServer(addr string) *http.Server {
	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	mux.HandleFunc("/readyz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	srv := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}
	go func() {
		slog.Info("worker metrics server started", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("worker metrics server stopped", "error", err)
			os.Exit(1)
		}
	}()
	return srv
}
