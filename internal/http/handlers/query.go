package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
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

	ctx := r.Context()
	model, modelErr := h.loadModel(ctx, lq.ModelID)
	if modelErr != nil {
		writeError(w, http.StatusNotFound, "semantic model not found")
		return
	}

	// Validate
	if valErr := h.deps.Validator.Validate(lq, model); valErr != nil {
		writeError(w, http.StatusBadRequest, valErr.Error())
		return
	}

	// Get datasource to find driver
	ds, dsErr := h.deps.MetaRepo.GetDatasource(ctx, lq.DatasourceID)
	if dsErr != nil {
		writeError(w, http.StatusNotFound, "datasource not found")
		return
	}

	driver, driverErr := h.deps.DriverReg.Get(ds.Type)
	if driverErr != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("unsupported driver: %s", ds.Type))
		return
	}

	// Compile
	compiler := query.NewCompiler(driver.Dialect())
	cq, compileErr := compiler.Compile(ctx, lq, model)
	if compileErr != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("compilation failed: %s", compileErr.Error()))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"sql":  cq.SQL,
		"args": cq.Args,
	})
}

// Run validates, compiles, and executes a LogicalQuery.
func (h *QueryHandler) Run(w http.ResponseWriter, r *http.Request) {
	var lq query.LogicalQuery
	if err := json.NewDecoder(r.Body).Decode(&lq); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx := r.Context()
	model, modelErr := h.loadModel(ctx, lq.ModelID)
	if modelErr != nil {
		writeError(w, http.StatusNotFound, "semantic model not found")
		return
	}

	// Validate
	if valErr := h.deps.Validator.Validate(lq, model); valErr != nil {
		writeError(w, http.StatusBadRequest, valErr.Error())
		return
	}

	// Get datasource
	ds, dsErr := h.deps.MetaRepo.GetDatasource(ctx, lq.DatasourceID)
	if dsErr != nil {
		writeError(w, http.StatusNotFound, "datasource not found")
		return
	}

	driver, driverErr := h.deps.DriverReg.Get(ds.Type)
	if driverErr != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("unsupported driver: %s", ds.Type))
		return
	}

	// Compile
	compiler := query.NewCompiler(driver.Dialect())
	cq, compileErr := compiler.Compile(ctx, lq, model)
	if compileErr != nil {
		slog.ErrorContext(ctx, "compile failed", "error", compileErr)
		h.recordQueryHistory(ctx, lq, model, nil, nil, queryStatusFailed, compileErr)
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("compilation failed: %s", compileErr.Error()))
		return
	}

	// Execute
	db, dbErr := driver.Open(ctx, ds.DSNEncrypted)
	if dbErr != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("connection failed: %s", dbErr.Error()))
		return
	}
	defer func() { _ = db.Close() }()

	result, execErr := h.deps.Executor.Execute(ctx, db, cq)
	if execErr != nil {
		slog.ErrorContext(ctx, "execute failed", "error", execErr)
		h.recordQueryHistory(ctx, lq, model, cq, nil, queryStatusFailed, execErr)
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("execution failed: %s", execErr.Error()))
		return
	}

	slog.InfoContext(ctx, "query executed",
		"duration_ms", result.Stats.DurationMs,
		"rows", result.Stats.RowCount,
	)
	h.recordQueryHistory(ctx, lq, model, cq, result, queryStatusSuccess, nil)

	writeJSON(w, http.StatusOK, result)
}

// Explain returns the compiled SQL and metadata for debugging.
func (h *QueryHandler) Explain(w http.ResponseWriter, r *http.Request) {
	var lq query.LogicalQuery
	if err := json.NewDecoder(r.Body).Decode(&lq); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx := r.Context()
	model, modelErr := h.loadModel(ctx, lq.ModelID)
	if modelErr != nil {
		writeError(w, http.StatusNotFound, "semantic model not found")
		return
	}

	// Validate
	if valErr := h.deps.Validator.Validate(lq, model); valErr != nil {
		writeError(w, http.StatusBadRequest, valErr.Error())
		return
	}

	ds, dsErr := h.deps.MetaRepo.GetDatasource(ctx, lq.DatasourceID)
	if dsErr != nil {
		writeError(w, http.StatusNotFound, "datasource not found")
		return
	}

	driver, driverErr := h.deps.DriverReg.Get(ds.Type)
	if driverErr != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("unsupported driver: %s", ds.Type))
		return
	}

	compiler := query.NewCompiler(driver.Dialect())
	cq, compileErr := compiler.Compile(ctx, lq, model)
	if compileErr != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("compilation failed: %s", compileErr.Error()))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"logical_query":  lq,
		"semantic_model": model,
		"compiled_sql":   cq.SQL,
		"args":           cq.Args,
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

func (h *QueryHandler) loadModel(ctx context.Context, modelID string) (*semantic.SemanticModel, error) {
	if modelID == "" {
		return nil, fmt.Errorf("model_id is required")
	}
	return h.deps.SemanticRepo.GetFullModel(ctx, modelID)
}
