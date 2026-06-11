package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/core"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
	"github.com/biqly/biqly/pkg/internalapi"
)

// InternalQueryHandler exposes /internal/query/* endpoints used by AI Service
// (and any future caller) to compile and execute LogicalQueries through the
// same pipeline /api/query/* uses. The wire shape lives in pkg/internalapi
// so peer services can vendor the request/response types without depending
// on internal/*.
//
// Phase 1 reuses the monolith's core.QueryService end-to-end; Phase 3 extracts
// these handlers into the Query Engine binary unchanged.
type InternalQueryHandler struct {
	query   internalQueryRunner
	metrics QueryMetricsRecorder
}

// NewInternalQueryHandler returns a handler ready to be mounted under
// /internal/query.
func NewInternalQueryHandler(deps *app.QueryDeps) *InternalQueryHandler {
	return &InternalQueryHandler{query: deps.QueryService}
}

// SetQueryMetricsRecorder wires process-level Query metrics.
func (h *InternalQueryHandler) SetQueryMetricsRecorder(m QueryMetricsRecorder) {
	h.metrics = m
}

// Compile validates a LogicalQuery against the published semantic model and
// returns the parameterized SQL. It DOES NOT execute the query, so it is
// safe to call from any caller that needs a deterministic SQL fingerprint
// without touching user data.
func (h *InternalQueryHandler) compileToSQL(w http.ResponseWriter, r *http.Request, lq *query.LogicalQuery, inline *semantic.SemanticModel) (string, []any, string, bool) {
	result, ok := h.compileLogicalQuery(w, r, lq, inline)
	if !ok {
		return "", nil, "", false
	}
	fingerprint, err := fingerprintFor(&result.LogicalQuery, result.Model)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "query failed", err)
		return "", nil, "", false
	}
	return result.Compiled.SQL, result.Compiled.Args, fingerprint, true
}

// Compile validates a LogicalQuery against the published semantic model and
// returns the parameterized SQL. It DOES NOT execute the query, so it is
// safe to call from any caller that needs a deterministic SQL fingerprint
// without touching user data.
func (h *InternalQueryHandler) Compile(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[internalapi.CompileRequest](w, r)
	if !ok {
		return
	}
	sql, args, fingerprint, ok := h.compileToSQL(w, r, &req.LogicalQuery, req.Model)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, internalapi.CompileResponse{
		SQL:         sql,
		Args:        args,
		Fingerprint: fingerprint,
	})
}

// Run validates, compiles, and executes a LogicalQuery. The executor enforces
// its configured row/timeout limits; any per-request overrides in the body
// are advisory only — the global ceiling always wins.
func (h *InternalQueryHandler) Run(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[internalapi.RunRequest](w, r)
	if !ok {
		return
	}
	start := time.Now()
	result, se := h.query.RunWithModel(r.Context(), &req.LogicalQuery, req.Model)
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
	if result == nil || result.Compiled == nil {
		writeInternalError(
			r.Context(),
			w,
			http.StatusInternalServerError,
			"query failed",
			errors.New("query run returned nil result"),
		)
		return
	}
	fingerprint, err := fingerprintFor(&result.LogicalQuery, result.Model)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "query failed", err)
		return
	}
	resp := internalapi.RunResponse{
		Fingerprint: fingerprint,
		SQL:         result.Compiled.SQL,
	}
	if result.Result != nil {
		resp.Columns = result.Result.Columns
		resp.Rows = result.Result.Rows
		resp.RowCount = result.Result.Stats.RowCount
		resp.DurationMs = result.Result.Stats.DurationMs
	}
	writeJSON(w, http.StatusOK, resp)
}

// DryRun validates and compiles a LogicalQuery without executing it. The
// response is shape-identical to Compile but the endpoint is named distinctly
// so audit logs / metrics can distinguish "dry-run requested" from "compile
// requested as part of a run pipeline".
func (h *InternalQueryHandler) DryRun(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[internalapi.DryRunRequest](w, r)
	if !ok {
		return
	}
	sql, args, fingerprint, ok := h.compileToSQL(w, r, &req.LogicalQuery, req.Model)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, internalapi.DryRunResponse{
		SQL:         sql,
		Args:        args,
		Fingerprint: fingerprint,
	})
}

func (h *InternalQueryHandler) compileLogicalQuery(
	w http.ResponseWriter,
	r *http.Request,
	lq *query.LogicalQuery,
	inline *semantic.SemanticModel,
) (*core.CompileResult, bool) {
	start := time.Now()
	result, se := h.query.CompileWithModel(r.Context(), lq, inline)
	if h.metrics != nil {
		h.metrics.RecordQueryCompile(time.Since(start).Milliseconds(), se == nil)
	}
	if se != nil {
		writeServiceError(r.Context(), w, se)
		return nil, false
	}
	if result == nil || result.Compiled == nil {
		writeInternalError(
			r.Context(),
			w,
			http.StatusInternalServerError,
			"query failed",
			errors.New("query compile returned nil result"),
		)
		return nil, false
	}
	return result, true
}

// fingerprintFor produces the canonical fingerprint for a compiled query.
// The model's published version goes into ContextVersion so publishing a new
// semantic-model revision naturally invalidates downstream fingerprint caches.
// PermissionScope is left empty for the internal surface; row-level filter
// injection is handled by /api/query/* through CompileWithPermissions.
func fingerprintFor(lq *query.LogicalQuery, model *semantic.SemanticModel) (string, error) {
	in := query.FingerprintInputs{
		LogicalQuery: lq,
		DatasourceID: lq.DatasourceID,
	}
	if model != nil {
		in.ContextVersion = fmt.Sprintf("%s@%d", model.ID, model.Version)
	}
	return query.ComputeFingerprint(in)
}
