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
