package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/semantic"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// SemanticHandler handles semantic layer CRUD operations.
type SemanticHandler struct {
	deps *app.Dependencies
}

// NewSemanticHandler creates a new semantic handler.
func NewSemanticHandler(deps *app.Dependencies) *SemanticHandler {
	return &SemanticHandler{deps: deps}
}

type createModelRequest struct {
	DatasourceID string   `json:"datasource_id"`
	Name         string   `json:"name"`
	Label        string   `json:"label,omitempty"`
	Description  string   `json:"description,omitempty"`
	BaseSchema   string   `json:"base_schema"`
	BaseTable    string   `json:"base_table"`
	Synonyms     []string `json:"synonyms,omitempty"`
}

func (h *SemanticHandler) CreateModel(w http.ResponseWriter, r *http.Request) {
	var req createModelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	m := &semantic.SemanticModel{
		ID:           uuid.New().String(),
		DatasourceID: req.DatasourceID,
		Name:         req.Name,
		BaseSchema:   req.BaseSchema,
		BaseTable:    req.BaseTable,
		Synonyms:     req.Synonyms,
		IsActive:     true,
	}

	if req.Label != "" {
		m.Label = &req.Label
	}
	if req.Description != "" {
		m.Description = &req.Description
	}

	ctx := r.Context()
	if err := h.deps.SemanticRepo.CreateModel(ctx, m); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create model")
		return
	}

	writeJSON(w, http.StatusCreated, m)
}

func (h *SemanticHandler) ListModels(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	models, err := h.deps.SemanticRepo.ListModels(ctx, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list models")
		return
	}

	writeJSON(w, http.StatusOK, models)
}

func (h *SemanticHandler) GetModel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()

	model, err := h.deps.SemanticRepo.GetFullModel(ctx, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "model not found")
		return
	}

	writeJSON(w, http.StatusOK, model)
}

type updateModelRequest struct {
	Name        string   `json:"name"`
	Label       string   `json:"label,omitempty"`
	Description string   `json:"description,omitempty"`
	BaseSchema  string   `json:"base_schema"`
	BaseTable   string   `json:"base_table"`
	Synonyms    []string `json:"synonyms,omitempty"`
	IsActive    *bool    `json:"is_active,omitempty"`
}

func (h *SemanticHandler) UpdateModel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()

	existing, err := h.deps.SemanticRepo.GetModel(ctx, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "model not found")
		return
	}

	var req updateModelRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.BaseSchema != "" {
		existing.BaseSchema = req.BaseSchema
	}
	if req.BaseTable != "" {
		existing.BaseTable = req.BaseTable
	}
	if req.Synonyms != nil {
		existing.Synonyms = req.Synonyms
	}
	if req.Label != "" {
		existing.Label = &req.Label
	}
	if req.Description != "" {
		existing.Description = &req.Description
	}
	if req.IsActive != nil {
		existing.IsActive = *req.IsActive
	}

	if err := h.deps.SemanticRepo.UpdateModel(ctx, existing); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to update model")
		return
	}

	writeJSON(w, http.StatusOK, existing)
}

func (h *SemanticHandler) DeleteModel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()

	if err := h.deps.SemanticRepo.DeleteModel(ctx, id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete model")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type createDimensionRequest struct {
	Name      string   `json:"name"`
	Label     string   `json:"label,omitempty"`
	ColumnRef string   `json:"column_ref"`
	Type      string   `json:"type"`
	Synonyms  []string `json:"synonyms,omitempty"`
}

func (h *SemanticHandler) CreateDimension(w http.ResponseWriter, r *http.Request) {
	modelID := chi.URLParam(r, "id")

	var req createDimensionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	d := &semantic.Dimension{
		ID:       uuid.New().String(),
		ModelID:  modelID,
		Name:     req.Name,
		ColumnRef: req.ColumnRef,
		Type:     req.Type,
		Synonyms: req.Synonyms,
		IsActive: true,
	}

	if req.Label != "" {
		d.Label = &req.Label
	}

	ctx := r.Context()
	if err := h.deps.SemanticRepo.CreateDimension(ctx, d); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create dimension")
		return
	}

	writeJSON(w, http.StatusCreated, d)
}

type createMetricRequest struct {
	Name        string `json:"name"`
	Label       string `json:"label,omitempty"`
	Expression  string `json:"expression"`
	Aggregation string `json:"aggregation"`
	Format      string `json:"format,omitempty"`
	Synonyms    []string `json:"synonyms,omitempty"`
}

func (h *SemanticHandler) CreateMetric(w http.ResponseWriter, r *http.Request) {
	modelID := chi.URLParam(r, "id")

	var req createMetricRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	m := &semantic.Metric{
		ID:          uuid.New().String(),
		ModelID:     modelID,
		Name:        req.Name,
		Expression:  req.Expression,
		Aggregation: req.Aggregation,
		Synonyms:    req.Synonyms,
		IsActive:    true,
	}

	if req.Label != "" {
		m.Label = &req.Label
	}
	if req.Format != "" {
		m.Format = &req.Format
	}

	ctx := r.Context()
	if err := h.deps.SemanticRepo.CreateMetric(ctx, m); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create metric")
		return
	}

	writeJSON(w, http.StatusCreated, m)
}

type createJoinRequest struct {
	Name         string `json:"name"`
	FromTable    string `json:"from_table"`
	FromColumn   string `json:"from_column"`
	ToTable      string `json:"to_table"`
	ToColumn     string `json:"to_column"`
	JoinType     string `json:"join_type,omitempty"`
	Relationship string `json:"relationship,omitempty"`
}

func (h *SemanticHandler) CreateJoin(w http.ResponseWriter, r *http.Request) {
	modelID := chi.URLParam(r, "id")

	var req createJoinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	joinType := req.JoinType
	if joinType == "" {
		joinType = "LEFT"
	}

	relationship := req.Relationship
	if relationship == "" {
		relationship = "many_to_one"
	}

	j := &semantic.Join{
		ID:           uuid.New().String(),
		ModelID:      modelID,
		Name:         req.Name,
		FromTable:    req.FromTable,
		FromColumn:   req.FromColumn,
		ToTable:      req.ToTable,
		ToColumn:     req.ToColumn,
		JoinType:     joinType,
		Relationship: relationship,
		IsActive:     true,
	}

	ctx := r.Context()
	if err := h.deps.SemanticRepo.CreateJoin(ctx, j); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to create join")
		return
	}

	writeJSON(w, http.StatusCreated, j)
}
