package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/query"
)

// QueryHandler handles query operations.
type QueryHandler struct {
	deps    *app.QueryDeps
	query   internalQueryRunner
	metrics QueryMetricsRecorder
}

// NewQueryHandler creates a new query handler.
func NewQueryHandler(deps *app.QueryDeps) *QueryHandler {
	return &QueryHandler{deps: deps, query: deps.QueryService}
}

// SetQueryMetricsRecorder wires process-level Query metrics.
func (h *QueryHandler) SetQueryMetricsRecorder(m QueryMetricsRecorder) {
	h.metrics = m
}

// Compile validates and compiles a LogicalQuery into SQL.
func (h *QueryHandler) Compile(w http.ResponseWriter, r *http.Request) {
	lq, ok := decodeJSON[query.LogicalQuery](w, r)
	if !ok {
		return
	}

	start := time.Now()
	compiled, se := h.query.Compile(r.Context(), lq)
	if h.metrics != nil {
		h.metrics.RecordQueryCompile(time.Since(start).Milliseconds(), se == nil)
	}
	if se != nil {
		writeServiceError(r.Context(), w, se)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"sql":  compiled.Compiled.SQL,
		"args": compiled.Compiled.Args,
	})
}

// Run validates, compiles, and executes a LogicalQuery.
func (h *QueryHandler) Run(w http.ResponseWriter, r *http.Request) {
	lq, ok := decodeJSON[query.LogicalQuery](w, r)
	if !ok {
		return
	}

	start := time.Now()
	result, se := h.query.Run(r.Context(), lq)
	rows := 0
	if result != nil && result.Result != nil {
		rows = result.Result.Stats.RowCount
	}
	if h.metrics != nil {
		h.metrics.RecordQueryExecution(time.Since(start).Milliseconds(), se == nil, rows)
	}
	if se != nil {
		writeServiceError(r.Context(), w, se)
		return
	}

	writeJSON(w, http.StatusOK, result.Result)
}

// Explain returns the compiled SQL and metadata for debugging.
func (h *QueryHandler) Explain(w http.ResponseWriter, r *http.Request) {
	lq, ok := decodeJSON[query.LogicalQuery](w, r)
	if !ok {
		return
	}

	start := time.Now()
	compiled, se := h.query.Compile(r.Context(), lq)
	if h.metrics != nil {
		h.metrics.RecordQueryCompile(time.Since(start).Milliseconds(), se == nil)
	}
	if se != nil {
		writeServiceError(r.Context(), w, se)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"logical_query":  *lq,
		"semantic_model": compiled.Model,
		"compiled_sql":   compiled.Compiled.SQL,
		"args":           compiled.Compiled.Args,
	})
}

// History returns query history. When auth is enabled and the caller is not
// super_admin, results are scoped to the active workspace's datasource.
func (h *QueryHandler) History(w http.ResponseWriter, r *http.Request) {
	entries, err := h.deps.MetaRepo.ListQueryHistory(r.Context(), h.deps.Config.Query.HistoryListLimit)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to list query history", err)
		return
	}
	wsFilter, applied, err := resolveDatasourceScope(r.Context(), h.deps.Config, true)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to resolve datasource scope", "err", err)
		wsFilter = map[string]struct{}{}
		applied = true
	}
	if applied {
		entries = FilterQueryHistoryByDatasources(entries, wsFilter)
	}
	writeJSON(w, http.StatusOK, entries)
}

// GetHistory returns a single history entry. Scoped to the active workspace
// when auth is enabled and the caller is not super_admin.
func (h *QueryHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	entry, err := h.deps.MetaRepo.GetQueryHistory(r.Context(), id)
	if err != nil {
		writeEntityNotFound(w, "query history")
		return
	}
	wsFilter, applied, err := resolveDatasourceScope(r.Context(), h.deps.Config, true)
	if err != nil {
		slog.ErrorContext(r.Context(), "failed to resolve datasource scope", "err", err)
		wsFilter = map[string]struct{}{}
		applied = true
	}
	if applied {
		if _, ok := wsFilter[entry.DatasourceID]; !ok {
			writeEntityNotFound(w, "query history")
			return
		}
	}
	writeJSON(w, http.StatusOK, entry)
}
