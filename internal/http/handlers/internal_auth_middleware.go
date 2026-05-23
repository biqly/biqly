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
	// Authorization MUST use the Bearer scheme. Earlier versions of this
	// middleware accepted the raw header as a fallback, which let a client
	// send the token unprefixed and bypass the scheme check.
	authHeader := strings.TrimSpace(r.Header.Get("Authorization"))
	const prefix = "bearer "
	if len(authHeader) > len(prefix) && strings.EqualFold(authHeader[:len(prefix)], prefix) {
		return strings.TrimSpace(authHeader[len(prefix):])
	}
	return ""
}
