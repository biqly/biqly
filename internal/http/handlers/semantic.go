package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/metadata"
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

// CreateModel creates a new semantic model.
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

// ListModels returns all semantic models for a datasource.
func (h *SemanticHandler) ListModels(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	models, err := h.deps.SemanticRepo.ListModels(ctx, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list models")
		return
	}

	writeJSON(w, http.StatusOK, models)
}

// GetModel returns a semantic model with its dimensions, metrics, and joins.
func (h *SemanticHandler) GetModel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()

	model, err := h.deps.SemanticRepo.GetFullModel(ctx, id)
	if err != nil {
		slog.ErrorContext(ctx, "get semantic model failed", "id", id, "error", err)
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

// UpdateModel updates an existing semantic model.
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

// DeleteModel removes a semantic model.
func (h *SemanticHandler) DeleteModel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()

	if err := h.deps.SemanticRepo.DeleteModel(ctx, id); err != nil {
		writeError(w, http.StatusInternalServerError, "failed to delete model")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type publishRequest struct {
	PublishedBy string `json:"published_by,omitempty"`
}

type rollbackRequest struct {
	Version     int    `json:"version,omitempty"`
	PublishedBy string `json:"published_by,omitempty"`
}

func (h *SemanticHandler) ValidateModel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	result, err := h.deps.SemanticRepo.ValidateModel(r.Context(), id, semanticCatalogAdapter{repo: h.deps.MetaRepo})
	if err != nil {
		writeError(w, http.StatusNotFound, "model not found")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *SemanticHandler) PublishModel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req publishRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.deps.SemanticRepo.PublishModel(r.Context(), id, req.PublishedBy, semanticCatalogAdapter{repo: h.deps.MetaRepo})
	if err != nil {
		slog.ErrorContext(r.Context(), "publish semantic model failed", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to publish model")
		return
	}
	if !result.Validation.Valid {
		writeJSON(w, http.StatusUnprocessableEntity, result)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *SemanticHandler) RollbackModel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var req rollbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	result, err := h.deps.SemanticRepo.RollbackModel(r.Context(), id, req.Version, req.PublishedBy)
	if err != nil {
		slog.ErrorContext(r.Context(), "rollback semantic model failed", "id", id, "error", err)
		writeError(w, http.StatusInternalServerError, "failed to rollback model")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

type createDimensionRequest struct {
	Name      string   `json:"name"`
	Label     string   `json:"label,omitempty"`
	ColumnRef string   `json:"column_ref"`
	Type      string   `json:"type"`
	Synonyms  []string `json:"synonyms,omitempty"`
}

// CreateDimension adds a dimension to a semantic model.
func (h *SemanticHandler) CreateDimension(w http.ResponseWriter, r *http.Request) {
	modelID := chi.URLParam(r, "id")

	var req createDimensionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	d := &semantic.Dimension{
		ID:        uuid.New().String(),
		ModelID:   modelID,
		Name:      req.Name,
		ColumnRef: req.ColumnRef,
		Type:      req.Type,
		Synonyms:  req.Synonyms,
		IsActive:  true,
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
	Name        string   `json:"name"`
	Label       string   `json:"label,omitempty"`
	Expression  string   `json:"expression"`
	Aggregation string   `json:"aggregation"`
	Format      string   `json:"format,omitempty"`
	Synonyms    []string `json:"synonyms,omitempty"`
}

// CreateMetric adds a metric to a semantic model.
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

// CreateJoin adds a join definition to a semantic model.
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

type semanticCatalogAdapter struct {
	repo interface {
		ListColumns(ctx context.Context, datasourceID, schemaName, tableName string) ([]metadata.Column, error)
		ListRelations(ctx context.Context, datasourceID string) ([]metadata.Relation, error)
		ListPermissionPolicies(ctx context.Context, datasourceID string) ([]metadata.PermissionPolicyRecord, error)
	}
}

func (a semanticCatalogAdapter) ListSemanticColumns(ctx context.Context, datasourceID string) ([]semantic.CatalogColumn, error) {
	columns, err := a.repo.ListColumns(ctx, datasourceID, "", "")
	if err != nil {
		return nil, err
	}
	out := make([]semantic.CatalogColumn, 0, len(columns))
	for _, col := range columns {
		out = append(out, semantic.CatalogColumn{
			SchemaName: col.SchemaName,
			TableName:  col.TableName,
			ColumnName: col.ColumnName,
		})
	}
	return out, nil
}

func (a semanticCatalogAdapter) ListSemanticRelations(ctx context.Context, datasourceID string) ([]semantic.CatalogRelation, error) {
	relations, err := a.repo.ListRelations(ctx, datasourceID)
	if err != nil {
		return nil, err
	}
	out := make([]semantic.CatalogRelation, 0, len(relations))
	for _, rel := range relations {
		out = append(out, semantic.CatalogRelation{
			FromSchema: rel.FromSchema,
			FromTable:  rel.FromTable,
			FromColumn: rel.FromColumn,
			ToSchema:   rel.ToSchema,
			ToTable:    rel.ToTable,
			ToColumn:   rel.ToColumn,
		})
	}
	return out, nil
}

func (a semanticCatalogAdapter) ListSemanticPolicies(ctx context.Context, datasourceID string) ([]semantic.CatalogPolicy, error) {
	policies, err := a.repo.ListPermissionPolicies(ctx, datasourceID)
	if err != nil {
		return nil, err
	}
	out := make([]semantic.CatalogPolicy, 0, len(policies))
	for _, policy := range policies {
		p := semantic.CatalogPolicy{DeniedFields: policy.DeniedFields}
		for _, filter := range policy.RowFilters {
			p.RowFilters = append(p.RowFilters, semantic.CatalogRowFilter{Field: filter.Field})
		}
		out = append(out, p)
	}
	return out, nil
}
