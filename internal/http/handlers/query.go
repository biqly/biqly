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
)

// QueryHandler handles query operations.
type QueryHandler struct {
	deps *app.Dependencies
}

// NewQueryHandler creates a new query handler.
func NewQueryHandler(deps *app.Dependencies) *QueryHandler {
	return &QueryHandler{deps: deps}
}

func (h *QueryHandler) Compile(w http.ResponseWriter, r *http.Request) {
	var lq query.LogicalQuery
	if err := json.NewDecoder(r.Body).Decode(&lq); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx := r.Context()
	model, err := h.loadModel(ctx, lq.ModelID)
	if err != nil {
		writeError(w, http.StatusNotFound, "semantic model not found")
		return
	}

	// Validate
	if err := h.deps.Validator.Validate(lq, model); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Get datasource to find driver
	ds, err := h.deps.MetaRepo.GetDatasource(ctx, lq.DatasourceID)
	if err != nil {
		writeError(w, http.StatusNotFound, "datasource not found")
		return
	}

	driver, err := h.deps.DriverReg.Get(ds.Type)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("unsupported driver: %s", ds.Type))
		return
	}

	// Compile
	compiler := query.NewCompiler(driver.Dialect())
	cq, err := compiler.Compile(ctx, lq, model)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("compilation failed: %s", err.Error()))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"sql":  cq.SQL,
		"args": cq.Args,
	})
}

func (h *QueryHandler) Run(w http.ResponseWriter, r *http.Request) {
	var lq query.LogicalQuery
	if err := json.NewDecoder(r.Body).Decode(&lq); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx := r.Context()
	model, err := h.loadModel(ctx, lq.ModelID)
	if err != nil {
		writeError(w, http.StatusNotFound, "semantic model not found")
		return
	}

	// Validate
	if err := h.deps.Validator.Validate(lq, model); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Get datasource
	ds, err := h.deps.MetaRepo.GetDatasource(ctx, lq.DatasourceID)
	if err != nil {
		writeError(w, http.StatusNotFound, "datasource not found")
		return
	}

	driver, err := h.deps.DriverReg.Get(ds.Type)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("unsupported driver: %s", ds.Type))
		return
	}

	// Compile
	compiler := query.NewCompiler(driver.Dialect())
	cq, err := compiler.Compile(ctx, lq, model)
	if err != nil {
		slog.ErrorContext(ctx, "compile failed", "error", err)
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("compilation failed: %s", err.Error()))
		return
	}

	// Execute
	db, err := driver.Open(ctx, ds.DSNEncrypted)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("connection failed: %s", err.Error()))
		return
	}
	defer db.Close()

	result, err := h.deps.Executor.Execute(ctx, db, cq)
	if err != nil {
		slog.ErrorContext(ctx, "execute failed", "error", err)
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("execution failed: %s", err.Error()))
		return
	}

	slog.InfoContext(ctx, "query executed",
		"duration_ms", result.Stats.DurationMs,
		"rows", result.Stats.RowCount,
	)

	writeJSON(w, http.StatusOK, result)
}

func (h *QueryHandler) Explain(w http.ResponseWriter, r *http.Request) {
	var lq query.LogicalQuery
	if err := json.NewDecoder(r.Body).Decode(&lq); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx := r.Context()
	model, err := h.loadModel(ctx, lq.ModelID)
	if err != nil {
		writeError(w, http.StatusNotFound, "semantic model not found")
		return
	}

	// Validate
	if err := h.deps.Validator.Validate(lq, model); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ds, err := h.deps.MetaRepo.GetDatasource(ctx, lq.DatasourceID)
	if err != nil {
		writeError(w, http.StatusNotFound, "datasource not found")
		return
	}

	driver, err := h.deps.DriverReg.Get(ds.Type)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("unsupported driver: %s", ds.Type))
		return
	}

	compiler := query.NewCompiler(driver.Dialect())
	cq, err := compiler.Compile(ctx, lq, model)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("compilation failed: %s", err.Error()))
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"logical_query": lq,
		"semantic_model": model,
		"compiled_sql":  cq.SQL,
		"args":          cq.Args,
	})
}

func (h *QueryHandler) History(w http.ResponseWriter, r *http.Request) {
	// TODO: implement query history
	writeJSON(w, http.StatusOK, []any{})
}

func (h *QueryHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	// TODO: implement get single history entry
	writeError(w, http.StatusNotFound, "not implemented")
}

func (h *QueryHandler) loadModel(ctx context.Context, modelID string) (*semantic.SemanticModel, error) {
	if modelID == "" {
		return nil, fmt.Errorf("model_id is required")
	}
	return h.deps.SemanticRepo.GetFullModel(ctx, modelID)
}

