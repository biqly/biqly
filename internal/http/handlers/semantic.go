package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/semantic"
	"github.com/biqly/biqly/internal/semanticgen"
	"github.com/google/uuid"
)

// SemanticHandler handles semantic layer CRUD operations.
type SemanticHandler struct {
	deps    *app.Dependencies
	metrics CatalogMetricsRecorder
}

// NewSemanticHandler creates a new semantic handler.
func NewSemanticHandler(deps *app.Dependencies) *SemanticHandler {
	return &SemanticHandler{deps: deps}
}

// SetCatalogMetricsRecorder wires process-level Catalog metrics.
func (h *SemanticHandler) SetCatalogMetricsRecorder(m CatalogMetricsRecorder) {
	h.metrics = m
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

type generateModelRequest struct {
	DatasourceID string `json:"datasource_id"`
	BaseSchema   string `json:"base_schema,omitempty"`
	BaseTable    string `json:"base_table,omitempty"`
	Publish      bool   `json:"publish,omitempty"`
	PublishedBy  string `json:"published_by,omitempty"`
}

type generateModelResponse struct {
	Model      *semantic.SemanticModel          `json:"model"`
	Warnings   []string                         `json:"warnings,omitempty"`
	Validation semantic.PublishValidationResult `json:"validation"`
	Published  bool                             `json:"published"`
}

func (h *SemanticHandler) GenerateModel(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[generateModelRequest](w, r)
	if !ok {
		return
	}
	req.DatasourceID = strings.TrimSpace(req.DatasourceID)
	req.BaseSchema = strings.TrimSpace(req.BaseSchema)
	req.BaseTable = strings.TrimSpace(req.BaseTable)
	if req.DatasourceID == "" {
		writeError(w, http.StatusBadRequest, "datasource_id is required")
		return
	}

	ctx := r.Context()
	tables, err := h.deps.MetaRepo.ListTables(ctx, req.DatasourceID, "")
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to load tables", err)
		return
	}
	columns, err := h.deps.MetaRepo.ListColumns(ctx, req.DatasourceID, "", "")
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to load columns", err)
		return
	}
	relations, err := h.deps.MetaRepo.ListRelations(ctx, req.DatasourceID)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to load relations", err)
		return
	}
	existing, err := h.deps.SemanticRepo.ListModels(ctx, req.DatasourceID)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to list semantic models", err)
		return
	}
	existingNames := make([]string, 0, len(existing))
	for _, model := range existing {
		existingNames = append(existingNames, model.Name)
	}

	datasourceName := ""
	if ds, err := h.deps.MetaRepo.GetDatasource(ctx, req.DatasourceID); err == nil && ds != nil {
		datasourceName = ds.Name
	}

	generated, err := semanticgen.GenerateModelFromMetadata(tables, columns, relations, semanticgen.GenerateModelOptions{
		DatasourceID:   req.DatasourceID,
		DatasourceName: datasourceName,
		BaseSchema:     req.BaseSchema,
		BaseTable:      req.BaseTable,
		ExistingNames:  existingNames,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	model := generated.Model
	if err := h.persistGeneratedModel(ctx, model); err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to create generated model", err)
		return
	}

	validation, err := h.deps.SemanticRepo.ValidateModel(ctx, model.ID, semanticCatalogAdapter{repo: h.deps.MetaRepo})
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to validate generated model", err)
		return
	}

	full, err := h.deps.SemanticRepo.GetFullModel(ctx, model.ID)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to load generated model", err)
		return
	}

	published := false
	if req.Publish {
		if !validation.Valid {
			writeJSON(w, http.StatusUnprocessableEntity, generateModelResponse{
				Model:      full,
				Warnings:   generated.Warnings,
				Validation: validation,
				Published:  false,
			})
			return
		}
		result, err := h.deps.SemanticRepo.PublishModel(ctx, model.ID, req.PublishedBy, semanticCatalogAdapter{repo: h.deps.MetaRepo})
		if err != nil {
			writeInternalError(ctx, w, http.StatusInternalServerError, "failed to publish generated model", err)
			return
		}
		full = result.Model
		validation = result.Validation
		published = validation.Valid
	}

	writeJSON(w, http.StatusCreated, generateModelResponse{
		Model:      full,
		Warnings:   generated.Warnings,
		Validation: validation,
		Published:  published,
	})
}

func (h *SemanticHandler) persistGeneratedModel(ctx context.Context, model *semantic.SemanticModel) error {
	if err := h.deps.SemanticRepo.CreateModel(ctx, model); err != nil {
		return err
	}
	cleanup := true
	defer func() {
		if cleanup {
			_ = h.deps.SemanticRepo.DeleteModel(ctx, model.ID)
		}
	}()
	if err := h.deps.SemanticRepo.BulkInsertModelChildren(ctx, model.ID, model.Dimensions, model.Metrics, model.Joins); err != nil {
		return err
	}
	cleanup = false
	return nil
}

// CreateModel creates a new semantic model.
func (h *SemanticHandler) CreateModel(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[createModelRequest](w, r)
	if !ok {
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
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to create model", err)
		return
	}

	writeJSON(w, http.StatusCreated, m)
}

// ListModels returns semantic models, optionally filtered by ?datasource_id=.
func (h *SemanticHandler) ListModels(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	dsID := strings.TrimSpace(r.URL.Query().Get("datasource_id"))
	models, err := h.deps.SemanticRepo.ListModels(ctx, dsID)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to list models", err)
		return
	}

	writeJSON(w, http.StatusOK, models)
}

// GetModel returns a semantic model with its dimensions, metrics, and joins.
// When ?include_inactive=true is passed the response also contains soft-deleted
// children so the modeling UI can offer "re-add" controls.
func (h *SemanticHandler) GetModel(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()

	model, err := h.deps.SemanticRepo.GetFullModel(ctx, id)
	if err != nil {
		writeInternalError(ctx, w, http.StatusNotFound, "model not found", err)
		return
	}

	if r.URL.Query().Get("include_inactive") == "true" {
		if dims, err := h.deps.SemanticRepo.ListAllDimensions(ctx, id); err == nil {
			model.Dimensions = dims
		}
		if mets, err := h.deps.SemanticRepo.ListAllMetrics(ctx, id); err == nil {
			model.Metrics = mets
		}
		if joins, err := h.deps.SemanticRepo.ListAllJoins(ctx, id); err == nil {
			model.Joins = joins
		}
	}

	writeJSON(w, http.StatusOK, model)
}

type updateModelRequest struct {
	Name            string    `json:"name"`
	Label           string    `json:"label,omitempty"`
	Description     string    `json:"description,omitempty"`
	BaseSchema      string    `json:"base_schema"`
	BaseTable       string    `json:"base_table"`
	Synonyms        []string  `json:"synonyms,omitempty"`
	ExcludedSchemas *[]string `json:"excluded_schemas,omitempty"`
	IsActive        *bool     `json:"is_active,omitempty"`
}

// UpdateModel updates an existing semantic model.
func (h *SemanticHandler) UpdateModel(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()

	existing, err := h.deps.SemanticRepo.GetModel(ctx, id)
	if err != nil {
		writeEntityNotFound(w, "model")
		return
	}

	req, ok := decodeJSON[updateModelRequest](w, r)
	if !ok {
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
	if req.ExcludedSchemas != nil {
		existing.ExcludedSchemas = *req.ExcludedSchemas
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
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to update model", err)
		return
	}

	writeJSON(w, http.StatusOK, existing)
}

// DeleteModel removes a semantic model.
func (h *SemanticHandler) DeleteModel(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()

	if err := h.deps.SemanticRepo.DeleteModel(ctx, id); err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to delete model", err)
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

// ValidateModel validates whether a semantic model can be published.
func (h *SemanticHandler) ValidateModel(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	result, err := h.deps.SemanticRepo.ValidateModel(r.Context(), id, semanticCatalogAdapter{repo: h.deps.MetaRepo})
	if err != nil {
		writeEntityNotFound(w, "model")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// PublishModel publishes a draft semantic model after validation.
func (h *SemanticHandler) PublishModel(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	req, ok := decodeJSONAllowEmpty[publishRequest](w, r)
	if !ok {
		return
	}
	start := time.Now()
	result, err := h.deps.SemanticRepo.PublishModel(r.Context(), id, req.PublishedBy, semanticCatalogAdapter{repo: h.deps.MetaRepo})
	if h.metrics != nil {
		h.metrics.RecordModelPublish(time.Since(start).Milliseconds(), err == nil && result != nil && result.Validation.Valid)
	}
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to publish model", err)
		return
	}
	if !result.Validation.Valid {
		writeJSON(w, http.StatusUnprocessableEntity, result)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// RollbackModel restores a previously published semantic model version.
func (h *SemanticHandler) RollbackModel(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	req, ok := decodeJSONAllowEmpty[rollbackRequest](w, r)
	if !ok {
		return
	}
	result, err := h.deps.SemanticRepo.RollbackModel(r.Context(), id, req.Version, req.PublishedBy)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to rollback model", err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

type createDimensionRequest struct {
	Name      string   `json:"name"`
	Label     string   `json:"label,omitempty"`
	ColumnRef string   `json:"column_ref"`
	Type      string   `json:"type"`
	TimeGrain string   `json:"time_grain,omitempty"`
	Synonyms  []string `json:"synonyms,omitempty"`
}

// CreateDimension adds a dimension to a semantic model.
func (h *SemanticHandler) CreateDimension(w http.ResponseWriter, r *http.Request) {
	modelID, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}

	req, ok := decodeJSON[createDimensionRequest](w, r)
	if !ok {
		return
	}

	d := &semantic.Dimension{
		ID:        uuid.New().String(),
		ModelID:   modelID,
		Name:      req.Name,
		ColumnRef: req.ColumnRef,
		Type:      req.Type,
		TimeGrain: req.TimeGrain,
		Synonyms:  req.Synonyms,
		IsActive:  true,
	}

	if req.Label != "" {
		d.Label = &req.Label
	}

	ctx := r.Context()
	if err := h.deps.SemanticRepo.CreateDimension(ctx, d); err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to create dimension", err)
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
	modelID, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}

	req, ok := decodeJSON[createMetricRequest](w, r)
	if !ok {
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
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to create metric", err)
		return
	}

	writeJSON(w, http.StatusCreated, m)
}

type createJoinRequest struct {
	Name         string `json:"name"`
	FromSchema   string `json:"from_schema,omitempty"`
	FromTable    string `json:"from_table"`
	FromColumn   string `json:"from_column"`
	ToSchema     string `json:"to_schema,omitempty"`
	ToTable      string `json:"to_table"`
	ToColumn     string `json:"to_column"`
	JoinType     string `json:"join_type,omitempty"`
	Relationship string `json:"relationship,omitempty"`
}

// joinFromRequest builds a semantic.Join from a createJoinRequest, applying
// project-wide defaults for join type and relationship cardinality.
func joinFromRequest(id, modelID string, req createJoinRequest) *semantic.Join {
	joinType := req.JoinType
	if joinType == "" {
		joinType = semantic.DefaultJoinType
	}
	relationship := req.Relationship
	if relationship == "" {
		relationship = semantic.DefaultRelationshipType
	}
	return &semantic.Join{
		ID:           id,
		ModelID:      modelID,
		Name:         req.Name,
		FromSchema:   req.FromSchema,
		FromTable:    req.FromTable,
		FromColumn:   req.FromColumn,
		ToSchema:     req.ToSchema,
		ToTable:      req.ToTable,
		ToColumn:     req.ToColumn,
		JoinType:     joinType,
		Relationship: relationship,
		IsActive:     true,
	}
}

// CreateJoin adds a join definition to a semantic model.
func (h *SemanticHandler) CreateJoin(w http.ResponseWriter, r *http.Request) {
	modelID, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}

	req, ok := decodeJSON[createJoinRequest](w, r)
	if !ok {
		return
	}

	j := joinFromRequest(uuid.New().String(), modelID, *req)

	ctx := r.Context()
	if err := h.deps.SemanticRepo.CreateJoin(ctx, j); err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to create join", err)
		return
	}

	writeJSON(w, http.StatusCreated, j)
}

// DeleteDimension removes a dimension from a semantic model.
func (h *SemanticHandler) DeleteDimension(w http.ResponseWriter, r *http.Request) {
	modelID, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	dimID, ok := requireURLParam(w, r, "dimension_id")
	if !ok {
		return
	}
	if err := h.deps.SemanticRepo.DeleteDimension(r.Context(), modelID, dimID); err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to delete dimension", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// UpdateDimension modifies an existing dimension.
func (h *SemanticHandler) UpdateDimension(w http.ResponseWriter, r *http.Request) {
	modelID, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	dimID, ok := requireURLParam(w, r, "dimension_id")
	if !ok {
		return
	}
	req, ok := decodeJSON[createDimensionRequest](w, r)
	if !ok {
		return
	}
	d := &semantic.Dimension{
		ID:        dimID,
		ModelID:   modelID,
		Name:      req.Name,
		ColumnRef: req.ColumnRef,
		Type:      req.Type,
		TimeGrain: req.TimeGrain,
		Synonyms:  req.Synonyms,
		IsActive:  true,
	}
	if req.Label != "" {
		d.Label = &req.Label
	}
	if err := h.deps.SemanticRepo.UpdateDimension(r.Context(), d); err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to update dimension", err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// DeleteMetric removes a metric from a semantic model.
func (h *SemanticHandler) DeleteMetric(w http.ResponseWriter, r *http.Request) {
	modelID, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	metricID, ok := requireURLParam(w, r, "metric_id")
	if !ok {
		return
	}
	if err := h.deps.SemanticRepo.DeleteMetric(r.Context(), modelID, metricID); err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to delete metric", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// UpdateMetric modifies an existing metric.
func (h *SemanticHandler) UpdateMetric(w http.ResponseWriter, r *http.Request) {
	modelID, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	metricID, ok := requireURLParam(w, r, "metric_id")
	if !ok {
		return
	}
	req, ok := decodeJSON[createMetricRequest](w, r)
	if !ok {
		return
	}
	m := &semantic.Metric{
		ID:          metricID,
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
	if err := h.deps.SemanticRepo.UpdateMetric(r.Context(), m); err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to update metric", err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

// DeleteJoin removes a join from a semantic model.
func (h *SemanticHandler) DeleteJoin(w http.ResponseWriter, r *http.Request) {
	modelID, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	joinID, ok := requireURLParam(w, r, "join_id")
	if !ok {
		return
	}
	if err := h.deps.SemanticRepo.DeleteJoin(r.Context(), modelID, joinID); err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to delete join", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type removeTableRequest struct {
	Schema string `json:"schema"`
	Table  string `json:"table"`
}

type removeTableResponse struct {
	JoinsRemoved      int `json:"joins_removed"`
	DimensionsRemoved int `json:"dimensions_removed"`
	MetricsRemoved    int `json:"metrics_removed"`
}

// RemoveTable cascade-deletes joins, dimensions, and metrics referencing the
// given table. Rejects the request if the table is the model's base table.
func (h *SemanticHandler) RemoveTable(w http.ResponseWriter, r *http.Request) {
	modelID, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	req, ok := decodeJSON[removeTableRequest](w, r)
	if !ok {
		return
	}
	if strings.TrimSpace(req.Table) == "" {
		writeError(w, http.StatusBadRequest, "table is required")
		return
	}
	ctx := r.Context()
	model, err := h.deps.SemanticRepo.GetFullModel(ctx, modelID)
	if err != nil {
		writeEntityNotFound(w, "model")
		return
	}
	schema := strings.TrimSpace(req.Schema)
	if schema == "" {
		schema = model.BaseSchema
	}
	if schema == model.BaseSchema && req.Table == model.BaseTable {
		writeError(w, http.StatusBadRequest, "cannot remove base table; change base first")
		return
	}

	resp := removeTableResponse{}
	for _, j := range model.Joins {
		fromSchema := j.FromSchema
		if fromSchema == "" {
			fromSchema = model.BaseSchema
		}
		toSchema := j.ToSchema
		if toSchema == "" {
			toSchema = model.BaseSchema
		}
		if (fromSchema == schema && j.FromTable == req.Table) || (toSchema == schema && j.ToTable == req.Table) {
			if err := h.deps.SemanticRepo.DeleteJoin(ctx, modelID, j.ID); err != nil {
				slog.WarnContext(ctx, "remove table: delete join failed", "model_id", modelID, "join_id", j.ID, "error", err)
				continue
			}
			resp.JoinsRemoved++
		}
	}
	for _, d := range model.Dimensions {
		if columnRefMatchesTable(d.ColumnRef, schema, req.Table, model.BaseSchema) {
			if err := h.deps.SemanticRepo.DeleteDimension(ctx, modelID, d.ID); err != nil {
				slog.WarnContext(ctx, "remove table: delete dimension failed", "model_id", modelID, "dimension_id", d.ID, "error", err)
				continue
			}
			resp.DimensionsRemoved++
		}
	}
	for _, m := range model.Metrics {
		if expressionReferencesTable(m.Expression, schema, req.Table, model.BaseSchema) {
			if err := h.deps.SemanticRepo.DeleteMetric(ctx, modelID, m.ID); err != nil {
				slog.WarnContext(ctx, "remove table: delete metric failed", "model_id", modelID, "metric_id", m.ID, "error", err)
				continue
			}
			resp.MetricsRemoved++
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

type removeSchemaRequest struct {
	Schema string `json:"schema"`
}

type removeSchemaResponse struct {
	JoinsRemoved      int `json:"joins_removed"`
	DimensionsRemoved int `json:"dimensions_removed"`
	MetricsRemoved    int `json:"metrics_removed"`
}

// RemoveSchema cascade-deletes joins, dimensions, and metrics referencing the
// given schema and adds it to excluded_schemas. Rejects excluding the base schema.
func (h *SemanticHandler) RemoveSchema(w http.ResponseWriter, r *http.Request) {
	modelID, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	req, ok := decodeJSON[removeSchemaRequest](w, r)
	if !ok {
		return
	}
	schema := strings.TrimSpace(req.Schema)
	if schema == "" {
		writeError(w, http.StatusBadRequest, "schema is required")
		return
	}
	ctx := r.Context()
	model, err := h.deps.SemanticRepo.GetFullModel(ctx, modelID)
	if err != nil {
		writeEntityNotFound(w, "model")
		return
	}
	if schema == model.BaseSchema {
		writeError(w, http.StatusBadRequest, "cannot exclude base schema")
		return
	}

	resp := removeSchemaResponse{}
	for _, j := range model.Joins {
		if joinMatchesSchema(j, schema, model.BaseSchema) {
			if err := h.deps.SemanticRepo.DeleteJoin(ctx, modelID, j.ID); err != nil {
				slog.WarnContext(ctx, "remove schema: delete join failed", "model_id", modelID, "join_id", j.ID, "error", err)
				continue
			}
			resp.JoinsRemoved++
		}
	}
	for _, d := range model.Dimensions {
		if columnRefMatchesSchema(d.ColumnRef, schema, model.BaseSchema) {
			if err := h.deps.SemanticRepo.DeleteDimension(ctx, modelID, d.ID); err != nil {
				slog.WarnContext(ctx, "remove schema: delete dimension failed", "model_id", modelID, "dimension_id", d.ID, "error", err)
				continue
			}
			resp.DimensionsRemoved++
		}
	}
	for _, m := range model.Metrics {
		if expressionReferencesSchema(m.Expression, schema) {
			if err := h.deps.SemanticRepo.DeleteMetric(ctx, modelID, m.ID); err != nil {
				slog.WarnContext(ctx, "remove schema: delete metric failed", "model_id", modelID, "metric_id", m.ID, "error", err)
				continue
			}
			resp.MetricsRemoved++
		}
	}

	excluded := model.ExcludedSchemas
	if excluded == nil {
		excluded = []string{}
	}
	if !slices.Contains(excluded, schema) {
		excluded = append(excluded, schema)
		model.ExcludedSchemas = excluded
		if err := h.deps.SemanticRepo.UpdateModel(ctx, model); err != nil {
			writeInternalError(ctx, w, http.StatusInternalServerError, "failed to update excluded schemas", err)
			return
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

func joinMatchesSchema(j semantic.Join, schema, baseSchema string) bool {
	fromSchema := j.FromSchema
	if fromSchema == "" {
		fromSchema = baseSchema
	}
	toSchema := j.ToSchema
	if toSchema == "" {
		toSchema = baseSchema
	}
	return fromSchema == schema || toSchema == schema
}

func columnRefMatchesSchema(ref, schema, baseSchema string) bool {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return false
	}
	if strings.HasPrefix(ref, schema+".") {
		return true
	}
	if schema == baseSchema {
		parts := strings.Split(ref, ".")
		if len(parts) == 2 {
			return true
		}
	}
	return false
}

func expressionReferencesSchema(expr, schema string) bool {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return false
	}
	tokens := []string{
		schema + ".",
		`"` + schema + `".`,
	}
	lower := strings.ToLower(expr)
	for _, tok := range tokens {
		if strings.Contains(lower, strings.ToLower(tok)) {
			return true
		}
	}
	return false
}

func columnRefMatchesTable(ref, schema, table, baseSchema string) bool {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return false
	}
	qualified := schema + "." + table + "."
	if strings.HasPrefix(ref, qualified) {
		return true
	}
	if schema == baseSchema && strings.HasPrefix(ref, table+".") {
		return true
	}
	return false
}

func expressionReferencesTable(expr, schema, table, baseSchema string) bool {
	expr = strings.TrimSpace(expr)
	if expr == "" {
		return false
	}
	tokens := []string{
		schema + "." + table + ".",
		`"` + schema + `"."` + table + `".`,
	}
	if schema == baseSchema {
		tokens = append(tokens, table+".", `"`+table+`".`)
	}
	lower := strings.ToLower(expr)
	for _, tok := range tokens {
		if strings.Contains(lower, strings.ToLower(tok)) {
			return true
		}
	}
	return false
}

// UpdateJoin modifies an existing join.
func (h *SemanticHandler) UpdateJoin(w http.ResponseWriter, r *http.Request) {
	modelID, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	joinID, ok := requireURLParam(w, r, "join_id")
	if !ok {
		return
	}
	req, ok := decodeJSON[createJoinRequest](w, r)
	if !ok {
		return
	}
	j := joinFromRequest(joinID, modelID, *req)
	if err := h.deps.SemanticRepo.UpdateJoin(r.Context(), j); err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to update join", err)
		return
	}
	writeJSON(w, http.StatusOK, j)
}

type suggestedJoin struct {
	FromSchema string `json:"from_schema"`
	FromTable  string `json:"from_table"`
	FromColumn string `json:"from_column"`
	ToSchema   string `json:"to_schema"`
	ToTable    string `json:"to_table"`
	ToColumn   string `json:"to_column"`
	Name       string `json:"name"`
}

// SuggestedJoins returns FK relations that are not yet in the model.
func (h *SemanticHandler) SuggestedJoins(w http.ResponseWriter, r *http.Request) {
	modelID, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	model, err := h.deps.SemanticRepo.GetFullModel(ctx, modelID)
	if err != nil {
		writeEntityNotFound(w, "model")
		return
	}
	relations, err := h.deps.MetaRepo.ListRelations(ctx, model.DatasourceID)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to load relations", err)
		return
	}
	existing := make(map[string]bool, len(model.Joins))
	for _, j := range model.Joins {
		existing[joinFingerprint(j.FromSchema, j.FromTable, j.FromColumn, j.ToSchema, j.ToTable, j.ToColumn)] = true
		existing[joinFingerprint(j.ToSchema, j.ToTable, j.ToColumn, j.FromSchema, j.FromTable, j.FromColumn)] = true
	}
	tables := map[string]bool{strings.ToLower(model.BaseTable): true}
	for _, j := range model.Joins {
		tables[strings.ToLower(j.FromTable)] = true
		tables[strings.ToLower(j.ToTable)] = true
	}
	var suggestions []suggestedJoin
	for _, rel := range relations {
		fp := joinFingerprint(rel.FromSchema, rel.FromTable, rel.FromColumn, rel.ToSchema, rel.ToTable, rel.ToColumn)
		if existing[fp] {
			continue
		}
		if !tables[strings.ToLower(rel.FromTable)] && !tables[strings.ToLower(rel.ToTable)] {
			continue
		}
		name := fmt.Sprintf("%s_%s_to_%s_%s", rel.FromTable, rel.FromColumn, rel.ToTable, rel.ToColumn)
		suggestions = append(suggestions, suggestedJoin{
			FromSchema: rel.FromSchema,
			FromTable:  rel.FromTable,
			FromColumn: rel.FromColumn,
			ToSchema:   rel.ToSchema,
			ToTable:    rel.ToTable,
			ToColumn:   rel.ToColumn,
			Name:       name,
		})
	}
	if suggestions == nil {
		suggestions = []suggestedJoin{}
	}
	writeJSON(w, http.StatusOK, suggestions)
}

func joinFingerprint(fromSchema, fromTable, fromColumn, toSchema, toTable, toColumn string) string {
	return fmt.Sprintf("%s.%s.%s->%s.%s.%s",
		strings.ToLower(fromSchema), strings.ToLower(fromTable), strings.ToLower(fromColumn),
		strings.ToLower(toSchema), strings.ToLower(toTable), strings.ToLower(toColumn))
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
