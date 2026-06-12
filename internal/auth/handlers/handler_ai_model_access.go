package handlers

import (
	"context"
	"net/http"

	"github.com/biqly/biqly/internal/auth/rbac"
	"github.com/biqly/biqly/internal/http/response"
	"github.com/go-chi/chi/v5"
)

func (h *RBACHandler) registerAIModelAccessAdminRoutes(r chi.Router) {
	r.Get("/ai-model-access", h.handleAdminListAIModelAccess)
	r.Post("/ai-model-access/workspace/provider", h.handleAdminGrantProviderWorkspace)
	r.Delete("/ai-model-access/workspace/provider", h.handleAdminRevokeProviderWorkspace)
	r.Post("/ai-model-access/workspace/model", h.handleAdminGrantModelWorkspace)
	r.Delete("/ai-model-access/workspace/model", h.handleAdminRevokeModelWorkspace)
	r.Post("/ai-model-access/role/provider", h.handleAdminGrantProviderRole)
	r.Delete("/ai-model-access/role/provider", h.handleAdminRevokeProviderRole)
	r.Post("/ai-model-access/role/model", h.handleAdminGrantModelRole)
	r.Delete("/ai-model-access/role/model", h.handleAdminRevokeModelRole)
}

// userSelectableAIPurpose mirrors ai.UserSelectablePurpose: only purposes that
// are resolved per-user at runtime and safe to vary per user may be stored as a
// personal preference. Kept as a local list to avoid importing the ai package
// into the auth service.
func userSelectableAIPurpose(p string) bool {
	return p == "query" || p == "describe"
}

func (h *RBACHandler) registerAIModelAccessUserRoutes(r chi.Router) {
	r.Get("/me/ai-preferences", h.handleGetMyAIPreferences)
	r.Put("/me/ai-preferences", h.handlePutMyAIPreferences)
	r.Delete("/me/ai-preferences/{purpose}", h.handleDeleteMyAIPreference)
}

func (h *RBACHandler) registerAIModelAccessInternalRoutes(r chi.Router) {
	r.Get("/user/{id}/ai-access", h.handleInternalUserAIAccess)
	r.Get("/user/{id}/ai-preferences", h.handleInternalListUserAIPreferences)
	r.Put("/user/{id}/ai-preferences", h.handleInternalPutUserAIPreference)
	r.Delete("/user/{id}/ai-preferences/{purpose}", h.handleInternalDeleteUserAIPreference)
}

func (h *RBACHandler) handleAdminListAIModelAccess(w http.ResponseWriter, r *http.Request) {
	grants, err := h.aiModelAccess.ListAllGrants(r.Context())
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.WriteJSON(w, http.StatusOK, grants)
}

type grantProviderWorkspaceReq struct {
	WorkspaceID string `json:"workspace_id"`
	ProviderID  string `json:"provider_id"`
}

func decodeGrantRequest[T any](w http.ResponseWriter, r *http.Request) (T, bool) {
	return decodeJSON[T](w, r)
}

func (*RBACHandler) handleAdminGrant(w http.ResponseWriter, r *http.Request, grant func(context.Context, string) error) {
	userID, ok := requireContextUserID(w, r)
	if !ok {
		return
	}
	if err := grant(r.Context(), userID); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (*RBACHandler) handleAdminRevoke(w http.ResponseWriter, r *http.Request, revoke func(context.Context) error) {
	if err := revoke(r.Context()); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *RBACHandler) handleAdminGrantProviderWorkspace(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeGrantRequest[grantProviderWorkspaceReq](w, r)
	if !ok {
		return
	}
	h.handleAdminGrant(w, r, func(ctx context.Context, userID string) error {
		return h.aiModelAccess.GrantProviderWorkspace(ctx, req.WorkspaceID, req.ProviderID, userID)
	})
}

func (h *RBACHandler) handleAdminRevokeProviderWorkspace(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeGrantRequest[grantProviderWorkspaceReq](w, r)
	if !ok {
		return
	}
	h.handleAdminRevoke(w, r, func(ctx context.Context) error {
		return h.aiModelAccess.RevokeProviderWorkspace(ctx, req.WorkspaceID, req.ProviderID)
	})
}

type grantModelWorkspaceReq struct {
	WorkspaceID string `json:"workspace_id"`
	ModelID     string `json:"model_id"`
}

func (h *RBACHandler) handleAdminGrantModelWorkspace(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeGrantRequest[grantModelWorkspaceReq](w, r)
	if !ok {
		return
	}
	h.handleAdminGrant(w, r, func(ctx context.Context, userID string) error {
		return h.aiModelAccess.GrantModelWorkspace(ctx, req.WorkspaceID, req.ModelID, userID)
	})
}

func (h *RBACHandler) handleAdminRevokeModelWorkspace(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeGrantRequest[grantModelWorkspaceReq](w, r)
	if !ok {
		return
	}
	h.handleAdminRevoke(w, r, func(ctx context.Context) error {
		return h.aiModelAccess.RevokeModelWorkspace(ctx, req.WorkspaceID, req.ModelID)
	})
}

type grantProviderRoleReq struct {
	RoleID     string `json:"role_id"`
	ProviderID string `json:"provider_id"`
}

func (h *RBACHandler) handleAdminGrantProviderRole(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeGrantRequest[grantProviderRoleReq](w, r)
	if !ok {
		return
	}
	h.handleAdminGrant(w, r, func(ctx context.Context, userID string) error {
		return h.aiModelAccess.GrantProviderRole(ctx, req.RoleID, req.ProviderID, userID)
	})
}

func (h *RBACHandler) handleAdminRevokeProviderRole(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeGrantRequest[grantProviderRoleReq](w, r)
	if !ok {
		return
	}
	h.handleAdminRevoke(w, r, func(ctx context.Context) error {
		return h.aiModelAccess.RevokeProviderRole(ctx, req.RoleID, req.ProviderID)
	})
}

type grantModelRoleReq struct {
	RoleID  string `json:"role_id"`
	ModelID string `json:"model_id"`
}

func (h *RBACHandler) handleAdminGrantModelRole(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeGrantRequest[grantModelRoleReq](w, r)
	if !ok {
		return
	}
	h.handleAdminGrant(w, r, func(ctx context.Context, userID string) error {
		return h.aiModelAccess.GrantModelRole(ctx, req.RoleID, req.ModelID, userID)
	})
}

func (h *RBACHandler) handleAdminRevokeModelRole(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeGrantRequest[grantModelRoleReq](w, r)
	if !ok {
		return
	}
	h.handleAdminRevoke(w, r, func(ctx context.Context) error {
		return h.aiModelAccess.RevokeModelRole(ctx, req.RoleID, req.ModelID)
	})
}

func (h *RBACHandler) handleGetMyAIPreferences(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextUserID(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	prefs, err := h.aiModelAccess.ListUserPreferences(r.Context(), userID)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.WriteJSON(w, http.StatusOK, map[string]any{"preferences": prefs})
}

type putAIPrefsReq struct {
	Preferences []rbac.UserAIModelPreference `json:"preferences"`
}

func (h *RBACHandler) handlePutMyAIPreferences(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextUserID(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	req, ok := decodeJSON[putAIPrefsReq](w, r)
	if !ok {
		return
	}
	for _, p := range req.Preferences {
		if p.Purpose == "" || p.ModelID == "" {
			continue
		}
		if !userSelectableAIPurpose(p.Purpose) {
			response.WriteError(w, http.StatusBadRequest, "purpose not user-selectable: "+p.Purpose)
			return
		}
		ok, err := h.aiModelAccess.CanUseModel(r.Context(), userID, p.ModelID)
		if err != nil {
			response.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if !ok {
			response.WriteError(w, http.StatusForbidden, "model not allowed for this user")
			return
		}
		if err := h.aiModelAccess.SetUserPreference(r.Context(), userID, p.Purpose, p.ModelID); err != nil {
			response.WriteError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}
	prefs, err := h.aiModelAccess.ListUserPreferences(r.Context(), userID)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.WriteJSON(w, http.StatusOK, map[string]any{"preferences": prefs})
}

func (h *RBACHandler) handleDeleteMyAIPreference(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextUserID(r)
	if !ok {
		response.WriteError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	purpose := chi.URLParam(r, "purpose")
	if err := h.aiModelAccess.DeleteUserPreference(r.Context(), userID, purpose); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *RBACHandler) handleInternalUserAIAccess(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	access, err := h.aiModelAccess.UserAIAccess(r.Context(), userID)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.WriteJSON(w, http.StatusOK, access)
}

func (h *RBACHandler) handleInternalListUserAIPreferences(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	prefs, err := h.aiModelAccess.ListUserPreferences(r.Context(), userID)
	if err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	response.WriteJSON(w, http.StatusOK, map[string]any{"preferences": prefs})
}

type internalPutAIPrefReq struct {
	Purpose string `json:"purpose"`
	ModelID string `json:"model_id"`
}

func (h *RBACHandler) handleInternalPutUserAIPreference(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	req, ok := decodeJSON[internalPutAIPrefReq](w, r)
	if !ok {
		return
	}
	if req.Purpose == "" || req.ModelID == "" {
		response.WriteError(w, http.StatusBadRequest, "purpose and model_id are required")
		return
	}
	if !userSelectableAIPurpose(req.Purpose) {
		response.WriteError(w, http.StatusBadRequest, "purpose not user-selectable: "+req.Purpose)
		return
	}
	if err := h.aiModelAccess.SetUserPreference(r.Context(), userID, req.Purpose, req.ModelID); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *RBACHandler) handleInternalDeleteUserAIPreference(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	purpose := chi.URLParam(r, "purpose")
	if err := h.aiModelAccess.DeleteUserPreference(r.Context(), userID, purpose); err != nil {
		response.WriteError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
