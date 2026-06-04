package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"github.com/biqly/biqly/internal/config"
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

// resolveAccessibleDatasources returns the set of datasource IDs the current
// caller may access, intersected with the active workspace's attached
// datasources. The boolean reports whether scoping applies: it is false when
// auth is disabled, the request carries no user, or the caller is a super
// admin — in those cases callers must not filter their results. On error the
// helper returns it for the caller to surface with a context-appropriate
// message; nothing is written to w.
func resolveAccessibleDatasources(ctx context.Context, config *config.Config) (map[string]struct{}, bool, error) {
	if !config.Auth.Enabled {
		return nil, false, nil
	}
	userID := bimw.UserID(ctx)
	if userID == "" || bimw.HasRole(ctx, bimw.RoleSuperAdmin) {
		return nil, false, nil
	}

	authClient := bimw.NewAuthClient(config.Auth.ServiceURL, config.Auth.InternalToken)
	allowed, err := authClient.ListUserDatasources(ctx, userID)
	if err != nil {
		return nil, false, fmt.Errorf("list user datasources: %w", err)
	}
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, id := range allowed {
		allowedSet[id] = struct{}{}
	}

	if wsID := bimw.WorkspaceID(ctx); wsID != "" {
		wsIDs, err := authClient.ListWorkspaceDatasources(ctx, wsID)
		if err != nil {
			return nil, false, fmt.Errorf("list workspace datasources: %w", err)
		}
		wsSet := make(map[string]struct{}, len(wsIDs))
		for _, id := range wsIDs {
			wsSet[id] = struct{}{}
		}
		for id := range allowedSet {
			if _, ok := wsSet[id]; !ok {
				delete(allowedSet, id)
			}
		}
	}

	return allowedSet, true, nil
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
