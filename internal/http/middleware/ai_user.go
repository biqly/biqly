package middleware

import (
	"net/http"

	"github.com/biqly/biqly/internal/ai"
)

// InjectAIUserContext copies the authenticated user id into the AI context
// namespace so PurposeProvider resolvers can pick per-user model preferences.
func InjectAIUserContext(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if uid := UserID(r.Context()); uid != "" {
			r = r.WithContext(ai.WithUserID(r.Context(), uid))
		}
		next.ServeHTTP(w, r)
	})
}
