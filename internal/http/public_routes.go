package http

import (
	"github.com/go-chi/chi/v5"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/http/handlers"
	bimw "github.com/biqly/biqly/internal/http/middleware"
)

// registerPublicDashboardRoutes mounts the anonymous dashboard-metadata
// endpoint. Caller mounts it under /api/public with PublicEmbedHeaders.
func registerPublicDashboardRoutes(r chi.Router, deps *app.CatalogDeps, authClient *bimw.AuthClient) {
	limiter := bimw.NewPublicRateLimiter(deps.PublicShareRedis, deps.Config.PublicShare.RateLimitPerMinute)
	h := handlers.NewPublicDashboardHandler(deps.PublicResolver, authClient)
	r.With(limiter.Middleware()).Get("/dashboards/{token}", h.Get)
}

// registerPublicWidgetQueryRoutes mounts the anonymous widget execution
// endpoint. Caller mounts it under /api/public with PublicEmbedHeaders.
func registerPublicWidgetQueryRoutes(r chi.Router, deps *app.QueryDeps, authClient *bimw.AuthClient) {
	limiter := bimw.NewPublicRateLimiter(deps.PublicShareRedis, deps.Config.PublicShare.RateLimitPerMinute)
	h := handlers.NewPublicWidgetQueryHandler(deps.PublicResolver, deps.QueryService, authClient, deps.PublicShareRedis, deps.Config.PublicShare.CacheTTL)
	r.With(limiter.Middleware()).Post("/widget-query/{token}/{widgetID}", h.Run)
}
