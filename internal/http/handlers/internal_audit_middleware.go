package handlers

import (
	"net/http"
	"strings"
	"time"

	"github.com/biqly/biqly/internal/audit"
	"github.com/biqly/biqly/internal/http/response"
	"github.com/biqly/biqly/internal/platform/observability"
	"github.com/biqly/biqly/pkg/common/requestid"
)

// InternalAuditMiddleware writes one audit event for each /internal/* request.
func InternalAuditMiddleware(logger *audit.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rec := response.NewStatusRecorder(w)
			next.ServeHTTP(rec, r)
			if logger == nil {
				return
			}
			traceID, spanID := observability.SpanIDs(r.Context())
			logger.Log(r.Context(), audit.Event{
				UserID:    internalCallerFromRequest(r),
				EventType: audit.EventInternalRequest,
				Details: map[string]any{
					"source":     "service",
					"caller":     internalCallerFromRequest(r),
					"method":     r.Method,
					"path":       r.URL.Path,
					"status":     rec.Status(),
					"request_id": requestid.FromContext(r.Context()),
					"trace_id":   traceID,
					"span_id":    spanID,
				},
				Timestamp: time.Now().UTC(),
			})
		})
	}
}

func internalCallerFromRequest(r *http.Request) string {
	if caller := strings.TrimSpace(r.Header.Get("X-Internal-Caller")); caller != "" {
		return caller
	}
	return "unknown"
}
