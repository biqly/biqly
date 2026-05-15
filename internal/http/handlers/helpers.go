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

func writeInternalError(ctx context.Context, w http.ResponseWriter, status int, publicMsg string, err error) {
	if err != nil {
		slog.ErrorContext(ctx, publicMsg, "error", err)
	}
	writeError(w, status, publicMsg)
}

func writeServiceError(ctx context.Context, w http.ResponseWriter, se *core.ServiceError) {
	if se == nil {
		writeError(w, http.StatusInternalServerError, "query failed")
		return
	}
	if se.Status >= http.StatusInternalServerError {
		slog.ErrorContext(ctx, se.Message, "error", core.LogCause(se))
	}
	writeError(w, se.Status, se.Message)
}

func writeCoreServiceError(ctx context.Context, w http.ResponseWriter, err error) {
	writeServiceError(ctx, w, core.MapQueryServiceError(err))
}

func decodeJSON[T any](w http.ResponseWriter, r *http.Request) (*T, bool) {
	var v T
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return nil, false
	}
	return &v, true
}

func decodeJSONAllowEmpty[T any](w http.ResponseWriter, r *http.Request) (*T, bool) {
	var v T
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return nil, false
	}
	return &v, true
}

func requireURLParam(w http.ResponseWriter, r *http.Request, key string) (string, bool) {
	v := chi.URLParam(r, key)
	if v == "" {
		writeError(w, http.StatusBadRequest, key+" is required")
		return "", false
	}
	return v, true
}
