package handlers

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
	"github.com/bytedance/sonic"
)

// QueryHandler handles query operations.
type QueryHandler struct {
	deps    *app.QueryDeps
	query   internalQueryRunner
	metrics QueryMetricsRecorder
}

type queryPayload struct {
	LogicalQuery query.LogicalQuery      `json:"logical_query"`
	Model        *semantic.SemanticModel `json:"model,omitempty"`
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
	payload, ok := decodeQueryPayload(w, r)
	if !ok {
		return
	}

	start := time.Now()
	compiled, se := h.query.CompileWithModel(r.Context(), &payload.LogicalQuery, payload.Model)
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
	payload, ok := decodeQueryPayload(w, r)
	if !ok {
		return
	}

	start := time.Now()
	result, se := h.query.RunWithModel(r.Context(), &payload.LogicalQuery, payload.Model)
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
	payload, ok := decodeQueryPayload(w, r)
	if !ok {
		return
	}

	start := time.Now()
	compiled, se := h.query.CompileWithModel(r.Context(), &payload.LogicalQuery, payload.Model)
	if h.metrics != nil {
		h.metrics.RecordQueryCompile(time.Since(start).Milliseconds(), se == nil)
	}
	if se != nil {
		writeServiceError(r.Context(), w, se)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"logical_query":  payload.LogicalQuery,
		"semantic_model": compiled.Model,
		"compiled_sql":   compiled.Compiled.SQL,
		"args":           compiled.Compiled.Args,
	})
}

// MetricQuery accepts a structured metric query (measures, dimensions, filters)
// without natural language, converts it to a LogicalQuery, and executes it
// through the same governed run path as Run.
func (h *QueryHandler) MetricQuery(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[query.StructuredMetricQuery](w, r)
	if !ok {
		return
	}
	lq, err := req.ToLogicalQuery()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	start := time.Now()
	result, se := h.query.RunWithModel(r.Context(), &lq, nil)
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

	writeJSON(w, http.StatusOK, map[string]any{
		"logical_query": lq,
		"result":        result.Result,
	})
}

// DryRun validates a LogicalQuery against the target datasource without
// executing it. It returns the parameterized SQL, bind arguments, and canonical
// fingerprint through the public route's RBAC and datasource access checks.
func (h *QueryHandler) DryRun(w http.ResponseWriter, r *http.Request) {
	payload, ok := decodeQueryPayload(w, r)
	if !ok {
		return
	}

	start := time.Now()
	compiled, se := h.query.DryRunWithModel(r.Context(), &payload.LogicalQuery, payload.Model)
	if h.metrics != nil {
		h.metrics.RecordQueryCompile(time.Since(start).Milliseconds(), se == nil)
	}
	if se != nil {
		writeServiceError(r.Context(), w, se)
		return
	}

	fingerprint, err := fingerprintFor(&payload.LogicalQuery, compiled.Model)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "query failed", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"sql":         compiled.Compiled.SQL,
		"args":        compiled.Compiled.Args,
		"fingerprint": fingerprint,
	})
}

func decodeQueryPayload(w http.ResponseWriter, r *http.Request) (*queryPayload, bool) {
	body, ok := readRequestBody(w, r)
	if !ok {
		return nil, false
	}
	var wrapped queryPayload
	if err := sonic.ConfigStd.Unmarshal(body, &wrapped); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return nil, false
	}
	if wrapped.LogicalQuery.DatasourceID != "" || wrapped.Model != nil {
		return &wrapped, true
	}
	var legacy query.LogicalQuery
	if err := sonic.ConfigStd.Unmarshal(body, &legacy); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return nil, false
	}
	return &queryPayload{LogicalQuery: legacy}, true
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
