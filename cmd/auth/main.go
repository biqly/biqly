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
	"strings"
	"syscall"
	"time"

	biqauth "github.com/biqly/biqly/internal/auth"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/biqly/biqly/internal/mail"
	"github.com/biqly/biqly/internal/security"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/prometheus/client_golang/prometheus/promhttp"
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

	var tokenEnc *security.Encryption
	if cfg.EncryptionKey != "" {
		tokenEnc, err = security.NewEncryptionFromBase64(cfg.EncryptionKey)
		if err != nil {
			slog.Error("initialize oauth token encryption", "err", err)
			os.Exit(1)
		}
	}
	userRepo := biqauth.NewUserRepository(db, tokenEnc)
	rbacRepo := biqauth.NewRBACRepository(db)
	sessionMgr := biqauth.NewSessionManager(db)
	sessionMgr.SetLifecycleTTLs(cfg.SessionAbsoluteTTL, cfg.SessionIdleTTL)

	var emailSender mail.EmailSender
	if cfg.MailServiceURL != "" {
		emailSender = mail.NewAPIClient(cfg.MailServiceURL, cfg.MailInternalToken, nil)
	} else {
		emailSender = mail.NewMockEmailSender()
	}

	authSvc := biqauth.NewAuthService(userRepo, rbacRepo, sessionMgr, jwtMgr, cfg, redisClient, emailSender)
	webAuthnSvc, err := biqauth.NewWebAuthnService(cfg, userRepo)
	if err != nil {
		slog.Error("initialize webauthn", "err", err)
		os.Exit(1)
	}
	rbacSvc := biqauth.NewRBACService(rbacRepo)
	dsAccessSvc := biqauth.NewDatasourceAccessService(db, redisClient, rbacSvc)
	workspaceSvc := biqauth.NewWorkspaceService(db, dsAccessSvc)
	authSvc.SetWorkspaceService(workspaceSvc)
	sharingSvc := biqauth.NewSharingService(db)
	auditSvc := biqauth.NewAuditService(db)

	mfaRepo := biqauth.NewMFARepository(db, tokenEnc)
	mfaSvc := biqauth.NewMFAService(mfaRepo, userRepo, cfg.JWTIssuer)
	authSvc.SetMFAService(mfaSvc)

	magicLinkRepo := biqauth.NewMagicLinkRepository(db)
	authSvc.SetMagicLinkRepository(magicLinkRepo)

	limiter := biqauth.NewRateLimiter(redisClient)
	authHandler := biqauth.NewAuthHandler(authSvc, webAuthnSvc, jwtMgr, cfg, limiter)
	authHandler.SetMFA(mfaSvc)
	gdprExporter := biqauth.NewGDPRExporter(db, userRepo, workspaceSvc, dsAccessSvc, sharingSvc, auditSvc, webAuthnSvc)
	authHandler.SetGDPRExporter(gdprExporter)
	authHandler.SetAuditService(auditSvc)
	rbacHandler := biqauth.NewRBACHandler(rbacSvc, rbacRepo, userRepo, dsAccessSvc, workspaceSvc, sharingSvc, auditSvc, jwtMgr, cfg)

	state := &appState{
		db:          db,
		redis:       redisClient,
		startedAt:   time.Now().UTC(),
		serviceName: "auth",
	}
	router := newRouter(state, authHandler, rbacHandler, limiter, cfg)
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

func newRouter(state *appState, authHandler *biqauth.AuthHandler, rbacHandler *biqauth.RBACHandler, limiter *biqauth.RateLimiter, cfg *biqauth.Config) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	httpsOnly := len(cfg.WebAuthnOrigins) > 0 && strings.HasPrefix(cfg.WebAuthnOrigins[0], "https")
	r.Use(bimw.SecurityHeaders(bimw.SecurityHeadersConfig{
		HSTSEnabled:           httpsOnly,
		HSTSIncludeSubdomains: true,
		HSTSPreload:           httpsOnly && cfg.HSTSPreload,
		HSTSMaxAgeSeconds:     cfg.HSTSMaxAgeSeconds,
		ContentSecurityPolicy: "default-src 'self'; frame-ancestors 'none'",
	}))

	if len(cfg.CORSAllowedOrigins) == 0 {
		slog.Warn("auth CORS allowed origins is empty — cross-origin requests will be blocked. Set BI_AUTH_CORS_ALLOWED_ORIGINS to allow specific frontend origins.")
	}
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   cfg.CORSAllowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Accept-Language", "Authorization", "Content-Type", "X-CSRF-Token"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	if limiter != nil {
		r.Use(limiter.Limit(cfg.RateLimitPerMin, time.Minute, "general"))
	}

	r.Get("/health", state.handleHealth)
	r.Get("/ready", state.handleReady)
	r.Handle("/metrics", promhttp.Handler())

	r.Route("/api/auth", func(r chi.Router) {
		r.Use(biqauth.CSRF(httpsOnly))
		authHandler.RegisterAuthRoutes(r)
		rbacHandler.RegisterAuthRoutes(r, authHandler.AuthMiddleware())
		authHandler.RegisterAccountAdminRoutes(r, authHandler.AuthMiddleware())
	})

	r.Route("/auth", func(r chi.Router) {
		r.Use(biqauth.CSRF(httpsOnly))
		authHandler.RegisterAuthRoutes(r)
		rbacHandler.RegisterAuthRoutes(r, authHandler.AuthMiddleware())
		authHandler.RegisterAccountAdminRoutes(r, authHandler.AuthMiddleware())
	})

	r.Route("/internal/auth", func(r chi.Router) {
		authHandler.RegisterInternalRoutes(r)
		rbacHandler.RegisterInternalRoutes(r, authHandler.InternalTokenMiddleware())
	})

	return r
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

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
