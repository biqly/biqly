package main

import (
	"context"
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

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type config struct {
	Port          string
	RedisDSN      string
	InternalToken string
}

func loadConfig() config {
	return config{
		Port:          envOrDefault("BI_AUTH_PORT", "8889"),
		RedisDSN:      envOrDefault("BI_AUTH_REDIS_DSN", ""),
		InternalToken: envOrDefault("BI_AUTH_INTERNAL_TOKEN", ""),
	}
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

var startupTime = time.Now().UTC()

func main() {
	cfg := loadConfig()

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	r.Get("/health", healthHandler)
	r.Get("/ready", readyHandler)
	r.Get("/metrics", metricsHandler)

	r.Route("/auth", func(r chi.Router) {
		r.Post("/register", placeholderHandler("register"))
		r.Post("/login", placeholderHandler("login"))
		r.Post("/refresh", placeholderHandler("refresh"))
		r.Post("/logout", placeholderHandler("logout"))
		r.Post("/forgot-password", placeholderHandler("forgot-password"))
		r.Post("/reset-password", placeholderHandler("reset-password"))
		r.Get("/verify-email", placeholderHandler("verify-email"))
		r.Post("/resend-verification", placeholderHandler("resend-verification"))

		r.Route("/oauth", func(r chi.Router) {
			r.Get("/github", placeholderHandler("oauth-github"))
			r.Get("/github/callback", placeholderHandler("oauth-github-callback"))
			r.Get("/google", placeholderHandler("oauth-google"))
			r.Get("/google/callback", placeholderHandler("oauth-google-callback"))
		})

		r.Route("/passkey", func(r chi.Router) {
			r.Post("/register-begin", placeholderHandler("passkey-register-begin"))
			r.Post("/register-finish", placeholderHandler("passkey-register-finish"))
			r.Post("/login-begin", placeholderHandler("passkey-login-begin"))
			r.Post("/login-finish", placeholderHandler("passkey-login-finish"))
		})

		r.Route("/me", func(r chi.Router) {
			r.Get("/", placeholderHandler("me"))
			r.Put("/", placeholderHandler("me-update"))
			r.Put("/password", placeholderHandler("me-password"))
			r.Get("/passkeys", placeholderHandler("me-passkeys"))
			r.Get("/sessions", placeholderHandler("me-sessions"))
		})

		r.Route("/admin", func(r chi.Router) {
			r.Use(internalTokenMiddleware(cfg.InternalToken))
			r.Get("/users", placeholderHandler("admin-users"))
			r.Get("/roles", placeholderHandler("admin-roles"))
			r.Get("/permissions", placeholderHandler("admin-permissions"))
			r.Get("/audit-log", placeholderHandler("admin-audit"))
			r.Get("/datasource-access", placeholderHandler("admin-datasource-access"))
		})

		r.Route("/workspaces", func(r chi.Router) {
			r.Get("/", placeholderHandler("workspaces-list"))
			r.Post("/", placeholderHandler("workspaces-create"))
			r.Route("/{id}", func(r chi.Router) {
				r.Get("/", placeholderHandler("workspace-get"))
				r.Get("/members", placeholderHandler("workspace-members"))
				r.Get("/datasources", placeholderHandler("workspace-datasources"))
			})
		})

		r.Get("/shares", placeholderHandler("shares-list"))
		r.Post("/shares", placeholderHandler("shares-create"))

		r.Get("/me/datasources", placeholderHandler("me-datasources"))
	})

	if os.Getenv("BI_AUTH_SUBCOMMAND") == "migrate" {
		slog.Info("migrate subcommand not yet implemented")
		os.Exit(0)
	}

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%s", cfg.Port),
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		slog.Info("auth service starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("listen error", "err", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()

	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "err", err)
	}
}

func healthHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func readyHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"status":    "ok",
		"uptime":    time.Since(startupTime).Round(time.Second).String(),
		"service":   "auth",
	})
}

func metricsHandler(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	fmt.Fprintf(w, "# HELP auth_up Service readiness\n# TYPE auth_up gauge\nauth_up 1\n")
}

func placeholderHandler(name string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotImplemented)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "not_implemented",
			"message": fmt.Sprintf("%s endpoint is not yet implemented", name),
		})
	}
}

func internalTokenMiddleware(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if token == "" {
				next.ServeHTTP(w, r)
				return
			}
			auth := r.Header.Get("Authorization")
			if strings.TrimPrefix(auth, "Bearer ") != token &&
				r.Header.Get("X-Internal-Token") != token {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]string{"error": "unauthorized"})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
