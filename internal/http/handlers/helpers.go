package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"reflect"

	"github.com/biqly/biqly/internal/core"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/biqly/biqly/pkg/common/requestid"
	"github.com/go-chi/chi/v5"
)

// Response is a generic JSON response.
type Response struct {
	Error   string `json:"error,omitempty"`
	Message string `json:"message,omitempty"`
	Data    any    `json:"data,omitempty"`
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, status int, data any) {
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

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	resp := Response{Error: message}
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		slog.Error("failed to encode error response", "error", err)
	}
}

func writeInternalError(ctx context.Context, w http.ResponseWriter, status int, publicMsg string, err error, args ...any) {
	if err != nil {
		allArgs := append([]any{"error", err}, args...)
		if reqID := requestid.FromContext(ctx); reqID != "" {
			allArgs = append(allArgs, "request_id", reqID)
		}
		if userID := bimw.UserID(ctx); userID != "" {
			allArgs = append(allArgs, "user_id", userID)
		}
		if wsID := bimw.WorkspaceID(ctx); wsID != "" {
			allArgs = append(allArgs, "workspace_id", wsID)
		}
		slog.ErrorContext(ctx, publicMsg, allArgs...)
	}
	writeError(w, status, publicMsg)
}

// writeEntityNotFound writes a standardized 404 response of the form
// "<entity> not found".
func writeEntityNotFound(w http.ResponseWriter, entity string) {
	writeError(w, http.StatusNotFound, entity+" not found")
}

func writeServiceError(ctx context.Context, w http.ResponseWriter, se *core.ServiceError, args ...any) {
	if se == nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	if se.Status >= http.StatusInternalServerError {
		allArgs := append([]any{"error", core.LogCause(se)}, args...)
		if reqID := requestid.FromContext(ctx); reqID != "" {
			allArgs = append(allArgs, "request_id", reqID)
		}
		if userID := bimw.UserID(ctx); userID != "" {
			allArgs = append(allArgs, "user_id", userID)
		}
		if wsID := bimw.WorkspaceID(ctx); wsID != "" {
			allArgs = append(allArgs, "workspace_id", wsID)
		}
		slog.ErrorContext(ctx, se.Message, allArgs...)
	}
	writeError(w, se.Status, se.Message)
}

func writeCoreServiceError(ctx context.Context, w http.ResponseWriter, err error, args ...any) {
	writeServiceError(ctx, w, core.MapQueryServiceError(err), args...)
}

// maxJSONRequestBytes caps the size of incoming JSON bodies. Keeps a buggy
// or malicious client from forcing the server to buffer a multi-GB POST.
// 1 MiB is generous for every endpoint here (LogicalQuery, semantic models,
// few-shot examples) and well below typical proxy / chi defaults.
const maxJSONRequestBytes = 1 << 20 // 1 MiB

func decodeJSON[T any](w http.ResponseWriter, r *http.Request) (*T, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONRequestBytes)
	var v T
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		if isMaxBytesError(err) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return nil, false
	}
	return &v, true
}

func decodeJSONAllowEmpty[T any](w http.ResponseWriter, r *http.Request) (*T, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONRequestBytes)
	var v T
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil && !errors.Is(err, io.EOF) {
		if isMaxBytesError(err) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
			return nil, false
		}
		writeError(w, http.StatusBadRequest, "invalid request body")
		return nil, false
	}
	return &v, true
}

func isMaxBytesError(err error) bool {
	var maxErr *http.MaxBytesError
	return errors.As(err, &maxErr)
}

func requireURLParam(w http.ResponseWriter, r *http.Request, key string) (string, bool) {
	v := chi.URLParam(r, key)
	if v == "" {
		writeError(w, http.StatusBadRequest, key+" is required")
		return "", false
	}
	return v, true
}
