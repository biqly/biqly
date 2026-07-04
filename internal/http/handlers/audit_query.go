package handlers

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/audit"
	"github.com/biqly/biqly/internal/query"
)

// AuditQueryHandler serves the query-audit prove-ability API: which policy
// decisions were applied to which executed query, linked to the exact
// compiled SQL in query_history.
type AuditQueryHandler struct {
	deps *app.QueryDeps
}

// NewAuditQueryHandler creates an AuditQueryHandler.
func NewAuditQueryHandler(deps *app.QueryDeps) *AuditQueryHandler {
	return &AuditQueryHandler{deps: deps}
}

// List returns recent query execution audit events, newest first, scoped to
// the caller's workspace datasources.
func (h *AuditQueryHandler) List(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	events, err := h.deps.AuditReader.ListQueryExecutionEvents(r.Context(), limit)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to list query audit events", err)
		return
	}
	wsFilter, applied, err := resolveDatasourceScope(r.Context(), h.deps.Config, true)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to resolve datasource scope", "err", err)
		wsFilter = map[string]struct{}{}
		applied = true
	}
	if applied {
		scoped := make([]audit.Event, 0, len(events))
		for i := range events {
			if _, ok := wsFilter[events[i].DatasourceID]; ok {
				scoped = append(scoped, events[i])
			}
		}
		events = scoped
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": events})
}

// Detail returns the audit event and the linked query history row for one
// executed query, keyed by the query_history id.
func (h *AuditQueryHandler) Detail(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	entry, err := h.deps.MetaRepo.GetQueryHistory(r.Context(), id)
	if err != nil {
		writeEntityNotFound(w, "query history")
		return
	}
	if !h.historyInScope(w, r, entry) {
		return
	}
	event, err := h.deps.AuditReader.QueryExecutionEvent(r.Context(), id)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to load query audit event", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"history": entry,
		"audit":   event,
	})
}

func (h *AuditQueryHandler) historyInScope(w http.ResponseWriter, r *http.Request, entry *query.HistoryEntry) bool {
	wsFilter, applied, err := resolveDatasourceScope(r.Context(), h.deps.Config, true)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to resolve datasource scope", "err", err)
		wsFilter = map[string]struct{}{}
		applied = true
	}
	if applied {
		if _, ok := wsFilter[entry.DatasourceID]; !ok {
			writeEntityNotFound(w, "query history")
			return false
		}
	}
	return true
}
