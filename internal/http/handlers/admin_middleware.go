package handlers

import (
	"crypto/subtle"
	"net/http"
	"strings"

	bimw "github.com/biqly/biqly/internal/http/middleware"
)

// AdminKeyMiddleware guards operational AI admin endpoints. A caller passes
// when it already carries a verified super_admin identity (JWT roles populated
// by the outer auth middleware, or the admin-key bypass) or presents the
// configured BI_ADMIN_API_KEY. Comparison is constant-time. The Authorization
// header MUST use the Bearer scheme — an earlier version accepted raw tokens.
func AdminKeyMiddleware(adminKey string) func(http.Handler) http.Handler {
	return AdminAccessMiddleware(adminKey, nil, "")
}

// AdminAccessMiddleware guards operational AI admin endpoints. Pass order:
//  1. verified super_admin identity (JWT roles from the outer auth middleware)
//  2. the configured BI_ADMIN_API_KEY (constant-time compare; Bearer or
//     X-Admin-Key header) — kept for machine-to-machine/operational callers
//  3. an RBAC permission check against the auth service when authClient and
//     permission are wired (e.g. "ai:settings"), so admins granted the
//     permission via role management reach these endpoints with their JWT.
func AdminAccessMiddleware(adminKey string, authClient *bimw.AuthClient, permission string) func(http.Handler) http.Handler {
	expected := []byte(strings.TrimSpace(adminKey))
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Super admins authenticated by the outer JWT middleware don't
			// need the shared key — their session token is the credential.
			if bimw.HasRole(r.Context(), bimw.RoleSuperAdmin) {
				next.ServeHTTP(w, r)
				return
			}
			if token := adminKeyFromRequest(r); len(expected) > 0 && len(token) > 0 &&
				subtle.ConstantTimeCompare(token, expected) == 1 {
				r.Header.Del("X-Admin-Key")
				next.ServeHTTP(w, r)
				return
			}
			if userID := bimw.UserID(r.Context()); authClient != nil && permission != "" && userID != "" {
				allowed, err := authClient.CheckPermission(r.Context(), userID, permission, "workspace", bimw.WorkspaceID(r.Context()))
				switch {
				case err != nil:
					writeError(w, http.StatusServiceUnavailable, "permission check failed")
				case allowed:
					next.ServeHTTP(w, r)
				default:
					writeError(w, http.StatusForbidden, "permission denied: "+permission)
				}
				return
			}
			if len(expected) == 0 {
				writeError(w, http.StatusForbidden, "admin endpoints require BI_ADMIN_API_KEY to be configured")
				return
			}
			writeError(w, http.StatusUnauthorized, "invalid or missing admin API key")
		})
	}
}

func adminKeyFromRequest(r *http.Request) []byte {
	if k := strings.TrimSpace(r.Header.Get("X-Admin-Key")); k != "" {
		return []byte(k)
	}
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	const prefix = "bearer "
	if len(authHeader) > len(prefix) && strings.EqualFold(authHeader[:len(prefix)], prefix) {
		return []byte(strings.TrimSpace(authHeader[len(prefix):]))
	}
	return nil
}
