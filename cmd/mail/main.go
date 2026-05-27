// Package main runs the standalone transactional mail worker. It owns the
// mail database (block list), exposes an internal HTTP API for the auth
// service to enqueue messages, and dispatches them via SMTP with retry.
package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"

	"github.com/biqly/biqly/internal/mail"
)

func main() {
	cfg := mail.NewConfigFromEnv()

	if cfg.DBDSN == "" {
		slog.Error("BI_MAIL_DB_DSN is required")
		os.Exit(1)
	}
	if cfg.InternalToken == "" {
		slog.Error("BI_MAIL_INTERNAL_TOKEN is required")
		os.Exit(1)
	}

	db, err := sql.Open("pgx", cfg.DBDSN)
	if err != nil {
		slog.Error("open database", "err", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(30 * time.Minute)

	pingCtx, cancelPing := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelPing()
	if err := db.PingContext(pingCtx); err != nil {
		slog.Error("ping database", "err", err)
		os.Exit(1)
	}

	var redisClient *redis.Client
	if cfg.RedisDSN != "" {
		opts, parseErr := redis.ParseURL(cfg.RedisDSN)
		if parseErr != nil {
			slog.Error("parse redis dsn", "err", parseErr)
			os.Exit(1)
		}
		redisClient = redis.NewClient(opts)
		defer func() { _ = redisClient.Close() }()
	}

	blockList := mail.NewEmailBlockListRepo(db)
	sender, err := mail.NewSMTPEmailSender(cfg, blockList, redisClient)
	if err != nil {
		slog.Error("initialize smtp sender", "err", err)
		os.Exit(1)
	}
	defer sender.Close()

	mailServer := mail.NewServer(sender, cfg.InternalToken)

	router := chi.NewRouter()
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)
	router.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	router.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := db.PingContext(ctx); err != nil {
			http.Error(w, "not ready", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ready"))
	})
	router.Mount("/", mailServer.Routes())

	server := &http.Server{
		Addr:              ":" + strconv.Itoa(cfg.Port),
		Handler:           router,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("mail worker starting", "addr", server.Addr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("listen error", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "err", err)
	}
	// Drain the in-flight email queue before exit.
	sender.Close()
}
