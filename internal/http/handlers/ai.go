// Package handlers provides HTTP handlers for the BI query engine API.
package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
)

// AIHandler handles AI text-to-query operations.
type AIHandler struct {
	service *ai.Service
	deps    *app.Dependencies
}

// NewAIHandler creates a new AI handler.
func NewAIHandler(deps *app.Dependencies) *AIHandler {
	svc := ai.NewService(deps.Config.AI, deps.Validator)
	return &AIHandler{
		service: svc,
		deps:    deps,
	}
}

type aiQueryRequest struct {
	DatasourceID string `json:"datasource_id"`
	ModelID      string `json:"model_id"`
	Question     string `json:"question"`
}

// Query handles AI-powered natural language queries.
func (h *AIHandler) Query(w http.ResponseWriter, r *http.Request) {
	var req aiQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Question == "" {
		writeError(w, http.StatusBadRequest, "question is required")
		return
	}

	ctx := r.Context()
	model, err := h.loadModel(ctx, req.DatasourceID, req.ModelID)
	if err != nil {
		writeError(w, http.StatusNotFound, "semantic model not found")
		return
	}

	resp, err := h.service.ProcessQuestion(ctx, req.Question, model)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, resp)
}

// Preview handles AI query preview (compiles but does not execute).
func (h *AIHandler) Preview(w http.ResponseWriter, r *http.Request) {
	var req aiQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx := r.Context()
	model, err := h.loadModel(ctx, req.DatasourceID, req.ModelID)
	if err != nil {
		writeError(w, http.StatusNotFound, "semantic model not found")
		return
	}

	// Get AI response
	resp, err := h.service.ProcessQuestion(ctx, req.Question, model)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if resp.LogicalQuery == nil {
		writeJSON(w, http.StatusOK, resp)
		return
	}

	// Compile to SQL for preview
	ds, err := h.deps.MetaRepo.GetDatasource(ctx, req.DatasourceID)
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
	cq, err := compiler.Compile(ctx, *resp.LogicalQuery, model)
	if err != nil {
		resp.Warnings = append(resp.Warnings, "compilation failed: "+err.Error())
	} else {
		resp.SQL = cq.SQL
		resp.Args = cq.Args
	}

	writeJSON(w, http.StatusOK, resp)
}

// Run handles AI query execution (compiles and executes).
func (h *AIHandler) Run(w http.ResponseWriter, r *http.Request) {
	var req aiQueryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	ctx := r.Context()
	model, err := h.loadModel(ctx, req.DatasourceID, req.ModelID)
	if err != nil {
		writeError(w, http.StatusNotFound, "semantic model not found")
		return
	}

	// Get AI response
	resp, err := h.service.ProcessQuestion(ctx, req.Question, model)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if resp.LogicalQuery == nil {
		writeJSON(w, http.StatusOK, resp)
		return
	}

	// Get datasource and execute
	ds, err := h.deps.MetaRepo.GetDatasource(ctx, req.DatasourceID)
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
	cq, err := compiler.Compile(ctx, *resp.LogicalQuery, model)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("compilation failed: %s", err.Error()))
		return
	}

	resp.SQL = cq.SQL
	resp.Args = cq.Args

	// Execute
	db, err := driver.Open(ctx, ds.DSNEncrypted)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("connection failed: %s", err.Error()))
		return
	}
	defer func() {
		if closeErr := db.Close(); closeErr != nil {
			slog.Error("failed to close database connection", "error", closeErr)
		}
	}()

	result, err := h.deps.Executor.Execute(ctx, db, cq)
	if err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("execution failed: %s", err.Error()))
		return
	}

	resp.Result = result
	writeJSON(w, http.StatusOK, resp)
}

// Describe runs the AI metadata describer over a single table and (optionally) writes
// the suggested table/column descriptions back into the metadata DB.
// Describe handles AI-powered table/column description generation.
func (h *AIHandler) Describe(w http.ResponseWriter, r *http.Request) {
	var req ai.DescribeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.DatasourceID == "" || req.Table == "" {
		writeError(w, http.StatusBadRequest, "datasource_id and table are required")
		return
	}

	result, err := h.deps.AIDescriber.Describe(r.Context(), req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *AIHandler) loadModel(ctx context.Context, datasourceID, modelID string) (*semantic.SemanticModel, error) {
	model, err := h.deps.SemanticRepo.GetModelByName(ctx, datasourceID, modelID)
	if err != nil {
		slog.ErrorContext(ctx, "load semantic model failed", "datasource_id", datasourceID, "model_id", modelID, "error", err)
		return nil, err
	}
	model.Dimensions, err = h.deps.SemanticRepo.GetDimensions(ctx, model.ID)
	if err != nil {
		slog.ErrorContext(ctx, "load semantic dimensions failed", "model_id", model.ID, "error", err)
		return nil, err
	}
	model.Metrics, err = h.deps.SemanticRepo.GetMetrics(ctx, model.ID)
	if err != nil {
		slog.ErrorContext(ctx, "load semantic metrics failed", "model_id", model.ID, "error", err)
		return nil, err
	}
	model.Joins, err = h.deps.SemanticRepo.GetJoins(ctx, model.ID)
	if err != nil {
		slog.ErrorContext(ctx, "load semantic joins failed", "model_id", model.ID, "error", err)
		return nil, err
	}
	return model, nil
}
