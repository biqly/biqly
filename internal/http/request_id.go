package http

import (
	"net/http"

	"github.com/biqly/biqly/pkg/common/requestid"
	"github.com/go-chi/chi/v5/middleware"
)

func requestIDPropagationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := middleware.GetReqID(r.Context())
		ctx := requestid.WithRequestID(r.Context(), id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
