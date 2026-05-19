package http

import (
	"net/http"

	"github.com/biqly/biqly/pkg/common/tracecontext"
)

func traceContextPropagationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := tracecontext.WithTraceparent(r.Context(), r.Header.Get("traceparent"))
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
