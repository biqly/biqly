package handlers

import (
	"log/slog"
	"net/http"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/audit"
	bimw "github.com/biqly/biqly/internal/http/middleware"
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

// List returns one page of query execution audit events, newest first, scoped
// to the caller's workspace datasources. Pagination and free-text search run
// in SQL so large logs stay browsable (`page`, `page_size`, `search`).
func (h *AuditQueryHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	wsFilter, applied, err := resolveDatasourceScope(ctx, h.deps.Config, true)
	if err != nil {
		slog.ErrorContext(ctx, "failed to resolve datasource scope", "err", err)
		wsFilter = map[string]struct{}{}
		applied = true
	}
	datasourceIDs := make([]string, 0, len(wsFilter))
	for id := range wsFilter {
		datasourceIDs = append(datasourceIDs, id)
	}
	pag := bimw.PaginationFromContext(ctx)
	events, total, err := h.deps.AuditReader.ListQueryExecutionEventsPage(ctx, audit.QueryEventPage{
		Limit:         pag.Limit,
		Offset:        pag.Offset,
		Search:        r.URL.Query().Get("search"),
		Scoped:        applied,
		DatasourceIDs: datasourceIDs,
	})
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to list query audit events", err)
		return
	}
	if events == nil {
		events = []audit.Event{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": events, "total": total})
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
