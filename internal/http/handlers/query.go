package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/query"
	"github.com/go-chi/chi/v5"
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
	var lq query.LogicalQuery
	if err := json.NewDecoder(r.Body).Decode(&lq); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	compiled, err := h.deps.QueryService.Compile(r.Context(), lq)
	if err != nil {
		writeQueryServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"sql":  compiled.Compiled.SQL,
		"args": compiled.Compiled.Args,
	})
}

// Run validates, compiles, and executes a LogicalQuery.
func (h *QueryHandler) Run(w http.ResponseWriter, r *http.Request) {
	var lq query.LogicalQuery
	if err := json.NewDecoder(r.Body).Decode(&lq); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	result, err := h.deps.QueryService.Run(r.Context(), lq)
	if err != nil {
		writeQueryServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, result.Result)
}

// Explain returns the compiled SQL and metadata for debugging.
func (h *QueryHandler) Explain(w http.ResponseWriter, r *http.Request) {
	var lq query.LogicalQuery
	if err := json.NewDecoder(r.Body).Decode(&lq); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	compiled, err := h.deps.QueryService.Compile(r.Context(), lq)
	if err != nil {
		writeQueryServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"logical_query":  lq,
		"semantic_model": compiled.Model,
		"compiled_sql":   compiled.Compiled.SQL,
		"args":           compiled.Compiled.Args,
	})
}

// History returns query history.
func (h *QueryHandler) History(w http.ResponseWriter, r *http.Request) {
	entries, err := h.deps.MetaRepo.ListQueryHistory(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list query history")
		return
	}
	writeJSON(w, http.StatusOK, entries)
}

// GetHistory returns a single history entry.
func (h *QueryHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	entry, err := h.deps.MetaRepo.GetQueryHistory(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "query history not found")
		return
	}
	writeJSON(w, http.StatusOK, entry)
}

func writeQueryServiceError(w http.ResponseWriter, err error) {
	msg := err.Error()
	switch {
	case strings.Contains(msg, "model_id is required"), strings.Contains(msg, "datasource_id is required"), strings.Contains(msg, "validation failed"):
		writeError(w, http.StatusBadRequest, msg)
	case strings.Contains(msg, "load semantic model"), strings.Contains(msg, "load datasource"):
		writeError(w, http.StatusNotFound, msg)
	case strings.Contains(msg, "load driver"):
		writeError(w, http.StatusBadRequest, msg)
	default:
		writeError(w, http.StatusInternalServerError, msg)
	}
}
