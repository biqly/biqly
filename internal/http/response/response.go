// Package response provides the canonical JSON response helpers shared by every
// HTTP handler package. Centralizing them keeps wire format, nil-slice
// normalization, and internal-error sanitization consistent so the various
// handler packages cannot silently drift apart.
package response

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"reflect"

	"github.com/biqly/biqly/pkg/common/requestid"
)

// WriteJSON writes data as a JSON response with the given status code. A nil
// slice is normalized to an empty array so clients always receive `[]` rather
// than `null`.
func WriteJSON(w http.ResponseWriter, status int, data any) {
	if data != nil {
		v := reflect.ValueOf(data)
		if v.Kind() == reflect.Slice && v.IsNil() {
			data = reflect.MakeSlice(v.Type(), 0, 0).Interface()
		}
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

// WriteError writes a JSON error response of the form {"error": message}.
func WriteError(w http.ResponseWriter, status int, message string) {
	WriteJSON(w, status, map[string]string{"error": message})
}

// WriteInternalError logs err (with request_id and any caller-supplied args)
// and writes publicMsg to the client. The caller is responsible for passing a
// public-safe message; the detailed err is sent only to logs/telemetry.
func WriteInternalError(ctx context.Context, w http.ResponseWriter, status int, publicMsg string, err error, args ...any) {
	if err != nil {
		allArgs := append([]any{"error", err}, args...)
		if reqID := requestid.FromContext(ctx); reqID != "" {
			allArgs = append(allArgs, "request_id", reqID)
		}
		slog.ErrorContext(ctx, publicMsg, allArgs...)
	}
	WriteError(w, status, publicMsg)
}
