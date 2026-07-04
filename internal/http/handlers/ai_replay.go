package handlers

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/biqly/biqly/internal/metadata"
)

// ReplayAIHistory re-runs the NL→LogicalQuery generation for a stored history
// entry with the same question, datasource and model scope. It goes through
// the full pipeline (routing, prompt, LLM, validation), so it spends tokens
// and records a fresh history row — the original entry is never mutated.
func (h *AIHandler) ReplayAIHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	row, err := h.deps.MetaRepo.GetAIQueryHistoryByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "entry not found")
			return
		}
		writeInternalError(ctx, w, http.StatusInternalServerError, "get AI history failed", err)
		return
	}

	userID := bimw.UserID(ctx)
	hasViewDetails := canViewAIHistoryDetails(ctx, h.authClient, userID)
	if !hasViewDetails && userID != "" && (row.UserID == nil || *row.UserID != userID) {
		writeError(w, http.StatusForbidden, "not owner of this entry")
		return
	}
	wsFilter, applied, err := resolveDatasourceScope(ctx, h.deps.Config, true)
	if err != nil {
		slog.ErrorContext(ctx, "replay: failed to resolve datasource scope", "err", err)
		wsFilter = map[string]struct{}{}
		applied = true
	}
	if applied {
		if _, ok := wsFilter[row.DatasourceID]; !ok {
			writeError(w, http.StatusNotFound, "entry not found")
			return
		}
	}

	req := replayRequestFromHistory(row)
	if req.Question == "" || req.DatasourceID == "" {
		writeError(w, http.StatusUnprocessableEntity, "history entry cannot be replayed")
		return
	}

	workspaceID := bimw.WorkspaceID(ctx)
	if err := h.deps.SpendLimiter.Check(ctx, workspaceID); err != nil {
		writeError(w, http.StatusTooManyRequests, "workspace AI token budget exceeded for today")
		return
	}

	req, pc, model, routeResult, ok := h.routeAIQueryRequest(ctx, w, req)
	if !ok {
		return
	}
	resp, err := h.processAIQuestion(ctx, pc, req, model, routeResult)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to replay question", err,
			"replay_of", id,
			"question", req.Question,
			"datasource_id", req.DatasourceID,
		)
		return
	}
	if resp != nil && resp.Metadata != nil && resp.Metadata.TokenUsage != nil {
		h.deps.SpendLimiter.Record(ctx, workspaceID, resp.Metadata.TokenUsage.Total)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"replay_of": id,
		"response":  resp,
	})
}

// replayRequestFromHistory rebuilds the original request from a history row:
// the question and datasource come from dedicated columns, the model id and
// selected table scope from the persisted prompt_context.
func replayRequestFromHistory(row *metadata.AIQueryHistoryEntry) aiQueryRequest {
	req := aiQueryRequest{
		DatasourceID: row.DatasourceID,
		Question:     row.Question,
	}
	ctxMap, ok := row.PromptContext.(map[string]any)
	if !ok {
		return req
	}
	if v, ok := ctxMap["model_id"].(string); ok {
		req.ModelID = v
	}
	if scope, ok := ctxMap["selected_scope"].([]any); ok {
		for _, t := range scope {
			if s, ok := t.(string); ok {
				req.Tables = append(req.Tables, s)
			}
		}
	}
	return req
}
