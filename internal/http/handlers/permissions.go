package handlers

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/audit"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/google/uuid"
)

type PermissionHandler struct {
	deps *app.CatalogDeps
}

func NewPermissionHandler(deps *app.CatalogDeps) *PermissionHandler {
	return &PermissionHandler{deps: deps}
}

type upsertPermissionRequest struct {
	ID            string                              `json:"id,omitempty"`
	UserID        string                              `json:"user_id"`
	DatasourceID  string                              `json:"datasource_id"`
	AllowedModels []string                            `json:"allowed_models"`
	DeniedFields  []string                            `json:"denied_fields"`
	RowFilters    []metadata.PermissionRowFilter      `json:"row_filters"`
	PIIPolicy     map[string]metadata.PIIColumnAccess `json:"pii_policy,omitempty"`
}

func (h *PermissionHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	policies, err := h.deps.MetaRepo.ListSecurityPolicies(ctx)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to list permission policies", err)
		return
	}
	writeJSON(w, http.StatusOK, policies)
}

func (h *PermissionHandler) GetByKeys(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	datasourceID := strings.TrimSpace(r.URL.Query().Get("datasource_id"))

	if userID == "" || datasourceID == "" {
		writeError(w, http.StatusBadRequest, "user_id and datasource_id query parameters are required")
		return
	}

	ctx := r.Context()
	policy, err := h.deps.MetaRepo.GetSecurityPolicyByKeys(ctx, userID, datasourceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeJSON(w, http.StatusOK, emptySecurityPolicy(userID, datasourceID))
			return
		}
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to get permission policy", err)
		return
	}
	writeJSON(w, http.StatusOK, policy)
}

func (h *PermissionHandler) Upsert(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[upsertPermissionRequest](w, r)
	if !ok {
		return
	}

	userID := strings.TrimSpace(req.UserID)
	datasourceID := strings.TrimSpace(req.DatasourceID)
	if userID == "" || datasourceID == "" {
		writeError(w, http.StatusBadRequest, "user_id and datasource_id are required")
		return
	}

	id := req.ID
	var previousPIIPolicy map[string]metadata.PIIColumnAccess
	if existing, err := h.deps.MetaRepo.GetSecurityPolicyByKeys(r.Context(), userID, datasourceID); err == nil && existing != nil {
		if id == "" {
			id = existing.ID
		}
		previousPIIPolicy = existing.PIIPolicy
	}
	if id == "" {
		id = uuid.New().String()
	}

	for col, entry := range req.PIIPolicy {
		switch entry.Access {
		case "raw", "masked", "hidden":
		default:
			writeError(w, http.StatusBadRequest, "invalid pii_policy access for column "+col+": must be raw, masked or hidden")
			return
		}
	}

	policy := &metadata.SecurityPolicy{
		ID:            id,
		UserID:        userID,
		DatasourceID:  datasourceID,
		AllowedModels: req.AllowedModels,
		DeniedFields:  req.DeniedFields,
		RowFilters:    req.RowFilters,
		PIIPolicy:     req.PIIPolicy,
	}
	if policy.AllowedModels == nil {
		policy.AllowedModels = []string{}
	}
	if policy.DeniedFields == nil {
		policy.DeniedFields = []string{}
	}
	if policy.RowFilters == nil {
		policy.RowFilters = []metadata.PermissionRowFilter{}
	}
	if policy.PIIPolicy == nil {
		policy.PIIPolicy = map[string]metadata.PIIColumnAccess{}
	}

	ctx := r.Context()
	if err := h.deps.MetaRepo.UpsertSecurityPolicy(ctx, policy); err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to upsert permission policy", err)
		return
	}

	if h.deps.AuditLogger != nil && piiPolicyChanged(previousPIIPolicy, policy.PIIPolicy) {
		h.deps.AuditLogger.Log(ctx, audit.Event{
			UserID:       bimw.UserID(ctx),
			EventType:    audit.EventPIIPolicyUpdated,
			DatasourceID: datasourceID,
			Details: map[string]any{
				"policy_user_id": userID,
				"old_pii_policy": previousPIIPolicy,
				"new_pii_policy": policy.PIIPolicy,
			},
		})
	}

	writeJSON(w, http.StatusOK, policy)
}

func piiPolicyChanged(oldPolicy, newPolicy map[string]metadata.PIIColumnAccess) bool {
	if len(oldPolicy) != len(newPolicy) {
		return true
	}
	for col, entry := range newPolicy {
		if oldPolicy[col] != entry {
			return true
		}
	}
	return false
}

func (h *PermissionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	if err := h.deps.MetaRepo.DeleteSecurityPolicy(ctx, id); err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to delete permission policy", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *PermissionHandler) DeleteByKeys(w http.ResponseWriter, r *http.Request) {
	userID := strings.TrimSpace(r.URL.Query().Get("user_id"))
	datasourceID := strings.TrimSpace(r.URL.Query().Get("datasource_id"))

	if userID == "" || datasourceID == "" {
		writeError(w, http.StatusBadRequest, "user_id and datasource_id query parameters are required")
		return
	}

	ctx := r.Context()
	if err := h.deps.MetaRepo.DeleteSecurityPolicyByKeys(ctx, userID, datasourceID); err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to delete permission policy", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func emptySecurityPolicy(userID, datasourceID string) metadata.SecurityPolicy {
	return metadata.SecurityPolicy{
		UserID:        userID,
		DatasourceID:  datasourceID,
		AllowedModels: []string{},
		DeniedFields:  []string{},
		RowFilters:    []metadata.PermissionRowFilter{},
		PIIPolicy:     map[string]metadata.PIIColumnAccess{},
	}
}
