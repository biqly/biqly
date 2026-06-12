package http

import (
	"net/http"
	"time"

	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// BaseMiddlewareConfig controls the shared chi middleware stack used by the
// service routers.
type BaseMiddlewareConfig struct {
	Metrics                *Metrics
	Timeout                time.Duration
	ChiLogger              bool
	Locale                 bool
	CORS                   func(http.Handler) http.Handler
	SecurityHeaders        bimw.SecurityHeadersConfig
	DisableSecurityHeaders bool
}

// ApplyBaseMiddleware installs the shared request, logging, recovery, metrics,
// timeout, locale, CORS, and security middleware stack on r.
func ApplyBaseMiddleware(r chi.Router, cfg BaseMiddlewareConfig) {
	r.Use(middleware.RequestID)
	r.Use(RequestIDPropagation)
	r.Use(requestLoggerMiddleware)
	r.Use(bimw.RealIP)
	if cfg.ChiLogger {
		r.Use(middleware.Logger)
	}
	r.Use(middleware.Recoverer)
	if cfg.Metrics != nil {
		r.Use(HTTPMetricsMiddleware(cfg.Metrics))
	}
	if cfg.Timeout > 0 {
		r.Use(middleware.Timeout(cfg.Timeout))
	}
	if cfg.Locale {
		r.Use(bimw.Locale)
	}
	if cfg.CORS != nil {
		r.Use(cfg.CORS)
	}
	if !cfg.DisableSecurityHeaders {
		r.Use(bimw.SecurityHeaders(cfg.SecurityHeaders))
	}
}
