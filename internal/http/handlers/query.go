package handlers

import (
	"net/http"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/query"
)

// QueryHandler handles query operations.
type QueryHandler struct {
	deps *app.Dependencies
}

// NewQueryHandler creates a new query handler.
func NewQueryHandler(deps *app.Dependencies) *QueryHandler {
	return &QueryHandler{deps: deps}
}

// Compile validates and compiles a LogicalQuery into SQL.
func (h *QueryHandler) Compile(w http.ResponseWriter, r *http.Request) {
	lq, ok := decodeJSON[query.LogicalQuery](w, r)
	if !ok {
		return
	}

	compiled, se := h.deps.QueryService.Compile(r.Context(), *lq)
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

	result, se := h.deps.QueryService.Run(r.Context(), *lq)
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

	compiled, se := h.deps.QueryService.Compile(r.Context(), *lq)
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

// History returns query history.
func (h *QueryHandler) History(w http.ResponseWriter, r *http.Request) {
	entries, err := h.deps.MetaRepo.ListQueryHistory(r.Context(), h.deps.Config.Query.HistoryListLimit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list query history")
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

// GetHistory returns a single history entry.
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
	writeJSON(w, http.StatusOK, entry)
}

