package handlers

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/bytedance/sonic"
	"github.com/go-chi/chi/v5"

	"github.com/biqly/biqly/internal/app"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/query"
)

// AISkillsHandler serves the skills library: saved, parameterized
// LogicalQuery templates that re-run through the governed query path
// without fresh LLM generation.
type AISkillsHandler struct {
	deps *app.AIDeps
}

// NewAISkillsHandler creates an AISkillsHandler.
func NewAISkillsHandler(deps *app.AIDeps) *AISkillsHandler {
	return &AISkillsHandler{deps: deps}
}

// SkillParameter declares one substitutable slot in a skill's LogicalQuery.
// Filters whose value is the placeholder string "{{name}}" are replaced at
// run time with the caller-provided (or default) value.
type SkillParameter struct {
	Name     string `json:"name"`
	Label    string `json:"label,omitempty"`
	Type     string `json:"type,omitempty"`
	Required bool   `json:"required,omitempty"`
	Default  any    `json:"default,omitempty"`
}

type skillPayload struct {
	DatasourceID string              `json:"datasource_id"`
	ModelID      string              `json:"model_id,omitempty"`
	Name         string              `json:"name"`
	Description  string              `json:"description,omitempty"`
	Question     string              `json:"question,omitempty"`
	LogicalQuery *query.LogicalQuery `json:"logical_query"`
	Parameters   []SkillParameter    `json:"parameters,omitempty"`
	Tags         []string            `json:"tags,omitempty"`
	IsActive     *bool               `json:"is_active,omitempty"`
}

type skillResponse struct {
	ID             string              `json:"id"`
	DatasourceID   string              `json:"datasource_id"`
	ModelID        string              `json:"model_id,omitempty"`
	Name           string              `json:"name"`
	Description    string              `json:"description"`
	Question       string              `json:"question"`
	LogicalQuery   *query.LogicalQuery `json:"logical_query"`
	Parameters     []SkillParameter    `json:"parameters"`
	Tags           []string            `json:"tags"`
	CreatedBy      string              `json:"created_by"`
	Version        int                 `json:"version"`
	IsActive       bool                `json:"is_active"`
	LastVerifiedAt *string             `json:"last_verified_at,omitempty"`
	CreatedAt      string              `json:"created_at"`
	UpdatedAt      string              `json:"updated_at"`
}

func skillRowToResponse(row metadata.SavedQueryRow) (skillResponse, error) {
	resp := skillResponse{
		ID:           row.ID,
		DatasourceID: row.DatasourceID,
		ModelID:      row.ModelID,
		Name:         row.Name,
		Description:  row.Description,
		Question:     row.Question,
		Parameters:   []SkillParameter{},
		Tags:         row.Tags,
		CreatedBy:    row.CreatedBy,
		Version:      row.Version,
		IsActive:     row.IsActive,
		CreatedAt:    row.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:    row.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if row.LastVerifiedAt != nil {
		resp.LastVerifiedAt = new(row.LastVerifiedAt.UTC().Format(time.RFC3339))
	}
	if len(row.LogicalQuery) > 0 {
		var lq query.LogicalQuery
		if err := sonic.Unmarshal(row.LogicalQuery, &lq); err != nil {
			return resp, fmt.Errorf("decode skill logical query: %w", err)
		}
		resp.LogicalQuery = &lq
	}
	if len(row.Parameters) > 0 {
		if err := sonic.Unmarshal(row.Parameters, &resp.Parameters); err != nil {
			return resp, fmt.Errorf("decode skill parameters: %w", err)
		}
	}
	return resp, nil
}

// List returns skills, optionally filtered by ?datasource_id=.
func (h *AISkillsHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := h.deps.MetaRepo.ListSavedQueries(ctx, metadata.SavedQueryFilter{
		DatasourceID: r.URL.Query().Get("datasource_id"),
		RunnableOnly: true,
	})
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to list skills", err)
		return
	}
	out := make([]skillResponse, 0, len(rows))
	for _, row := range rows {
		resp, err := skillRowToResponse(row)
		if err != nil {
			writeInternalError(ctx, w, http.StatusInternalServerError, "failed to decode skill", err)
			return
		}
		out = append(out, resp)
	}
	writeJSON(w, http.StatusOK, map[string]any{"skills": out})
}

// Get returns one skill by id.
func (h *AISkillsHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	row, err := h.deps.MetaRepo.GetSavedQuery(ctx, chi.URLParam(r, "id"))
	if err != nil {
		if errors.Is(err, metadata.ErrSavedQueryNotFound) {
			writeEntityNotFound(w, "skill")
			return
		}
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to load skill", err)
		return
	}
	resp, err := skillRowToResponse(row)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to decode skill", err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

func validateSkillPayload(p *skillPayload) string {
	if strings.TrimSpace(p.Name) == "" {
		return "name is required"
	}
	if p.LogicalQuery == nil || len(p.LogicalQuery.Select) == 0 {
		return "logical_query with a non-empty select is required"
	}
	for i := range p.Parameters {
		if strings.TrimSpace(p.Parameters[i].Name) == "" {
			return "parameter name is required"
		}
	}
	return ""
}

// Create stores a new skill.
func (h *AISkillsHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	payload, ok := decodeJSON[skillPayload](w, r)
	if !ok {
		return
	}
	if payload.DatasourceID == "" {
		writeError(w, http.StatusBadRequest, "datasource_id is required")
		return
	}
	if msg := validateSkillPayload(payload); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	lqJSON, paramsJSON, ok := marshalSkillTemplate(w, payload)
	if !ok {
		return
	}
	id, err := h.deps.MetaRepo.InsertSavedQuery(ctx, metadata.SavedQueryInsert{
		DatasourceID: payload.DatasourceID,
		ModelID:      payload.ModelID,
		Name:         strings.TrimSpace(payload.Name),
		Description:  payload.Description,
		Question:     payload.Question,
		LogicalQuery: lqJSON,
		Parameters:   paramsJSON,
		Tags:         payload.Tags,
		Source:       "skill",
		Runnable:     true,
		CreatedBy:    bimw.UserID(ctx),
	})
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to create skill", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]string{"id": id})
}

// Update replaces the editable fields of a skill and bumps its version.
func (h *AISkillsHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	payload, ok := decodeJSON[skillPayload](w, r)
	if !ok {
		return
	}
	if msg := validateSkillPayload(payload); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	lqJSON, paramsJSON, ok := marshalSkillTemplate(w, payload)
	if !ok {
		return
	}
	isActive := true
	if payload.IsActive != nil {
		isActive = *payload.IsActive
	}
	err := h.deps.MetaRepo.UpdateSavedQuery(ctx, chi.URLParam(r, "id"), metadata.SavedQueryUpdate{
		Name:         strings.TrimSpace(payload.Name),
		Description:  payload.Description,
		Question:     payload.Question,
		LogicalQuery: lqJSON,
		Parameters:   paramsJSON,
		Tags:         payload.Tags,
		IsActive:     isActive,
	})
	if err != nil {
		if errors.Is(err, metadata.ErrSavedQueryNotFound) {
			writeEntityNotFound(w, "skill")
			return
		}
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to update skill", err)
		return
	}
	writeOK(w)
}

// Delete removes a skill.
func (h *AISkillsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := h.deps.MetaRepo.DeleteSavedQuery(ctx, chi.URLParam(r, "id")); err != nil {
		if errors.Is(err, metadata.ErrSavedQueryNotFound) {
			writeEntityNotFound(w, "skill")
			return
		}
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to delete skill", err)
		return
	}
	writeOK(w)
}

type skillRunRequest struct {
	Parameters map[string]any `json:"parameters,omitempty"`
}

type skillRunResponse struct {
	SkillID string        `json:"skill_id"`
	Name    string        `json:"name"`
	SQL     string        `json:"sql,omitempty"`
	Result  *query.Result `json:"result"`
}

// Run executes a skill through the governed query path: parameter values are
// substituted into the stored LogicalQuery template, which is then compiled
// and executed by the query service with all policy layers applied.
func (h *AISkillsHandler) Run(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	req, ok := decodeJSONAllowEmpty[skillRunRequest](w, r)
	if !ok {
		return
	}
	row, err := h.deps.MetaRepo.GetSavedQuery(ctx, chi.URLParam(r, "id"))
	if err != nil {
		if errors.Is(err, metadata.ErrSavedQueryNotFound) {
			writeEntityNotFound(w, "skill")
			return
		}
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to load skill", err)
		return
	}
	if !row.Runnable {
		writeEntityNotFound(w, "skill")
		return
	}
	if !row.IsActive {
		writeError(w, http.StatusConflict, "skill is deactivated")
		return
	}
	resp, err := skillRowToResponse(row)
	if err != nil || resp.LogicalQuery == nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to decode skill", err)
		return
	}
	lq := *resp.LogicalQuery
	if err := applySkillParameters(&lq, resp.Parameters, req.Parameters); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	run, err := h.deps.QueryClient.RunWithModel(ctx, &lq, nil, 0, 0)
	if err != nil {
		writeCoreServiceError(ctx, w, err, "skill_id", row.ID)
		return
	}
	result := &query.Result{
		Columns: run.Columns,
		Rows:    run.Rows,
		Stats: query.Stats{
			RowCount:   run.RowCount,
			DurationMs: run.DurationMs,
		},
	}
	query.EnrichResult(result, &lq, nil)
	if err := h.deps.MetaRepo.TouchSavedQueryVerified(ctx, row.ID); err != nil {
		// Best-effort freshness marker; the run itself succeeded.
		slog.WarnContext(ctx, "failed to touch skill verified", appendRequestLogArgs(ctx, []any{"skill_id", row.ID, "error", err})...)
	}
	writeJSON(w, http.StatusOK, skillRunResponse{
		SkillID: row.ID,
		Name:    row.Name,
		SQL:     run.SQL,
		Result:  result,
	})
}

// applySkillParameters resolves parameter values (caller-provided, then
// declared defaults) and substitutes "{{name}}" placeholders in the top-level
// Filters and Having clauses. Missing required parameters or unresolved
// placeholders are errors.
func applySkillParameters(lq *query.LogicalQuery, defs []SkillParameter, provided map[string]any) error {
	values := make(map[string]any, len(defs))
	for _, def := range defs {
		if v, ok := provided[def.Name]; ok {
			values[def.Name] = v
			continue
		}
		if def.Default != nil {
			values[def.Name] = def.Default
			continue
		}
		if def.Required {
			return fmt.Errorf("missing required parameter %q", def.Name)
		}
	}
	if err := substituteFilterValues(lq.Filters, values); err != nil {
		return err
	}
	return substituteFilterValues(lq.Having, values)
}

func substituteFilterValues(filters []query.Filter, values map[string]any) error {
	for i := range filters {
		resolved, err := resolvePlaceholderValue(filters[i].Value, values)
		if err != nil {
			return fmt.Errorf("filter %q: %w", filters[i].Field, err)
		}
		filters[i].Value = resolved
	}
	return nil
}

func resolvePlaceholderValue(value any, values map[string]any) (any, error) {
	switch v := value.(type) {
	case string:
		name, isPlaceholder := placeholderName(v)
		if !isPlaceholder {
			return value, nil
		}
		resolved, ok := values[name]
		if !ok {
			return nil, fmt.Errorf("unresolved parameter %q", name)
		}
		return resolved, nil
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			resolved, err := resolvePlaceholderValue(item, values)
			if err != nil {
				return nil, err
			}
			out[i] = resolved
		}
		return out, nil
	default:
		return value, nil
	}
}

func placeholderName(s string) (string, bool) {
	trimmed := strings.TrimSpace(s)
	if !strings.HasPrefix(trimmed, "{{") || !strings.HasSuffix(trimmed, "}}") {
		return "", false
	}
	name := strings.TrimSpace(trimmed[2 : len(trimmed)-2])
	if name == "" {
		return "", false
	}
	return name, true
}

func marshalSkillTemplate(w http.ResponseWriter, payload *skillPayload) (lqJSON, paramsJSON []byte, ok bool) {
	lqJSON, err := sonic.Marshal(payload.LogicalQuery)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid logical_query")
		return nil, nil, false
	}
	params := payload.Parameters
	if params == nil {
		params = []SkillParameter{}
	}
	paramsJSON, err = sonic.Marshal(params)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid parameters")
		return nil, nil, false
	}
	return lqJSON, paramsJSON, true
}
