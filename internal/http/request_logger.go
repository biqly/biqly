package http

import (
	"log/slog"
	"net/http"

	"github.com/biqly/biqly/internal/platform/observability"
	"github.com/go-chi/chi/v5/middleware"
)

// requestLoggerMiddleware attaches a request-scoped slog.Logger to the context,
// pre-populated with correlation fields (request_id plus the active trace_id and
// span_id) so handlers logging via observability.LoggerFrom(ctx) emit
// consistent, traceable structured lines without re-deriving those fields per
// call site.
//
// It must run after chi's RequestID and inside the otelhttp handler so the
// request id and active span context are already on the context.
func requestLoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		l := slog.Default()
		if id := middleware.GetReqID(r.Context()); id != "" {
			l = l.With("request_id", id)
		}
		if traceID, spanID := observability.SpanIDs(r.Context()); traceID != "" {
			l = l.With("trace_id", traceID, "span_id", spanID)
		}
		ctx := observability.ContextWithLogger(r.Context(), l)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
