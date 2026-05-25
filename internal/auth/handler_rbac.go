package auth

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

type RBACHandler struct {
	rbac     *RBACService
	rbacRepo *RBACRepository
	dsAccess *DatasourceAccessService
	ws       *WorkspaceService
	sharing  *SharingService
	jwtMgr   *JWTManager
	cfg      *Config
}

func NewRBACHandler(
	rbac *RBACService,
	rbacRepo *RBACRepository,
	dsAccess *DatasourceAccessService,
	ws *WorkspaceService,
	sharing *SharingService,
	jwtMgr *JWTManager,
	cfg *Config,
) *RBACHandler {
	return &RBACHandler{
		rbac:     rbac,
		rbacRepo: rbacRepo,
		dsAccess: dsAccess,
		ws:       ws,
		sharing:  sharing,
		jwtMgr:   jwtMgr,
		cfg:      cfg,
	}
}

func (h *RBACHandler) RegisterRoutes(r chi.Router, authMW func(http.Handler) http.Handler, internalMW func(http.Handler) http.Handler) {
	r.Route("/auth", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(authMW)

			r.Get("/workspaces", h.handleListWorkspaces)
			r.Post("/workspaces", h.handleCreateWorkspace)
			r.Get("/workspaces/{id}", h.handleGetWorkspace)
			r.Put("/workspaces/{id}", h.handleUpdateWorkspace)
			r.Delete("/workspaces/{id}", h.handleDeleteWorkspace)

			r.Get("/workspaces/{id}/members", h.handleListMembers)
			r.Post("/workspaces/{id}/members", h.handleAddMember)
			r.Put("/workspaces/{id}/members/{userId}", h.handleUpdateMemberRole)
			r.Delete("/workspaces/{id}/members/{userId}", h.handleRemoveMember)

			r.Get("/workspaces/{id}/datasources", h.handleListWorkspaceDatasources)
			r.Post("/workspaces/{id}/datasources", h.handleAttachDatasource)
			r.Delete("/workspaces/{id}/datasources/{dsId}", h.handleDetachDatasource)

			r.Get("/me/datasources", h.handleListMyDatasources)
			r.Get("/me/datasources/{id}/check", h.handleCheckMyDatasource)

			r.Post("/shares", h.handleCreateShare)
			r.Get("/shares", h.handleListShares)
			r.Delete("/shares/{id}", h.handleRevokeShare)

			r.Route("/admin", func(r chi.Router) {
				r.Get("/datasource-access", h.handleAdminListAccess)
				r.Post("/datasource-access", h.handleAdminGrantAccess)
				r.Put("/datasource-access/{id}", h.handleAdminUpdateAccess)
				r.Delete("/datasource-access/{id}", h.handleAdminRevokeAccess)

				r.Get("/roles", h.handleAdminListRoles)
				r.Get("/permissions", h.handleAdminListPermissions)
				r.Post("/users/{id}/roles", h.handleAdminAssignRole)
				r.Delete("/users/{id}/roles/{roleId}", h.handleAdminRemoveRole)
			})
		})
	})

	r.Route("/internal/auth", func(r chi.Router) {
		r.Use(internalMW)
		r.Post("/check-permission", h.handleInternalCheckPermission)
		r.Get("/user/{id}/datasources", h.handleInternalUserDatasources)
		r.Post("/check-datasource-access", h.handleInternalCheckDSAccess)
		r.Get("/user/{id}/workspaces", h.handleInternalUserWorkspaces)
		r.Post("/invalidate-cache", h.handleInternalInvalidateCache)
		r.Get("/public-key", h.handleInternalPublicKey)
	})
}

// === Workspace ===

func (h *RBACHandler) handleListWorkspaces(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	list, err := h.ws.ListForUser(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *RBACHandler) handleCreateWorkspace(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	ws, err := h.ws.Create(r.Context(), req.Name, req.Description, userID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, ws)
}

func (h *RBACHandler) handleGetWorkspace(w http.ResponseWriter, r *http.Request) {
	ws, err := h.ws.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, ws)
}

func (h *RBACHandler) handleUpdateWorkspace(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	ws, err := h.ws.Update(r.Context(), chi.URLParam(r, "id"), userID, req.Name, req.Description)
	if err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}
	writeJSON(w, http.StatusOK, ws)
}

func (h *RBACHandler) handleDeleteWorkspace(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	if err := h.ws.Delete(r.Context(), chi.URLParam(r, "id"), userID); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *RBACHandler) handleListMembers(w http.ResponseWriter, r *http.Request) {
	list, err := h.ws.ListMembers(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *RBACHandler) handleAddMember(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	var req struct {
		UserID string `json:"user_id"`
		RoleID string `json:"role_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := h.ws.AddMember(r.Context(), chi.URLParam(r, "id"), req.UserID, req.RoleID, userID); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (h *RBACHandler) handleUpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	var req struct {
		RoleID string `json:"role_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := h.ws.UpdateMemberRole(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "userId"), req.RoleID, userID); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *RBACHandler) handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	if err := h.ws.RemoveMember(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "userId"), userID); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *RBACHandler) handleListWorkspaceDatasources(w http.ResponseWriter, r *http.Request) {
	list, err := h.ws.ListDatasources(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *RBACHandler) handleAttachDatasource(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	var req struct {
		DatasourceID string `json:"datasource_id"`
		AccessLevel  string `json:"access_level"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.AccessLevel == "" {
		req.AccessLevel = "read"
	}
	if err := h.ws.AttachDatasource(r.Context(), chi.URLParam(r, "id"), req.DatasourceID, req.AccessLevel, userID); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (h *RBACHandler) handleDetachDatasource(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	if err := h.ws.DetachDatasource(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "dsId"), userID); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// === Datasource access ===

func (h *RBACHandler) handleListMyDatasources(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	ids, err := h.dsAccess.ListAccessibleDatasourceIDs(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"datasource_ids": ids})
}

func (h *RBACHandler) handleCheckMyDatasource(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	dsID := chi.URLParam(r, "id")
	level := r.URL.Query().Get("level")
	if level == "" {
		level = "read"
	}
	err := h.dsAccess.CheckAccess(r.Context(), userID, dsID, level)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"allowed": false, "reason": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"allowed": true})
}

// === Sharing ===

func (h *RBACHandler) handleCreateShare(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	var req ShareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	share, err := h.sharing.Share(r.Context(), userID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, share)
}

func (h *RBACHandler) handleListShares(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	resourceType := r.URL.Query().Get("resource_type")
	mode := r.URL.Query().Get("mode")
	if mode == "owned" {
		list, err := h.sharing.ListOwned(r.Context(), userID, resourceType, r.URL.Query().Get("resource_id"))
		if err != nil {
			writeError(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, list)
		return
	}
	list, err := h.sharing.ListShared(r.Context(), userID, resourceType)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *RBACHandler) handleRevokeShare(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	if err := h.sharing.Revoke(r.Context(), chi.URLParam(r, "id"), userID); err != nil {
		writeError(w, http.StatusForbidden, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// === Admin ===

func (h *RBACHandler) handleAdminListAccess(w http.ResponseWriter, r *http.Request) {
	list, err := h.dsAccess.ListAll(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *RBACHandler) handleAdminGrantAccess(w http.ResponseWriter, r *http.Request) {
	caller := r.Context().Value(userIDKey).(string)
	var req struct {
		UserID       string `json:"user_id"`
		DatasourceID string `json:"datasource_id"`
		AccessLevel  string `json:"access_level"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.AccessLevel == "" {
		req.AccessLevel = "read"
	}
	access, err := h.dsAccess.Grant(r.Context(), req.UserID, req.DatasourceID, req.AccessLevel, caller)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, access)
}

func (h *RBACHandler) handleAdminUpdateAccess(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AccessLevel string `json:"access_level"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := h.dsAccess.UpdateLevel(r.Context(), chi.URLParam(r, "id"), req.AccessLevel); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *RBACHandler) handleAdminRevokeAccess(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID       string `json:"user_id"`
		DatasourceID string `json:"datasource_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := h.dsAccess.Revoke(r.Context(), req.UserID, req.DatasourceID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *RBACHandler) handleAdminListRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := h.rbacRepo.ListRoles(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, roles)
}

func (h *RBACHandler) handleAdminListPermissions(w http.ResponseWriter, r *http.Request) {
	perms, err := h.rbacRepo.ListPermissions(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, perms)
}

func (h *RBACHandler) handleAdminAssignRole(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	var req struct {
		RoleID    string  `json:"role_id"`
		ScopeType *string `json:"scope_type,omitempty"`
		ScopeID   *string `json:"scope_id,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if err := h.rbacRepo.AssignRole(r.Context(), userID, req.RoleID, req.ScopeType, req.ScopeID); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (h *RBACHandler) handleAdminRemoveRole(w http.ResponseWriter, r *http.Request) {
	if err := h.rbacRepo.RemoveRole(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "roleId")); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// === Internal endpoints ===

type CheckPermissionRequest struct {
	UserID      string `json:"user_id"`
	Permission  string `json:"permission"`
	ScopeType   string `json:"scope_type,omitempty"`
	ScopeID     string `json:"scope_id,omitempty"`
	WorkspaceID string `json:"workspace_id,omitempty"`
}

func (h *RBACHandler) handleInternalCheckPermission(w http.ResponseWriter, r *http.Request) {
	var req CheckPermissionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	scopeID := req.ScopeID
	if scopeID == "" {
		scopeID = req.WorkspaceID
	}
	allowed, err := h.rbac.Check(r.Context(), PermissionCheck{
		UserID:     req.UserID,
		Permission: req.Permission,
		ScopeType:  ScopeType(req.ScopeType),
		ScopeID:    scopeID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"allowed": allowed})
}

func (h *RBACHandler) handleInternalUserDatasources(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	ids, err := h.dsAccess.ListAccessibleDatasourceIDs(r.Context(), userID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"datasource_ids": ids})
}

type CheckDSAccessRequest struct {
	UserID       string `json:"user_id"`
	DatasourceID string `json:"datasource_id"`
	Level        string `json:"level"`
}

func (h *RBACHandler) handleInternalCheckDSAccess(w http.ResponseWriter, r *http.Request) {
	var req CheckDSAccessRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if req.Level == "" {
		req.Level = "read"
	}
	err := h.dsAccess.CheckAccess(r.Context(), req.UserID, req.DatasourceID, req.Level)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"allowed": false, "reason": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"allowed": true})
}

func (h *RBACHandler) handleInternalUserWorkspaces(w http.ResponseWriter, r *http.Request) {
	list, err := h.ws.ListForUser(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *RBACHandler) handleInternalInvalidateCache(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID string `json:"user_id"`
		Scope  string `json:"scope"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	_ = h.dsAccess.InvalidateCache(r.Context(), req.UserID)
	w.WriteHeader(http.StatusNoContent)
}

func (h *RBACHandler) handleInternalPublicKey(w http.ResponseWriter, r *http.Request) {
	pem, err := h.jwtMgr.GetPublicKeyPEM()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"public_key": pem})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}
