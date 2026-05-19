package handlers

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/biqly/biqly/pkg/internalapi"
)

// InternalTokenMiddleware rejects /internal/* requests unless the shared
// peer-service token matches. It fails closed when unset so deployments cannot
// accidentally expose the internal wire protocol.
func InternalTokenMiddleware(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token = strings.TrimSpace(token)
			if token == "" {
				writeInternalAPIErrorMsg(w, http.StatusForbidden, internalapi.CodeUnauthorized,
					"internal API token is not configured")
				return
			}

			got := internalTokenFromRequest(r)
			if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(token)) != 1 {
				writeInternalAPIErrorMsg(w, http.StatusUnauthorized, internalapi.CodeUnauthorized,
					"invalid or missing internal API token")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func internalTokenFromRequest(r *http.Request) string {
	if token := strings.TrimSpace(r.Header.Get("X-Internal-Token")); token != "" {
		return token
	}
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	if strings.HasPrefix(strings.ToLower(authHeader), "bearer ") {
		return strings.TrimSpace(authHeader[7:])
	}
	return authHeader
}
