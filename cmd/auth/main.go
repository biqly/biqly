package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/bytedance/sonic"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	biqauth "github.com/biqly/biqly/internal/auth"
	"github.com/biqly/biqly/internal/auth/handlers"
	"github.com/biqly/biqly/internal/auth/ldap"
	"github.com/biqly/biqly/internal/auth/mfa"
	"github.com/biqly/biqly/internal/auth/rbac"
	"github.com/biqly/biqly/internal/auth/workspace"
	bihttp "github.com/biqly/biqly/internal/http"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/biqly/biqly/internal/mail"
	"github.com/biqly/biqly/internal/platform/observability"
	"github.com/biqly/biqly/internal/security"
	"github.com/biqly/biqly/pkg/common/requestid"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/prometheus/client_golang/prometheus"
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
	shutdownTracing := setupAuthObservability()
	defer func() { _ = shutdownTracing(context.Background()) }()
	runtime, err := newAuthRuntime(cfg)
	if err != nil {
		slog.Error("initialize auth runtime", "err", err)
		os.Exit(1)
	}
	defer runtime.close()
	runAuthHTTPServer(cfg, runtime)
}

func setupAuthObservability() func(context.Context) error {
	observability.SetupLogging(os.Getenv("BI_AUTH_LOG_LEVEL"), os.Getenv("BI_AUTH_LOG_FORMAT"))
	shutdownTracing, tracErr := observability.SetupTracing(context.Background(), "auth")
	if tracErr != nil {
		slog.Warn("tracing setup failed, continuing without traces", "error", tracErr)
	}
	return shutdownTracing
}

type authRuntime struct {
	db          *sql.DB
	redis       *redis.Client
	authHandler *handlers.AuthHandler
	rbacHandler *handlers.RBACHandler
	limiter     *biqauth.RateLimiter
	startedAt   time.Time
}

func (r *authRuntime) close() {
	if r.redis != nil {
		_ = r.redis.Close()
	}
	if r.db != nil {
		_ = r.db.Close()
	}
}

func newAuthRuntime(cfg *biqauth.Config) (*authRuntime, error) {
	db, err := sql.Open("pgx", cfg.DBDSN)
	if err != nil {
		return nil, err
	}
	configureDB(db)
	registerSessionGauge(db)

	redisClient, err := newRedisClient(cfg.RedisDSN)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	jwtMgr, err := biqauth.NewJWTManager(cfg.JWTPrivateKeyPath, cfg.JWTPublicKeyPath, cfg.JWTAccessTTL)
	if err != nil {
		_ = redisClient.Close()
		_ = db.Close()
		return nil, err
	}

	var tokenEnc *security.Encryption
	if cfg.EncryptionKey != "" {
		tokenEnc, err = security.NewEncryptionFromBase64(cfg.EncryptionKey)
		if err != nil {
			_ = redisClient.Close()
			_ = db.Close()
			return nil, err
		}
	}

	authHandler, rbacHandler, limiter, err := buildAuthHandlers(cfg, db, redisClient, jwtMgr, tokenEnc)
	if err != nil {
		_ = redisClient.Close()
		_ = db.Close()
		return nil, err
	}

	return &authRuntime{
		db:          db,
		redis:       redisClient,
		authHandler: authHandler,
		rbacHandler: rbacHandler,
		limiter:     limiter,
		startedAt:   time.Now().UTC(),
	}, nil
}

func buildAuthHandlers(cfg *biqauth.Config, db *sql.DB, redisClient *redis.Client, jwtMgr *biqauth.JWTManager, tokenEnc *security.Encryption) (*handlers.AuthHandler, *handlers.RBACHandler, *biqauth.RateLimiter, error) {
	userRepo := biqauth.NewUserRepository(db, tokenEnc)
	rbacRepo := rbac.NewRBACRepository(db)
	sessionMgr := biqauth.NewSessionManager(db)
	sessionMgr.SetLifecycleTTLs(cfg.SessionAbsoluteTTL, cfg.SessionIdleTTL)

	var emailSender mail.EmailSender
	if cfg.MailServiceURL != "" {
		emailSender = mail.NewAPIClient(cfg.MailServiceURL, cfg.MailInternalToken, nil)
	} else {
		emailSender = mail.NewMockEmailSender()
	}

	authSvc := biqauth.NewAuthService(userRepo, rbacRepo, sessionMgr, jwtMgr, cfg, redisClient, emailSender)
	platformSettingsRepo := biqauth.NewPlatformSettingsRepository(db)
	authSvc.SetPlatformSettingsRepository(platformSettingsRepo)
	ldapConfigRepo := biqauth.NewLDAPConfigRepository(db, tokenEnc)
	authSvc.SetLDAP(ldapConfigRepo, ldap.New())
	webAuthnSvc, err := mfa.NewWebAuthnService(cfg, userRepo)
	if err != nil {
		return nil, nil, nil, err
	}
	rbacSvc := rbac.NewRBACService(rbacRepo)
	dsAccessSvc := rbac.NewDatasourceAccessService(db, redisClient, rbacSvc)
	aiModelAccessSvc := rbac.NewAIModelAccessService(db, rbacSvc)
	workspaceSvc := workspace.NewWorkspaceService(db, dsAccessSvc)
	authSvc.SetWorkspaceService(workspaceSvc)
	sharingSvc := workspace.NewSharingService(db)
	auditSvc := biqauth.NewAuditService(db)

	mfaRepo := mfa.NewMFARepository(db, tokenEnc)
	mfaSvc := mfa.NewMFAService(mfaRepo, userRepo, cfg.JWTIssuer)
	authSvc.SetMFAService(mfaSvc)

	magicLinkRepo := biqauth.NewMagicLinkRepository(db)
	authSvc.SetMagicLinkRepository(magicLinkRepo)
	authSvc.SetAuditService(auditSvc)

	limiter := biqauth.NewRateLimiter(redisClient)
	authHandler := handlers.NewAuthHandler(authSvc, webAuthnSvc, jwtMgr, cfg, limiter)
	authHandler.SetMFA(mfaSvc)
	gdprExporter := handlers.NewGDPRExporter(db, userRepo, workspaceSvc, dsAccessSvc, sharingSvc, auditSvc, webAuthnSvc)
	authHandler.SetGDPRExporter(gdprExporter)
	authHandler.SetAuditService(auditSvc)
	rbacHandler := handlers.NewRBACHandler(rbacSvc, rbacRepo, userRepo, dsAccessSvc, aiModelAccessSvc, workspaceSvc, sharingSvc, auditSvc, jwtMgr, cfg)
	return authHandler, rbacHandler, limiter, nil
}

func runAuthHTTPServer(cfg *biqauth.Config, runtime *authRuntime) {
	state := &appState{
		db:          runtime.db,
		redis:       runtime.redis,
		startedAt:   runtime.startedAt,
		serviceName: "auth",
	}
	router := newRouter(state, runtime.authHandler, runtime.rbacHandler, runtime.limiter, cfg)
	server := &http.Server{
		Addr:         cfg.HTTPAddr(),
		Handler:      bihttp.OTELHTTPHandler("biqly-auth", router),
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

// registerSessionGauge exposes auth_active_sessions, the count of live
// (non-revoked, non-expired) sessions. It is a GaugeFunc so the value is
// computed from the source of truth on each scrape rather than tracked via
// inc/dec across every create/revoke path, which would drift.
func registerSessionGauge(db *sql.DB) {
	prometheus.MustRegister(prometheus.NewGaugeFunc(
		prometheus.GaugeOpts{
			Name: "auth_active_sessions",
			Help: "Number of active (non-revoked, non-expired) sessions",
		},
		func() float64 {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			var n float64
			err := db.QueryRowContext(ctx,
				`SELECT count(*) FROM sessions WHERE revoked_at IS NULL AND expires_at > NOW()`,
			).Scan(&n)
			if err != nil {
				slog.Warn("active sessions gauge query failed", "err", err)
				return 0
			}
			return n
		},
	))
}

func newRedisClient(dsn string) (*redis.Client, error) {
	opts, err := redis.ParseURL(dsn)
	if err != nil {
		return nil, fmt.Errorf("parse redis dsn: %w", err)
	}
	return redis.NewClient(opts), nil
}

// propagateRequestID copies chi's request ID into the shared requestid context
// key so handlers and error helpers (requestid.FromContext) attach request_id
// to every structured log line for that request.
func propagateRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id := middleware.GetReqID(r.Context()); id != "" {
			r = r.WithContext(requestid.WithRequestID(r.Context(), id))
		}
		next.ServeHTTP(w, r)
	})
}

func newRouter(state *appState, authHandler *handlers.AuthHandler, rbacHandler *handlers.RBACHandler, limiter *biqauth.RateLimiter, cfg *biqauth.Config) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(propagateRequestID)
	r.Use(bihttp.TraceContextPropagationMiddleware)
	r.Use(bimw.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	r.Use(bimw.SecurityHeaders(bimw.SecurityHeadersConfig{
		HSTSEnabled:           cfg.HSTSEnabled,
		HSTSIncludeSubdomains: true,
		HSTSPreload:           cfg.HSTSEnabled && cfg.HSTSPreload,
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
		r.Use(biqauth.CSRF(cfg.Port))
		authHandler.RegisterAuthRoutes(r)
		rbacHandler.RegisterAuthRoutes(r, authHandler.AuthMiddleware())
		authHandler.RegisterAccountAdminRoutes(r, authHandler.AuthMiddleware())
	})

	r.Route("/auth", func(r chi.Router) {
		r.Use(biqauth.CSRF(cfg.Port))
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

func (*appState) handleHealth(w http.ResponseWriter, _ *http.Request) {
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

	// Migration status: a dirty schema_migrations row means a migration failed
	// midway and the schema is in an inconsistent state — not safe to serve.
	// A missing table / no row (fresh DB or external migrator) is not treated
	// as a failure here; we only act on a definitive dirty=true reading.
	var dirty bool
	if err := s.db.QueryRowContext(ctx, `SELECT dirty FROM schema_migrations LIMIT 1`).Scan(&dirty); err == nil && dirty {
		writeJSON(w, http.StatusServiceUnavailable, map[string]string{
			"status": "unavailable",
			"error":  "migrations dirty",
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
	if err := sonic.ConfigStd.NewEncoder(w).Encode(data); err != nil {
		return
	}
}
