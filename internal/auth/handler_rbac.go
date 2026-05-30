package auth

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"log/slog"
	"github.com/biqly/biqly/pkg/common/requestid")

type RBACHandler struct {
	rbac     *RBACService
	rbacRepo *RBACRepository
	userRepo *UserRepository
	dsAccess *DatasourceAccessService
	ws       *WorkspaceService
	sharing  *SharingService
	audit    *AuditService
	jwtMgr   *JWTManager
	cfg      *Config
}

func NewRBACHandler(
	rbac *RBACService,
	rbacRepo *RBACRepository,
	userRepo *UserRepository,
	dsAccess *DatasourceAccessService,
	ws *WorkspaceService,
	sharing *SharingService,
	audit *AuditService,
	jwtMgr *JWTManager,
	cfg *Config,
) *RBACHandler {
	return &RBACHandler{
		rbac:     rbac,
		rbacRepo: rbacRepo,
		userRepo: userRepo,
		dsAccess: dsAccess,
		ws:       ws,
		sharing:  sharing,
		audit:    audit,
		jwtMgr:   jwtMgr,
		cfg:      cfg,
	}
}

func (h *RBACHandler) RegisterRoutes(r chi.Router, authMW func(http.Handler) http.Handler, internalMW func(http.Handler) http.Handler) {
	r.Route("/auth", func(r chi.Router) { h.RegisterAuthRoutes(r, authMW) })
	r.Route("/internal/auth", func(r chi.Router) { h.RegisterInternalRoutes(r, internalMW) })
}

func (h *RBACHandler) RegisterAuthRoutes(r chi.Router, authMW func(http.Handler) http.Handler) {
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
		r.Post("/me/datasources/{id}/request-access", h.handleRequestAccess)

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

			r.Get("/users", h.handleAdminListUsers)
			r.Get("/users/{id}", h.handleAdminGetUser)
			r.Get("/users/{id}/roles", h.handleAdminGetUserRoles)
			r.Put("/users/{id}", h.handleAdminUpdateUser)

			r.Get("/audit-log", h.handleAdminListAuditLog)
		})
	})
}

func (h *RBACHandler) RegisterInternalRoutes(r chi.Router, internalMW func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(internalMW)
		r.Post("/check-permission", h.handleInternalCheckPermission)
		r.Get("/user/{id}/datasources", h.handleInternalUserDatasources)
		r.Post("/check-datasource-access", h.handleInternalCheckDSAccess)
		r.Get("/user/{id}/workspaces", h.handleInternalUserWorkspaces)
		r.Get("/workspaces/{id}/datasources", h.handleInternalWorkspaceDatasources)
		r.Post("/invalidate-cache", h.handleInternalInvalidateCache)
		r.Get("/public-key", h.handleInternalPublicKey)
	})
}

// === Workspace ===

func (h *RBACHandler) handleListWorkspaces(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	list, err := h.ws.ListForUser(r.Context(), userID)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	paginated, total := paginateSlice(r, list)
	writeJSON(w, http.StatusOK, map[string]any{
		"workspaces": paginated,
		"total":      total,
	})
}

func (h *RBACHandler) handleCreateWorkspace(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, err)
		return
	}
	ws, err := h.ws.Create(r.Context(), req.Name, req.Description, userID)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, ws)
}

func (h *RBACHandler) handleGetWorkspace(w http.ResponseWriter, r *http.Request) {
	ws, err := h.ws.Get(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, http.StatusNotFound, err)
		return
	}
	writeJSON(w, http.StatusOK, ws)
}

func (h *RBACHandler) handleUpdateWorkspace(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		MFARequired *bool  `json:"mfa_required"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, err)
		return
	}
	ws, err := h.ws.Update(r.Context(), chi.URLParam(r, "id"), userID, req.Name, req.Description, req.MFARequired)
	if err != nil {
		writeError(w, r, http.StatusForbidden, err)
		return
	}
	writeJSON(w, http.StatusOK, ws)
}

func (h *RBACHandler) handleDeleteWorkspace(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	if err := h.ws.Delete(r.Context(), chi.URLParam(r, "id"), userID); err != nil {
		writeError(w, r, http.StatusForbidden, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *RBACHandler) handleListMembers(w http.ResponseWriter, r *http.Request) {
	list, err := h.ws.ListMembers(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
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
		writeError(w, r, http.StatusBadRequest, err)
		return
	}
	if err := h.ws.AddMember(r.Context(), chi.URLParam(r, "id"), req.UserID, req.RoleID, userID); err != nil {
		writeError(w, r, http.StatusForbidden, err)
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
		writeError(w, r, http.StatusBadRequest, err)
		return
	}
	if err := h.ws.UpdateMemberRole(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "userId"), req.RoleID, userID); err != nil {
		writeError(w, r, http.StatusForbidden, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *RBACHandler) handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	if err := h.ws.RemoveMember(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "userId"), userID); err != nil {
		writeError(w, r, http.StatusForbidden, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *RBACHandler) handleListWorkspaceDatasources(w http.ResponseWriter, r *http.Request) {
	list, err := h.ws.ListDatasources(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
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
		writeError(w, r, http.StatusBadRequest, err)
		return
	}
	if req.AccessLevel == "" {
		req.AccessLevel = "read"
	}
	if err := h.ws.AttachDatasource(r.Context(), chi.URLParam(r, "id"), req.DatasourceID, req.AccessLevel, userID); err != nil {
		writeError(w, r, http.StatusForbidden, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (h *RBACHandler) handleDetachDatasource(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	if err := h.ws.DetachDatasource(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "dsId"), userID); err != nil {
		writeError(w, r, http.StatusForbidden, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// === Datasource access ===

func (h *RBACHandler) handleListMyDatasources(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	ids, err := h.dsAccess.ListAccessibleDatasourceIDs(r.Context(), userID)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
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

func (h *RBACHandler) handleRequestAccess(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	dsID := chi.URLParam(r, "id")
	action := "datasource.request_access"
	resType := "datasource"
	err := h.audit.Log(r.Context(), &userID, action, &resType, &dsID, nil, nil)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

// === Sharing ===

func (h *RBACHandler) handleCreateShare(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	var req ShareRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, err)
		return
	}
	share, err := h.sharing.Share(r.Context(), userID, req)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, share)
}

func (h *RBACHandler) handleListShares(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	resourceType := r.URL.Query().Get("resource_type")
	mode := r.URL.Query().Get("mode")

	var list []ResourceShare
	var err error
	if mode == "owned" {
		list, err = h.sharing.ListOwned(r.Context(), userID, resourceType, r.URL.Query().Get("resource_id"))
	} else {
		list, err = h.sharing.ListShared(r.Context(), userID, resourceType)
	}
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}

	paginated, total := paginateSlice(r, list)
	writeJSON(w, http.StatusOK, map[string]any{
		"shares": paginated,
		"total":  total,
	})
}

func (h *RBACHandler) handleRevokeShare(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(userIDKey).(string)
	if err := h.sharing.Revoke(r.Context(), chi.URLParam(r, "id"), userID); err != nil {
		writeError(w, r, http.StatusForbidden, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// === Admin ===

func (h *RBACHandler) handleAdminListAccess(w http.ResponseWriter, r *http.Request) {
	list, err := h.dsAccess.ListAll(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	paginated, total := paginateSlice(r, list)
	writeJSON(w, http.StatusOK, map[string]any{
		"access": paginated,
		"total":  total,
	})
}

func (h *RBACHandler) handleAdminGrantAccess(w http.ResponseWriter, r *http.Request) {
	caller := r.Context().Value(userIDKey).(string)
	var req struct {
		UserID       string `json:"user_id"`
		DatasourceID string `json:"datasource_id"`
		AccessLevel  string `json:"access_level"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, err)
		return
	}
	if req.AccessLevel == "" {
		req.AccessLevel = "read"
	}
	access, err := h.dsAccess.Grant(r.Context(), req.UserID, req.DatasourceID, req.AccessLevel, caller)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusCreated, access)
}

func (h *RBACHandler) handleAdminUpdateAccess(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AccessLevel string `json:"access_level"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, err)
		return
	}
	if err := h.dsAccess.UpdateLevel(r.Context(), chi.URLParam(r, "id"), req.AccessLevel); err != nil {
		writeError(w, r, http.StatusBadRequest, err)
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
		writeError(w, r, http.StatusBadRequest, err)
		return
	}
	if err := h.dsAccess.Revoke(r.Context(), req.UserID, req.DatasourceID); err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *RBACHandler) handleAdminListRoles(w http.ResponseWriter, r *http.Request) {
	roles, err := h.rbacRepo.ListRoles(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	paginated, total := paginateSlice(r, roles)
	writeJSON(w, http.StatusOK, map[string]any{
		"roles": paginated,
		"total": total,
	})
}

func (h *RBACHandler) handleAdminListPermissions(w http.ResponseWriter, r *http.Request) {
	perms, err := h.rbacRepo.ListPermissions(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	paginated, total := paginateSlice(r, perms)
	writeJSON(w, http.StatusOK, map[string]any{
		"permissions": paginated,
		"total":       total,
	})
}

func (h *RBACHandler) handleAdminAssignRole(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	caller, _ := r.Context().Value(userIDKey).(string)
	if err := h.rbacRepo.EnforceSelfModificationGuard(r.Context(), caller, userID, "role.assign"); err != nil {
		h.auditSoD(r, caller, "role.assign")
		writeError(w, r, http.StatusForbidden, err)
		return
	}
	var req struct {
		RoleID    string  `json:"role_id"`
		ScopeType *string `json:"scope_type,omitempty"`
		ScopeID   *string `json:"scope_id,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, err)
		return
	}
	if err := h.rbacRepo.AssignRole(r.Context(), userID, req.RoleID, req.ScopeType, req.ScopeID); err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	if h.audit != nil {
		resType := "user_role"
		_ = h.audit.Log(r.Context(), &caller, AuditRoleAssigned, &resType, &userID,
			map[string]any{"role_id": req.RoleID}, nil)
	}
	w.WriteHeader(http.StatusCreated)
}

func (h *RBACHandler) auditSoD(r *http.Request, caller, action string) {
	if h.audit == nil {
		return
	}
	resType := "user"
	_ = h.audit.Log(r.Context(), &caller, AuditAdminBlockSod, &resType, &caller,
		map[string]any{"blocked_action": action}, nil)
}

func (h *RBACHandler) handleAdminListAuditLog(w http.ResponseWriter, r *http.Request) {
	filter, err := auditFilterFromQuery(r)
	if err != nil {
		writeError(w, r, http.StatusBadRequest, err)
		return
	}
	entries, err := h.audit.List(r.Context(), filter)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	total, err := h.audit.Count(r.Context(), filter)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}

	caller, _ := r.Context().Value(userIDKey).(string)
	exportAction := AuditAuditExport
	resType := "audit_log"
	_ = h.audit.Log(r.Context(), &caller, exportAction, &resType, nil,
		map[string]any{"format": "json", "count": len(entries)}, nil)

	switch r.URL.Query().Get("format") {
	case "csv":
		allFilter := filter
		allFilter.Limit = 10000
		allFilter.Offset = 0
		allEntries, err := h.audit.List(r.Context(), allFilter)
		if err != nil {
			writeError(w, r, http.StatusInternalServerError, err)
			return
		}
		writeAuditCSV(w, allEntries)
	default:
		writeJSON(w, http.StatusOK, map[string]any{"entries": entries, "total": total})
	}
}

func auditFilterFromQuery(r *http.Request) (AuditFilter, error) {
	q := r.URL.Query()

	page := 1
	if v := q.Get("page"); v != "" {
		if n, err := parsePositiveInt(v); err == nil {
			page = n
		}
	}

	pageSize := 10
	if v := q.Get("page_size"); v != "" {
		if n, err := parsePositiveInt(v); err == nil {
			pageSize = n
		}
	} else if v := q.Get("limit"); v != "" {
		if n, err := parsePositiveInt(v); err == nil {
			pageSize = n
		}
	}

	filter := AuditFilter{
		UserID: q.Get("user_id"),
		Action: q.Get("action"),
		Limit:  pageSize,
		Offset: (page - 1) * pageSize,
	}
	if v := q.Get("from"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return filter, errors.New("from must be RFC3339")
		}
		filter.From = &t
	}
	if v := q.Get("to"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return filter, errors.New("to must be RFC3339")
		}
		filter.To = &t
	}
	return filter, nil
}

func writeAuditCSV(w http.ResponseWriter, entries []AuditEntry) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=audit_log.csv")
	cw := csv.NewWriter(w)
	defer cw.Flush()
	_ = cw.Write([]string{"id", "created_at", "user_id", "action", "resource", "resource_id", "ip_address", "metadata"})
	for _, e := range entries {
		row := []string{
			e.ID,
			e.CreatedAt.UTC().Format(time.RFC3339Nano),
			strOrEmpty(e.UserID),
			e.Action,
			strOrEmpty(e.Resource),
			strOrEmpty(e.ResourceID),
			strOrEmpty(e.IPAddress),
			string(e.Metadata),
		}
		_ = cw.Write(row)
	}
}

func strOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func parsePositiveInt(s string) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, err
	}
	if n <= 0 {
		return 0, errors.New("non-positive")
	}
	if n > 100000 {
		return 0, errors.New("value too large")
	}
	return n, nil
}

func (h *RBACHandler) handleAdminRemoveRole(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	roleID := chi.URLParam(r, "roleId")
	caller, _ := r.Context().Value(userIDKey).(string)
	if err := h.rbacRepo.EnforceSelfModificationGuard(r.Context(), caller, userID, "role.remove"); err != nil {
		h.auditSoD(r, caller, "role.remove")
		writeError(w, r, http.StatusForbidden, err)
		return
	}
	if err := h.rbacRepo.RemoveRole(r.Context(), userID, roleID); err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	if h.audit != nil {
		resType := "user_role"
		_ = h.audit.Log(r.Context(), &caller, AuditRoleRemoved, &resType, &userID,
			map[string]any{"role_id": roleID}, nil)
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
		writeError(w, r, http.StatusBadRequest, err)
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
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"allowed": allowed})
}

func (h *RBACHandler) handleInternalUserDatasources(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	ids, err := h.dsAccess.ListAccessibleDatasourceIDs(r.Context(), userID)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
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
		writeError(w, r, http.StatusBadRequest, err)
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
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *RBACHandler) handleInternalWorkspaceDatasources(w http.ResponseWriter, r *http.Request) {
	list, err := h.ws.ListDatasources(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	ids := make([]string, 0, len(list))
	for _, wd := range list {
		ids = append(ids, wd.DatasourceID)
	}
	writeJSON(w, http.StatusOK, map[string]any{"datasource_ids": ids})
}

func (h *RBACHandler) handleInternalInvalidateCache(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID string `json:"user_id"`
		Scope  string `json:"scope"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, err)
		return
	}
	_ = h.dsAccess.InvalidateCache(r.Context(), req.UserID)
	w.WriteHeader(http.StatusNoContent)
}

func (h *RBACHandler) handleInternalPublicKey(w http.ResponseWriter, r *http.Request) {
	pem, err := h.jwtMgr.GetPublicKeyPEM()
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"public_key": pem,
		"issuer":     h.jwtMgr.Issuer(),
		"audience":   h.jwtMgr.Audience(),
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, err error) {
	if status >= http.StatusInternalServerError {
		ctx := r.Context()
		var userID string
		if uVal := ctx.Value(userIDKey); uVal != nil {
			userID, _ = uVal.(string)
		}
		allArgs := []any{"error", err}
		if reqID := requestid.FromContext(ctx); reqID != "" {
			allArgs = append(allArgs, "request_id", reqID)
		}
		if userID != "" {
			allArgs = append(allArgs, "user_id", userID)
		}
		slog.ErrorContext(ctx, "rbac handler internal error", allArgs...)
		writeJSON(w, status, map[string]string{"error": "internal server error"})
	} else {
		writeJSON(w, status, map[string]string{"error": err.Error()})
	}
}

func (h *RBACHandler) handleAdminListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.userRepo.ListUsers(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}

	search := strings.ToLower(r.URL.Query().Get("search"))
	status := r.URL.Query().Get("status")

	var filtered []UserResponse
	for _, u := range users {
		matchesSearch := true
		if search != "" {
			emailMatch := strings.Contains(strings.ToLower(u.Email), search)
			usernameMatch := u.Username != nil && strings.Contains(strings.ToLower(*u.Username), search)
			displayNameMatch := u.DisplayName != nil && strings.Contains(strings.ToLower(*u.DisplayName), search)
			matchesSearch = emailMatch || usernameMatch || displayNameMatch
		}

		matchesStatus := true
		switch status {
		case "active":
			matchesStatus = u.IsActive
		case "inactive":
			matchesStatus = !u.IsActive
		}

		if matchesSearch && matchesStatus {
			filtered = append(filtered, UserResponse{
				ID:            u.ID,
				Email:         u.Email,
				Username:      u.Username,
				DisplayName:   u.DisplayName,
				AvatarURL:     u.AvatarURL,
				IsActive:      u.IsActive,
				EmailVerified: u.EmailVerified,
				CreatedAt:     u.CreatedAt,
				UpdatedAt:     u.UpdatedAt,
			})
		}
	}

	paginated, total := paginateSlice(r, filtered)
	writeJSON(w, http.StatusOK, map[string]any{
		"users": paginated,
		"total": total,
	})
}

func (h *RBACHandler) handleAdminGetUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	u, err := h.userRepo.GetUserByID(r.Context(), id)
	if errors.Is(err, ErrUserNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	} else if err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	resp := UserResponse{
		ID:            u.ID,
		Email:         u.Email,
		Username:      u.Username,
		DisplayName:   u.DisplayName,
		AvatarURL:     u.AvatarURL,
		IsActive:      u.IsActive,
		EmailVerified: u.EmailVerified,
		CreatedAt:     u.CreatedAt,
		UpdatedAt:     u.UpdatedAt,
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h *RBACHandler) handleAdminGetUserRoles(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	roles, err := h.rbacRepo.GetUserRolesWithScope(r.Context(), id)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, roles)
}

func (h *RBACHandler) handleAdminUpdateUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	caller, _ := r.Context().Value(userIDKey).(string)
	var req struct {
		IsActive bool `json:"is_active"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, r, http.StatusBadRequest, err)
		return
	}
	if !req.IsActive {
		if err := h.rbacRepo.EnforceSelfModificationGuard(r.Context(), caller, id, "user.deactivate"); err != nil {
			h.auditSoD(r, caller, "user.deactivate")
			writeError(w, r, http.StatusForbidden, err)
			return
		}
	}
	if err := h.userRepo.UpdateUserActiveStatus(r.Context(), id, req.IsActive); err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func paginateSlice[T any](r *http.Request, items []T) (paginated []T, total int) {
	total = len(items)
	q := r.URL.Query()
	pageStr := q.Get("page")
	pageSizeStr := q.Get("page_size")

	if pageStr == "" && pageSizeStr == "" {
		return items, total
	}

	page := 1
	if pageStr != "" {
		if p, err := parsePositiveInt(pageStr); err == nil {
			page = p
		}
	}

	pageSize := 10
	if pageSizeStr != "" {
		if ps, err := parsePositiveInt(pageSizeStr); err == nil {
			pageSize = ps
		}
	}

	start := (page - 1) * pageSize
	end := start + pageSize

	if start > total {
		return []T{}, total
	}
	if end > total {
		end = total
	}

	return items[start:end], total
}
