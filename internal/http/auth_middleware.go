package http

import (
	"log/slog"
	"net/http"

	"github.com/biqly/biqly/internal/app"
	bimw "github.com/biqly/biqly/internal/http/middleware"
)

// buildAPIAuthMiddleware returns the auth middleware for /api/* routes.
//
// When BI_AUTH_ENABLED is true, requests must carry a JWT issued by the auth
// service (verified via its public key). Otherwise the legacy APIKey check is
// applied as before, preserving backward compatibility during migration.
func buildAPIAuthMiddleware(deps *app.Dependencies) func(http.Handler) http.Handler {
	authCfg := deps.Config.Auth
	if authCfg.Enabled {
		if authCfg.ServiceURL == "" {
			slog.Error("BI_AUTH_ENABLED is true but BI_AUTH_SERVICE_URL is empty — JWT verification will fail")
		}
		provider := bimw.NewPublicKeyProvider(authCfg.ServiceURL, authCfg.InternalToken)
		return bimw.JWTAuth(provider)
	}

	if deps.Config.Security.APIKey == "" {
		slog.Warn("BI_API_KEY is empty — /api/* routes are unauthenticated. Set BI_API_KEY in production.")
	}
	return bimw.APIKeyAuth(deps.Config.Security.APIKey)
}

// NewAuthClient builds an auth-service client used by RequirePermission and
// RequireDatasourceAccess. Returns nil when auth is disabled — middlewares
// gated by these checks should handle nil clients gracefully (current
// helpers in this package check Auth.Enabled before applying them).
func NewAuthClient(deps *app.Dependencies) *bimw.AuthClient {
	if !deps.Config.Auth.Enabled || deps.Config.Auth.ServiceURL == "" {
		return nil
	}
	return bimw.NewAuthClient(deps.Config.Auth.ServiceURL, deps.Config.Auth.InternalToken)
}
