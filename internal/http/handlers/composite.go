package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/semantic"
)

// CompositeHandler serves composite semantic model CRUD, component/cross-join
// management, configuration, and lifecycle (validate/publish/rollback) endpoints.
type CompositeHandler struct {
	deps *app.CatalogDeps
}

// NewCompositeHandler creates a CompositeHandler.
func NewCompositeHandler(deps *app.CatalogDeps) *CompositeHandler {
	return &CompositeHandler{deps: deps}
}

func (h *CompositeHandler) repo() *semantic.CompositeRepository { return h.deps.CompositeRepo }

// componentProvider returns the published-model loader used by validate/publish.
func (h *CompositeHandler) componentProvider() semantic.ComponentProvider {
	return h.deps.SemanticRepo
}

type createCompositeRequest struct {
	DatasourceID string `json:"datasource_id"`
	Name         string `json:"name"`
	Label        string `json:"label,omitempty"`
	Description  string `json:"description,omitempty"`
	CreatedBy    string `json:"created_by,omitempty"`
}

// CreateComposite creates a draft composite model.
func (h *CompositeHandler) CreateComposite(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[createCompositeRequest](w, r)
	if !ok {
		return
	}
	req.DatasourceID = strings.TrimSpace(req.DatasourceID)
	req.Name = strings.TrimSpace(req.Name)
	if req.DatasourceID == "" {
		writeError(w, http.StatusBadRequest, "datasource_id is required")
		return
	}
	if req.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}

	c := &semantic.CompositeModel{
		DatasourceID: req.DatasourceID,
		Name:         req.Name,
		Label:        optionalStringPtr(req.Label),
		Description:  optionalStringPtr(req.Description),
		CreatedBy:    optionalStringPtr(req.CreatedBy),
	}
	ctx := r.Context()
	if err := h.repo().CreateComposite(ctx, c); err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to create composite model", err)
		return
	}
	full, err := h.repo().GetFullComposite(ctx, c.ID)
	if err != nil {
		writeJSON(w, http.StatusCreated, c)
		return
	}
	writeJSON(w, http.StatusCreated, full)
}

// ListComposites lists composite models, optionally filtered by datasource_id.
func (h *CompositeHandler) ListComposites(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	datasourceID := strings.TrimSpace(r.URL.Query().Get("datasource_id"))
	composites, err := h.repo().ListComposites(ctx, datasourceID)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to list composite models", err)
		return
	}
	writeJSON(w, http.StatusOK, composites)
}

// GetComposite returns a composite model with components, cross-joins, canonical
// date, and dimension resolutions.
func (h *CompositeHandler) GetComposite(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	full, err := h.repo().GetFullComposite(ctx, id)
	if err != nil {
		writeEntityNotFound(w, "composite model")
		return
	}
	writeJSON(w, http.StatusOK, full)
}

type updateCompositeRequest struct {
	Name        *string `json:"name,omitempty"`
	Label       *string `json:"label,omitempty"`
	Description *string `json:"description,omitempty"`
	IsActive    *bool   `json:"is_active,omitempty"`
}

// UpdateComposite updates a composite header and returns it to draft status.
func (h *CompositeHandler) UpdateComposite(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	req, ok := decodeJSON[updateCompositeRequest](w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	existing, err := h.repo().GetComposite(ctx, id)
	if err != nil {
		writeEntityNotFound(w, "composite model")
		return
	}
	if req.Name != nil {
		name := strings.TrimSpace(*req.Name)
		if name == "" {
			writeError(w, http.StatusBadRequest, "name cannot be empty")
			return
		}
		existing.Name = name
	}
	if req.Label != nil {
		existing.Label = optionalStringPtr(*req.Label)
	}
	if req.Description != nil {
		existing.Description = optionalStringPtr(*req.Description)
	}
	if req.IsActive != nil {
		existing.IsActive = *req.IsActive
	}
	if err := h.repo().UpdateComposite(ctx, existing); err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to update composite model", err)
		return
	}
	full, err := h.repo().GetFullComposite(ctx, id)
	if err != nil {
		writeJSON(w, http.StatusOK, existing)
		return
	}
	writeJSON(w, http.StatusOK, full)
}

// DeleteComposite removes a composite model and its children.
func (h *CompositeHandler) DeleteComposite(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	if err := h.repo().DeleteComposite(ctx, id); err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to delete composite model", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type addComponentRequest struct {
	ModelID string `json:"model_id"`
	Alias   string `json:"alias"`
	Role    string `json:"role"`
}

// AddComponent attaches a component semantic model to a composite.
func (h *CompositeHandler) AddComponent(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	req, ok := decodeJSON[addComponentRequest](w, r)
	if !ok {
		return
	}
	req.ModelID = strings.TrimSpace(req.ModelID)
	req.Alias = strings.TrimSpace(req.Alias)
	req.Role = strings.TrimSpace(req.Role)
	if req.ModelID == "" {
		writeError(w, http.StatusBadRequest, "model_id is required")
		return
	}
	if req.Alias == "" {
		writeError(w, http.StatusBadRequest, "alias is required")
		return
	}
	if req.Role == "" {
		req.Role = semantic.ComponentRoleSecondary
	}
	if req.Role != semantic.ComponentRolePrimary && req.Role != semantic.ComponentRoleSecondary {
		writeError(w, http.StatusBadRequest, "role must be primary or secondary")
		return
	}
	ctx := r.Context()
	ref := semantic.ComponentModelRef{
		CompositeID: id,
		ModelID:     req.ModelID,
		Alias:       req.Alias,
		Role:        req.Role,
	}
	if err := h.repo().AddComponent(ctx, id, ref); err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to add component", err)
		return
	}
	h.respondFull(ctx, w, id, http.StatusCreated)
}

// RemoveComponent detaches a component model from a composite.
func (h *CompositeHandler) RemoveComponent(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	modelID, ok := requireURLParam(w, r, "model_id")
	if !ok {
		return
	}
	ctx := r.Context()
	if err := h.repo().RemoveComponent(ctx, id, modelID); err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to remove component", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type crossJoinRequest struct {
	Name          string `json:"name"`
	FromModel     string `json:"from_model"`
	FromDimension string `json:"from_dimension"`
	ToModel       string `json:"to_model"`
	ToDimension   string `json:"to_dimension"`
	JoinType      string `json:"join_type,omitempty"`
	Relationship  string `json:"relationship,omitempty"`
}

func (req *crossJoinRequest) toCrossModelJoin(compositeID string) semantic.CrossModelJoin {
	joinType := strings.TrimSpace(req.JoinType)
	if joinType == "" {
		joinType = semantic.DefaultJoinType
	}
	relationship := strings.TrimSpace(req.Relationship)
	if relationship == "" {
		relationship = semantic.DefaultRelationshipType
	}
	return semantic.CrossModelJoin{
		CompositeID:   compositeID,
		Name:          strings.TrimSpace(req.Name),
		FromModel:     strings.TrimSpace(req.FromModel),
		FromDimension: strings.TrimSpace(req.FromDimension),
		ToModel:       strings.TrimSpace(req.ToModel),
		ToDimension:   strings.TrimSpace(req.ToDimension),
		JoinType:      joinType,
		Relationship:  relationship,
		IsActive:      true,
	}
}

func (req *crossJoinRequest) validate() string {
	if strings.TrimSpace(req.FromModel) == "" || strings.TrimSpace(req.ToModel) == "" {
		return "from_model and to_model are required"
	}
	if strings.TrimSpace(req.FromDimension) == "" || strings.TrimSpace(req.ToDimension) == "" {
		return "from_dimension and to_dimension are required"
	}
	return ""
}

// AddCrossJoin adds a cross-model join between two component aliases.
func (h *CompositeHandler) AddCrossJoin(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	req, ok := decodeJSON[crossJoinRequest](w, r)
	if !ok {
		return
	}
	if msg := req.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	ctx := r.Context()
	if err := h.repo().AddCrossModelJoin(ctx, id, req.toCrossModelJoin(id)); err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to add cross-model join", err)
		return
	}
	h.respondFull(ctx, w, id, http.StatusCreated)
}

// UpdateCrossJoin updates an existing cross-model join.
func (h *CompositeHandler) UpdateCrossJoin(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	joinID, ok := requireURLParam(w, r, "join_id")
	if !ok {
		return
	}
	req, ok := decodeJSON[crossJoinRequest](w, r)
	if !ok {
		return
	}
	if msg := req.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	ctx := r.Context()
	join := req.toCrossModelJoin(id)
	join.ID = joinID
	if err := h.repo().UpdateCrossModelJoin(ctx, id, join); err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to update cross-model join", err)
		return
	}
	h.respondFull(ctx, w, id, http.StatusOK)
}

// RemoveCrossJoin deletes a cross-model join.
func (h *CompositeHandler) RemoveCrossJoin(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	joinID, ok := requireURLParam(w, r, "join_id")
	if !ok {
		return
	}
	ctx := r.Context()
	if err := h.repo().RemoveCrossModelJoin(ctx, id, joinID); err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to remove cross-model join", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type canonicalDateRequest struct {
	ModelAlias    string `json:"model_alias"`
	DimensionName string `json:"dimension_name"`
}

// SetCanonicalDate sets (or clears) the shared canonical date dimension.
func (h *CompositeHandler) SetCanonicalDate(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	req, ok := decodeJSONAllowEmpty[canonicalDateRequest](w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	var ref *semantic.CanonicalDateRef
	alias := strings.TrimSpace(req.ModelAlias)
	dim := strings.TrimSpace(req.DimensionName)
	if alias != "" || dim != "" {
		if alias == "" || dim == "" {
			writeError(w, http.StatusBadRequest, "model_alias and dimension_name are both required")
			return
		}
		ref = &semantic.CanonicalDateRef{ModelAlias: alias, DimensionName: dim}
	}
	if err := h.repo().SetCanonicalDate(ctx, id, ref); err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to set canonical date", err)
		return
	}
	h.respondFull(ctx, w, id, http.StatusOK)
}

type dimensionResolutionsRequest struct {
	Resolutions []dimensionResolutionItem `json:"resolutions"`
}

type dimensionResolutionItem struct {
	DimensionName string `json:"dimension_name"`
	Resolution    string `json:"resolution"`
	SourceAlias   string `json:"source_alias,omitempty"`
	TargetAlias   string `json:"target_alias,omitempty"`
}

// SetDimensionResolutions replaces the composite's dimension conflict resolutions.
func (h *CompositeHandler) SetDimensionResolutions(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	req, ok := decodeJSON[dimensionResolutionsRequest](w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	for _, item := range req.Resolutions {
		name := strings.TrimSpace(item.DimensionName)
		if name == "" {
			writeError(w, http.StatusBadRequest, "dimension_name is required for each resolution")
			return
		}
		res := semantic.DimensionConflictResolution{
			CompositeID:   id,
			DimensionName: name,
			Resolution:    strings.TrimSpace(item.Resolution),
			SourceAlias:   strings.TrimSpace(item.SourceAlias),
			TargetAlias:   strings.TrimSpace(item.TargetAlias),
		}
		if err := h.repo().SetDimensionResolution(ctx, id, res); err != nil {
			writeInternalError(ctx, w, http.StatusInternalServerError, "failed to set dimension resolution", err)
			return
		}
	}
	h.respondFull(ctx, w, id, http.StatusOK)
}

// ValidateComposite checks whether a draft composite can be published.
func (h *CompositeHandler) ValidateComposite(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	resolved, result, err := h.repo().ValidateComposite(ctx, id, h.componentProvider())
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to validate composite model", err)
		return
	}
	writeJSON(w, http.StatusOK, semantic.CompositePublishResult{
		Resolved:   resolved,
		Validation: result,
	})
}

type compositePublishRequest struct {
	PublishedBy string `json:"published_by,omitempty"`
}

// PublishComposite validates and publishes a draft composite model.
func (h *CompositeHandler) PublishComposite(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	req, ok := decodeJSONAllowEmpty[compositePublishRequest](w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	result, err := h.repo().PublishComposite(ctx, id, req.PublishedBy, h.componentProvider())
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to publish composite model", err)
		return
	}
	if !result.Validation.Valid {
		writeJSON(w, http.StatusUnprocessableEntity, result)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

type compositeRollbackRequest struct {
	Version     int    `json:"version,omitempty"`
	PublishedBy string `json:"published_by,omitempty"`
}

// RollbackComposite restores a previously published composite version.
func (h *CompositeHandler) RollbackComposite(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	req, ok := decodeJSONAllowEmpty[compositeRollbackRequest](w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	result, err := h.repo().RollbackComposite(ctx, id, req.Version, req.PublishedBy)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to rollback composite model", err)
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// SuggestedJoinsResponse lists heuristically inferred cross-model joins.
type SuggestedJoinsResponse struct {
	Suggestions []SuggestedCrossJoin `json:"suggestions"`
}

// SuggestedCrossJoin is a candidate cross-model join between two components.
type SuggestedCrossJoin struct {
	FromModel     string `json:"from_model"`
	FromDimension string `json:"from_dimension"`
	ToModel       string `json:"to_model"`
	ToDimension   string `json:"to_dimension"`
	Reason        string `json:"reason"`
}

// SuggestedJoins proposes cross-model joins by matching dimensions that share a
// name and type across two component models — a strong signal of a shared key
// (e.g. customer_id present in both sales and customer domains).
func (h *CompositeHandler) SuggestedJoins(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	components, err := h.repo().GetComponents(ctx, id)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to load components", err)
		return
	}
	models := make(map[string]*semantic.SemanticModel, len(components))
	for _, comp := range components {
		model, err := h.deps.SemanticRepo.GetPublishedFullModel(ctx, comp.ModelID)
		if err != nil {
			continue
		}
		models[comp.Alias] = model
	}
	suggestions := suggestCrossModelJoins(components, models)
	writeJSON(w, http.StatusOK, SuggestedJoinsResponse{Suggestions: suggestions})
}

// suggestCrossModelJoins pairs every two components and proposes a join wherever
// a dimension name (and type) appears in both. Each unordered pair is considered
// once to avoid duplicate suggestions.
func suggestCrossModelJoins(components []semantic.ComponentModelRef, models map[string]*semantic.SemanticModel) []SuggestedCrossJoin {
	var out []SuggestedCrossJoin
	for i := 0; i < len(components); i++ {
		from := models[components[i].Alias]
		if from == nil {
			continue
		}
		for j := i + 1; j < len(components); j++ {
			to := models[components[j].Alias]
			if to == nil {
				continue
			}
			out = append(out, matchSharedDimensions(components[i].Alias, from, components[j].Alias, to)...)
		}
	}
	return out
}

func matchSharedDimensions(fromAlias string, from *semantic.SemanticModel, toAlias string, to *semantic.SemanticModel) []SuggestedCrossJoin {
	toByName := make(map[string]semantic.Dimension, len(to.Dimensions))
	for _, d := range to.Dimensions {
		toByName[strings.ToLower(d.Name)] = d
	}
	var out []SuggestedCrossJoin
	for _, fd := range from.Dimensions {
		td, ok := toByName[strings.ToLower(fd.Name)]
		if !ok {
			continue
		}
		if fd.Type != "" && td.Type != "" && fd.Type != td.Type {
			continue
		}
		out = append(out, SuggestedCrossJoin{
			FromModel:     fromAlias,
			FromDimension: fd.Name,
			ToModel:       toAlias,
			ToDimension:   td.Name,
			Reason:        "shared dimension name and type",
		})
	}
	return out
}

func (h *CompositeHandler) respondFull(ctx context.Context, w http.ResponseWriter, id string, status int) {
	full, err := h.repo().GetFullComposite(ctx, id)
	if err != nil {
		writeJSON(w, status, map[string]string{"id": id})
		return
	}
	writeJSON(w, status, full)
}
