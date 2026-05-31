package http

import (
	"log/slog"
	"net/http"

	"github.com/biqly/biqly/internal/platform/observability"
	"github.com/biqly/biqly/pkg/common/tracecontext"
	"github.com/go-chi/chi/v5/middleware"
)

// requestLoggerMiddleware attaches a request-scoped slog.Logger to the context,
// pre-populated with correlation fields (request_id and the W3C traceparent)
// so handlers logging via observability.LoggerFrom(ctx) emit consistent,
// traceable structured lines without re-deriving those fields per call site.
//
// It must run after chi's RequestID and the traceparent propagation middleware
// so both values are already on the context.
func requestLoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l := slog.Default()
		if id := middleware.GetReqID(r.Context()); id != "" {
			l = l.With("request_id", id)
		}
		if tp := tracecontext.TraceparentFromContext(r.Context()); tp != "" {
			l = l.With("traceparent", tp)
		}
		ctx := observability.ContextWithLogger(r.Context(), l)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
