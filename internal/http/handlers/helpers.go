package handlers

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/biqly/biqly/internal/core"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/biqly/biqly/internal/http/response"
	"github.com/biqly/biqly/pkg/common/requestid"
	"github.com/go-chi/chi/v5"
)

// writeJSON writes a JSON response. Thin wrapper over the shared response
// package so the wire format and nil-slice normalization stay consistent
// across handler packages.
func writeJSON(w http.ResponseWriter, status int, data any) {
	response.WriteJSON(w, status, data)
}

// writeError writes a JSON error response.
func writeError(w http.ResponseWriter, status int, message string) {
	response.WriteError(w, status, message)
}

// writeOK writes a JSON success response of the form {"status": "ok"} with status 200 OK.
func writeOK(w http.ResponseWriter) {
	response.WriteOK(w)
}

func writeInternalError(ctx context.Context, w http.ResponseWriter, status int, publicMsg string, err error, args ...any) {
	// request_id is attached by response.WriteInternalError; add the
	// middleware-derived identity fields specific to this handler package.
	if userID := bimw.UserID(ctx); userID != "" {
		args = append(args, "user_id", userID)
	}
	if wsID := bimw.WorkspaceID(ctx); wsID != "" {
		args = append(args, "workspace_id", wsID)
	}
	response.WriteInternalError(ctx, w, status, publicMsg, err, args...)
}

func appendRequestLogArgs(ctx context.Context, args []any) []any {
	if reqID := requestid.FromContext(ctx); reqID != "" {
		args = append(args, "request_id", reqID)
	}
	if userID := bimw.UserID(ctx); userID != "" {
		args = append(args, "user_id", userID)
	}
	if wsID := bimw.WorkspaceID(ctx); wsID != "" {
		args = append(args, "workspace_id", wsID)
	}
	return args
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
		allArgs = appendRequestLogArgs(ctx, allArgs)
		slog.ErrorContext(ctx, se.Message, allArgs...)
	}
	writeError(w, se.Status, se.Message)
}

func writeCoreServiceError(ctx context.Context, w http.ResponseWriter, err error, args ...any) {
	writeServiceError(ctx, w, core.MapQueryServiceError(err), args...)
}

func decodeJSON[T any](w http.ResponseWriter, r *http.Request) (*T, bool) {
	return response.DecodeJSON[T](w, r)
}

func decodeJSONAllowEmpty[T any](w http.ResponseWriter, r *http.Request) (*T, bool) {
	return response.DecodeJSONAllowEmpty[T](w, r)
}

// readRequestBody buffers the size-capped request body for handlers that need
// to decode parts of it separately (e.g. strict per-domain decoding).
func readRequestBody(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, response.MaxJSONRequestBytes)
	body, err := io.ReadAll(r.Body)
	if err != nil {
		if isMaxBytesError(err) {
			writeError(w, http.StatusRequestEntityTooLarge, "request body too large")
		} else {
			writeError(w, http.StatusBadRequest, "invalid request body")
		}
		return nil, false
	}
	return body, true
}

func isMaxBytesError(err error) bool {
	_, ok := errors.AsType[*http.MaxBytesError](err)
	return ok
}

func requireURLParam(w http.ResponseWriter, r *http.Request, key string) (string, bool) {
	v := chi.URLParam(r, key)
	if v == "" {
		writeError(w, http.StatusBadRequest, key+" is required")
		return "", false
	}
	return v, true
}

func requireQueryParam(w http.ResponseWriter, r *http.Request, key string) (string, bool) {
	v := strings.TrimSpace(r.URL.Query().Get(key))
	if v == "" {
		writeError(w, http.StatusBadRequest, key+" is required")
		return "", false
	}
	return v, true
}
