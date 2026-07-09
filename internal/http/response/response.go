// Package response provides the canonical JSON response helpers shared by every
// HTTP handler package. Centralizing them keeps wire format, nil-slice
// normalization, and internal-error sanitization consistent so the various
// handler packages cannot silently drift apart.
package response

import (
	"context"
	"errors"
	"github.com/bytedance/sonic"
	"io"
	"log/slog"
	"net/http"
	"reflect"

	"github.com/biqly/biqly/internal/http/middleware"
	"github.com/biqly/biqly/pkg/common/requestid"
)

// MaxJSONRequestBytes caps incoming JSON request bodies.
const MaxJSONRequestBytes = 1 << 20 // 1 MiB

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
	if err := sonic.ConfigStd.NewEncoder(w).Encode(data); err != nil {
		slog.Error("failed to encode response", "error", err)
	}
}

// WriteError writes a JSON error response of the form {"error": message}.
// For status >= 500, the message is logged and sanitized to "internal server error"
// to prevent internal details from leaking.
func WriteError(w http.ResponseWriter, status int, message string) {
	if status >= http.StatusInternalServerError {
		slog.Error("server error", "detail", message, "status", status)
		message = "internal server error"
	}
	WriteJSON(w, status, map[string]string{"error": message})
}

// WriteInternalError logs err (with request_id and any caller-supplied args)
// and writes a sanitized public error response. The caller is responsible for
// passing a public-safe message; for status >= 500, the client response is
// sanitized to "internal server error".
func WriteInternalError(ctx context.Context, w http.ResponseWriter, status int, publicMsg string, err error, args ...any) {
	if err != nil {
		allArgs := append([]any{"error", err}, args...)
		if reqID := requestid.FromContext(ctx); reqID != "" {
			allArgs = append(allArgs, "request_id", reqID)
		}
		slog.ErrorContext(ctx, publicMsg, allArgs...)
	}
	msg := publicMsg
	if status >= http.StatusInternalServerError {
		msg = "internal server error"
	}
	WriteJSON(w, status, map[string]string{"error": msg})
}

// DecodeJSON decodes a required JSON request body into T and writes a standard
// 400/413 JSON error response on failure.
func DecodeJSON[T any](w http.ResponseWriter, r *http.Request) (*T, bool) {
	return decodeJSON[T](w, r, false)
}

// DecodeJSONAllowEmpty decodes an optional JSON request body into T. Empty
// bodies produce a zero-value T and no response.
func DecodeJSONAllowEmpty[T any](w http.ResponseWriter, r *http.Request) (*T, bool) {
	return decodeJSON[T](w, r, true)
}

func decodeJSON[T any](w http.ResponseWriter, r *http.Request, allowEmpty bool) (*T, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, MaxJSONRequestBytes)
	var v T
	if err := sonic.ConfigStd.NewDecoder(r.Body).Decode(&v); err != nil {
		if allowEmpty && errors.Is(err, io.EOF) {
			return &v, true
		}
		if isMaxBytesError(err) {
			WriteError(w, http.StatusRequestEntityTooLarge, "request body too large")
		} else {
			WriteError(w, http.StatusBadRequest, "invalid request body")
		}
		return nil, false
	}
	return &v, true
}

func isMaxBytesError(err error) bool {
	_, ok := errors.AsType[*http.MaxBytesError](err)
	return ok
}

// StatusRecorder wraps a http.ResponseWriter and captures the response status code.
type StatusRecorder struct {
	http.ResponseWriter
	status int
}

// NewStatusRecorder returns a new StatusRecorder wrapping w, initialized to http.StatusOK.
func NewStatusRecorder(w http.ResponseWriter) *StatusRecorder {
	return &StatusRecorder{ResponseWriter: w, status: http.StatusOK}
}

// WriteHeader captures the status code and delegates to the wrapped ResponseWriter.
func (r *StatusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

// Status returns the captured status code.
func (r *StatusRecorder) Status() int {
	return r.status
}

// Flush delegates to the wrapped ResponseWriter's http.Flusher, if it has one.
// Embedding http.ResponseWriter as an interface field only promotes that
// interface's own methods (Header/Write/WriteHeader) — it does NOT promote
// Flush from the underlying concrete writer, so without this method a
// *StatusRecorder silently fails the w.(http.Flusher) check every SSE/
// streaming handler relies on, even though the real ResponseWriter beneath it
// supports flushing. Any middleware in this codebase that wraps a
// ResponseWriter in a StatusRecorder — e.g. HTTPMetricsMiddleware, which is
// applied to every request — must not break streaming responses.
func (r *StatusRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// WriteOK writes a JSON success response of the form {"status": "ok"} with status 200 OK.
func WriteOK(w http.ResponseWriter) {
	WriteJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

// RequireUserIDFromContext retrieves the user ID from the context using the middleware helper.
// If the user ID is empty, it writes a 401 Unauthorized JSON response and returns ("", false).
func RequireUserIDFromContext(ctx context.Context, w http.ResponseWriter) (string, bool) {
	userID := middleware.UserID(ctx)
	if userID == "" {
		WriteError(w, http.StatusUnauthorized, "unauthorized")
		return "", false
	}
	return userID, true
}
