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
		// Both keys stay valid bypass credentials: BI_API_KEY (legacy /api
		// gate) and BI_ADMIN_API_KEY (the key admin tooling and the frontend's
		// useAdminApi flows actually send).
		return bimw.JWTAuthWithAdminBypass(provider,
			[]string{deps.Config.Security.APIKey, deps.Config.Security.AdminAPIKey})
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

// buildAIAuthClient builds the auth-service client for the AI service. Unlike
// NewAuthClient it does not require BI_AUTH_ENABLED: the AI service needs the
// client for per-user features (model preferences, RBAC model access) whenever
// the auth service is reachable, even when JWT enforcement is left off and
// identity is resolved optionally. Returns nil when no auth service URL is set.
func buildAIAuthClient(deps *app.Dependencies) *bimw.AuthClient {
	if deps.Config.Auth.ServiceURL == "" {
		return nil
	}
	return bimw.NewAuthClient(deps.Config.Auth.ServiceURL, deps.Config.Auth.InternalToken)
}

// buildAIAuthMiddleware selects the AI service's request-auth middleware:
//   - BI_AUTH_ENABLED=true  -> hard JWT enforcement (every /api request needs a JWT).
//   - auth service URL set  -> optional JWT identity: the caller is resolved from a
//     valid Bearer JWT when present, but admin-key and unauthenticated routes keep
//     working. This is the default microservice posture (network-trusted edge).
//   - otherwise             -> legacy API key check.
func buildAIAuthMiddleware(deps *app.Dependencies) func(http.Handler) http.Handler {
	authCfg := deps.Config.Auth
	if authCfg.Enabled {
		if authCfg.ServiceURL == "" {
			slog.Error("BI_AUTH_ENABLED is true but BI_AUTH_SERVICE_URL is empty — JWT verification will fail")
		}
		return bimw.JWTAuthWithAdminBypass(bimw.NewPublicKeyProvider(authCfg.ServiceURL, authCfg.InternalToken),
			[]string{deps.Config.Security.APIKey, deps.Config.Security.AdminAPIKey})
	}
	if authCfg.ServiceURL != "" {
		return bimw.OptionalJWTAuth(bimw.NewPublicKeyProvider(authCfg.ServiceURL, authCfg.InternalToken))
	}
	return bimw.APIKeyAuth(deps.Config.Security.APIKey)
}
