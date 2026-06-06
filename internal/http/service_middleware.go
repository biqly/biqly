package http

import (
	"log/slog"
	"net/http"

	"github.com/biqly/biqly/internal/app"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/go-chi/cors"
)

func serviceCORS(deps *app.Dependencies) func(http.Handler) http.Handler {
	origins := deps.Config.HTTP.CORSAllowedOrigins
	if len(origins) == 0 {
		slog.Warn("CORS allowed origins is empty — cross-origin requests will be blocked. Set BI_CORS_ALLOWED_ORIGINS (comma-separated) to allow specific frontend domains.")
	}
	return cors.Handler(cors.Options{
		AllowedOrigins:   origins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Accept-Language", "Authorization", "Content-Type", "X-API-Key", "X-CSRF-Token", "X-Locale"},
		ExposedHeaders:   []string{"Link", "X-CSRF-Token"},
		AllowCredentials: true,
		MaxAge:           300,
	})
}

func serviceSecurityHeaders(deps *app.Dependencies) func(http.Handler) http.Handler {
	return bimw.SecurityHeaders(bimw.SecurityHeadersConfig{
		HSTSEnabled:           deps.Config.HTTP.HSTSEnabled,
		HSTSIncludeSubdomains: true,
		ContentSecurityPolicy: "default-src 'self'; frame-ancestors 'none'",
	})
}
