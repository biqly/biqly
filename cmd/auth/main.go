package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	biqauth "github.com/biqly/biqly/internal/auth"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/redis/go-redis/v9"
)

type appState struct {
	db          *sql.DB
	redis       *redis.Client
	startedAt   time.Time
	serviceName string
}

func main() {
	cfg, err := biqauth.LoadConfig()
	if err != nil {
		slog.Error("load config", "err", err)
		os.Exit(1)
	}

	db, err := sql.Open("pgx", cfg.DBDSN)
	if err != nil {
		slog.Error("open database", "err", err)
		os.Exit(1)
	}
	defer func() { _ = db.Close() }()
	configureDB(db)

	redisClient, err := newRedisClient(cfg.RedisDSN)
	if err != nil {
		slog.Error("configure redis", "err", err)
		os.Exit(1)
	}
	defer func() { _ = redisClient.Close() }()

	jwtMgr, err := biqauth.NewJWTManager(cfg.JWTPrivateKeyPath, cfg.JWTPublicKeyPath, cfg.JWTAccessTTL)
	if err != nil {
		slog.Error("initialize jwt manager", "err", err)
		os.Exit(1)
	}

	userRepo := biqauth.NewUserRepository(db)
	rbacRepo := biqauth.NewRBACRepository(db)
	sessionMgr := biqauth.NewSessionManager(db)
	authSvc := biqauth.NewAuthService(userRepo, rbacRepo, sessionMgr, jwtMgr, cfg)
	webAuthnSvc, err := biqauth.NewWebAuthnService(cfg, userRepo)
	if err != nil {
		slog.Error("initialize webauthn", "err", err)
		os.Exit(1)
	}
	rbacSvc := biqauth.NewRBACService(rbacRepo)
	dsAccessSvc := biqauth.NewDatasourceAccessService(db, redisClient, rbacSvc)
	workspaceSvc := biqauth.NewWorkspaceService(db, dsAccessSvc)
	sharingSvc := biqauth.NewSharingService(db)
	auditSvc := biqauth.NewAuditService(db)

	authHandler := biqauth.NewAuthHandler(authSvc, webAuthnSvc, jwtMgr, cfg)
	rbacHandler := biqauth.NewRBACHandler(rbacSvc, rbacRepo, dsAccessSvc, workspaceSvc, sharingSvc, auditSvc, jwtMgr, cfg)

	state := &appState{
		db:          db,
		redis:       redisClient,
		startedAt:   time.Now().UTC(),
		serviceName: "auth",
	}
	router := newRouter(state, authHandler, rbacHandler)
	server := &http.Server{
		Addr:         cfg.HTTPAddr(),
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("auth service starting", "addr", cfg.HTTPAddr())
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
}

func configureDB(db *sql.DB) {
	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)
}

func newRedisClient(dsn string) (*redis.Client, error) {
	opts, err := redis.ParseURL(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse redis dsn: %w", err)
	}
	return redis.NewClient(opts), nil
}

func newRouter(state *appState, authHandler *biqauth.AuthHandler, rbacHandler *biqauth.RBACHandler) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	r.Get("/health", state.handleHealth)
	r.Get("/ready", state.handleReady)
	r.Get("/metrics", state.handleMetrics)

	mountAuthRoutes(r, "/api/auth", authHandler, rbacHandler)
	mountAuthRoutes(r, "/auth", authHandler, rbacHandler)
	r.Route("/internal/auth", func(r chi.Router) {
		authHandler.RegisterInternalRoutes(r)
		rbacHandler.RegisterInternalRoutes(r, authHandler.InternalTokenMiddleware())
	})

	return r
}

func mountAuthRoutes(r chi.Router, pattern string, authHandler *biqauth.AuthHandler, rbacHandler *biqauth.RBACHandler) {
	r.Route(pattern, func(r chi.Router) {
		authHandler.RegisterAuthRoutes(r)
		rbacHandler.RegisterAuthRoutes(r, authHandler.AuthMiddleware())
	})
}

func (s *appState) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *appState) handleReady(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := s.db.PingContext(ctx); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "unavailable",
			"error":  "database unavailable",
		})
		return
	}
	if err := s.redis.Ping(ctx).Err(); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "unavailable",
			"error":  "redis unavailable",
		})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":  "ok",
		"uptime":  time.Since(s.startedAt).Round(time.Second).String(),
		"service": s.serviceName,
	})
}

func (s *appState) handleMetrics(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprint(w, "# HELP auth_up Service readiness\n# TYPE auth_up gauge\nauth_up 1\n")
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
