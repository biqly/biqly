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

	"github.com/biqly/biqly/internal/auth"
	"github.com/biqly/biqly/internal/platform/db"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func main() {
	slog.Info("starting auth service...")

	cfg, err := auth.LoadConfig()
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Initialize Database Pool using existing platform package
	dbPool, err := db.NewPool(ctx, db.DefaultConfig(cfg.DBDSN))
	if err != nil {
		slog.Error("initialize database connection pool", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := db.Close(dbPool); err != nil {
			slog.Warn("close database connection pool", "error", err)
		}
	}()
	slog.Info("connected to metadata DB", "dsn", cfg.DBDSN)

	// Initialize JWT Manager
	jwtMgr, err := auth.NewJWTManager(cfg.JWTPrivateKeyPath, cfg.JWTPublicKeyPath, cfg.JWTAccessTTL)
	if err != nil {
		slog.Error("initialize JWT manager", "error", err)
		os.Exit(1)
	}

	// Initialize Repositories and Services
	userRepo := auth.NewUserRepository(dbPool)
	rbacRepo := auth.NewRBACRepository(dbPool)
	sessionMgr := auth.NewSessionManager(dbPool)
	authService := auth.NewAuthService(userRepo, rbacRepo, sessionMgr, jwtMgr, cfg)

	webAuthnService, err := auth.NewWebAuthnService(cfg, userRepo)
	if err != nil {
		slog.Error("initialize WebAuthn service", "error", err)
		os.Exit(1)
	}

	// Initialize Chi Router
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// Register Routes
	handler := auth.NewAuthHandler(authService, webAuthnService, jwtMgr, cfg)
	handler.RegisterRoutes(r)

	// HTTP Server Config
	srv := &http.Server{
		Addr:         cfg.HTTPAddr(),
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in background
	go func() {
		slog.Info("HTTP server listening", "addr", cfg.HTTPAddr())
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("HTTP server failure", "error", err)
			os.Exit(1)
		}
	}()

	// Wait for shutdown signal
	<-ctx.Done()
	slog.Info("shutting down HTTP server...")

	// Graceful shutdown with timeout
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("HTTP server graceful shutdown failed", "error", err)
	}
	slog.Info("auth service stopped.")
}
