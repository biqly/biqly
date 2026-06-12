package http

import (
	"net/http"

	"github.com/biqly/biqly/pkg/common/requestid"
	"github.com/go-chi/chi/v5/middleware"
)

// RequestIDPropagation copies chi's request ID into the shared requestid
// context key used by handlers and log helpers.
func RequestIDPropagation(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := middleware.GetReqID(r.Context())
		ctx := requestid.WithRequestID(r.Context(), id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
