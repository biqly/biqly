package handlers

import (
	"database/sql"
	"encoding/csv"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"log/slog"

	"github.com/biqly/biqly/internal/auth"
	"github.com/biqly/biqly/internal/auth/rbac"
	"github.com/biqly/biqly/internal/auth/workspace"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/biqly/biqly/internal/http/response"
)

type RBACHandler struct {
	rbac          *rbac.Service
	rbacRepo      *rbac.RBACRepository
	userRepo      *auth.UserRepository
	dsAccess      *rbac.DatasourceAccessService
	aiModelAccess *rbac.AIModelAccessService
	ws            *workspace.Service
	sharing       *workspace.SharingService
	audit         *auth.AuditService
	jwtMgr        *auth.JWTManager
	cfg           *auth.Config
}

func NewRBACHandler(
	rbacSvc *rbac.Service,
	rbacRepo *rbac.RBACRepository,
	userRepo *auth.UserRepository,
	dsAccess *rbac.DatasourceAccessService,
	aiModelAccess *rbac.AIModelAccessService,
	ws *workspace.Service,
	sharing *workspace.SharingService,
	audit *auth.AuditService,
	jwtMgr *auth.JWTManager,
	cfg *auth.Config,
) *RBACHandler {
	return &RBACHandler{
		rbac:          rbacSvc,
		rbacRepo:      rbacRepo,
		userRepo:      userRepo,
		dsAccess:      dsAccess,
		aiModelAccess: aiModelAccess,
		ws:            ws,
		sharing:       sharing,
		audit:         audit,
		jwtMgr:        jwtMgr,
		cfg:           cfg,
	}
}

// requirePermission gates a route on the caller holding at least one of the
// given global permissions. Super admins always pass (handled inside
// RBACService.RequireAny). Used for platform-wide admin endpoints.
func (h *RBACHandler) requirePermission(perms ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := contextUserID(r)
			if !ok {
				writeError(w, r, http.StatusUnauthorized, errors.New("authentication required"))
				return
			}
			ok, err := h.rbac.RequireAny(r.Context(), userID, perms...)
			if err != nil {
				writeError(w, r, http.StatusInternalServerError, err)
				return
			}
			if !ok {
				writeError(w, r, http.StatusForbidden, errors.New("insufficient permissions"))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// requireWorkspacePermission gates a route on the caller holding at least one
// of the given permissions, evaluated with the workspace scope taken from the
// {id} URL param (so a global grant OR a workspace-scoped grant both satisfy
// it). Super admins always pass (handled inside RBACService.Check).
func (h *RBACHandler) requireWorkspacePermission(perms ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userID, ok := contextUserID(r)
			if !ok {
				writeError(w, r, http.StatusUnauthorized, errors.New("authentication required"))
				return
			}
			wsID := chi.URLParam(r, "id")
			for _, p := range perms {
				ok, err := h.rbac.Check(r.Context(), rbac.PermissionCheck{
					UserID:     userID,
					Permission: p,
					ScopeType:  rbac.ScopeWorkspace,
					ScopeID:    wsID,
				})
				if err != nil {
					writeError(w, r, http.StatusInternalServerError, err)
					return
				}
				if ok {
					next.ServeHTTP(w, r)
					return
				}
			}
			writeError(w, r, http.StatusForbidden, errors.New("insufficient permissions"))
		})
	}
}

func (h *RBACHandler) RegisterAuthRoutes(r chi.Router, authMW func(http.Handler) http.Handler) {
	r.Group(func(r chi.Router) {
		r.Use(authMW)

		slicePagination := bimw.Paginate(bimw.PaginationConfig{DefaultPage: 1, DefaultPageSize: 10, MaxPageSize: 100000})

		r.With(slicePagination).Get("/workspaces", h.handleListWorkspaces)
		r.With(h.requirePermission("workspace:create")).Post("/workspaces", h.handleCreateWorkspace)
		r.Get("/workspaces/{id}", h.handleGetWorkspace)
		r.With(h.requireWorkspacePermission("admin:workspaces")).Put("/workspaces/{id}", h.handleUpdateWorkspace)
		r.With(h.requireWorkspacePermission("admin:workspaces")).Delete("/workspaces/{id}", h.handleDeleteWorkspace)

		r.With(slicePagination).Get("/workspaces/{id}/members", h.handleListMembers)
		r.With(h.requireWorkspacePermission("workspace:invite", "admin:workspaces")).Post("/workspaces/{id}/members", h.handleAddMember)
		r.With(h.requireWorkspacePermission("workspace:invite", "admin:workspaces")).Put("/workspaces/{id}/members/{userId}", h.handleUpdateMemberRole)
		r.With(h.requireWorkspacePermission("workspace:invite", "admin:workspaces")).Delete("/workspaces/{id}/members/{userId}", h.handleRemoveMember)

		r.With(slicePagination).Get("/workspaces/{id}/datasources", h.handleListWorkspaceDatasources)
		r.With(h.requireWorkspacePermission("workspace:manage_datasources", "admin:workspaces")).Post("/workspaces/{id}/datasources", h.handleAttachDatasource)
		r.With(h.requireWorkspacePermission("workspace:manage_datasources", "admin:workspaces")).Delete("/workspaces/{id}/datasources/{dsId}", h.handleDetachDatasource)

		r.Get("/me/permissions", h.handleMyPermissions)
		r.With(slicePagination).Get("/me/datasources", h.handleListMyDatasources)
		r.Get("/me/datasources/{id}/check", h.handleCheckMyDatasource)
		r.Post("/me/datasources/{id}/request-access", h.handleRequestAccess)

		r.Post("/shares", h.handleCreateShare)
		r.With(slicePagination).Get("/shares", h.handleListShares)
		r.Delete("/shares/{id}", h.handleRevokeShare)

		h.registerAIModelAccessUserRoutes(r)

		r.Route("/admin", func(r chi.Router) {
			r.Group(func(r chi.Router) {
				r.Use(h.requirePermission("datasource:grant_access"))
				r.With(slicePagination).Get("/datasource-access", h.handleAdminListAccess)
				r.Post("/datasource-access", h.handleAdminGrantAccess)
				r.Put("/datasource-access/{id}", h.handleAdminUpdateAccess)
				r.Delete("/datasource-access/{id}", h.handleAdminRevokeAccess)
			})

			r.Group(func(r chi.Router) {
				r.Use(h.requirePermission("admin:roles"))
				r.With(slicePagination).Get("/roles", h.handleAdminListRoles)
				r.Get("/roles/{roleId}/permissions", h.handleAdminGetRolePermissions)
				r.Put("/roles/{roleId}/permissions", h.handleAdminSetRolePermissions)
				r.With(slicePagination).Get("/permissions", h.handleAdminListPermissions)
				r.Post("/users/{id}/roles", h.handleAdminAssignRole)
				r.Delete("/users/{id}/roles/{roleId}", h.handleAdminRemoveRole)
			})

			r.Group(func(r chi.Router) {
				r.Use(h.requirePermission("admin:users"))
				r.With(slicePagination).Get("/users", h.handleAdminListUsers)
				r.Get("/users/{id}", h.handleAdminGetUser)
				r.Get("/users/{id}/roles", h.handleAdminGetUserRoles)
				r.Put("/users/{id}", h.handleAdminUpdateUser)
			})

			auditPagination := bimw.Paginate(bimw.PaginationConfig{DefaultPage: 1, DefaultPageSize: 10, MaxPageSize: 100000})
			r.With(h.requirePermission("admin:audit"), auditPagination).Get("/audit-log", h.handleAdminListAuditLog)

			r.Group(func(r chi.Router) {
				r.Use(h.requirePermission("admin:settings", "admin:roles"))
				h.registerAIModelAccessAdminRoutes(r)
			})
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
		r.Get("/user/{id}", h.handleInternalGetUser)
		h.registerAIModelAccessInternalRoutes(r)
	})
}

func requireContextUserID(w http.ResponseWriter, r *http.Request) (string, bool) {
	userID, ok := contextUserID(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, errors.New("unauthorized"))
		return "", false
	}
	return userID, true
}

// handleMyPermissions returns the caller's effective global permissions plus a
// super-admin flag, so the UI can disable controls the user is not allowed to
// use. This is a convenience mirror of the server-side checks — never the sole
// gate; every mutating endpoint is still enforced on the backend.
func (h *RBACHandler) handleMyPermissions(w http.ResponseWriter, r *http.Request) {
	userID, ok := contextUserID(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, errors.New("authentication required"))
		return
	}
	isSuper, err := h.rbac.IsSuperAdmin(r.Context(), userID)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	perms, err := h.rbac.EffectivePermissions(r.Context(), userID, "")
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	if perms == nil {
		perms = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"permissions":    perms,
		"is_super_admin": isSuper,
	})
}

// === Workspace ===

func (h *RBACHandler) handleListWorkspaces(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireContextUserID(w, r)
	if !ok {
		return
	}
	isSuper, err := h.rbac.IsSuperAdmin(r.Context(), userID)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	var list []workspace.Workspace
	if isSuper {
		list, err = h.ws.ListAll(r.Context())
	} else {
		list, err = h.ws.ListForUser(r.Context(), userID)
	}
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
	userID, ok := requireContextUserID(w, r)
	if !ok {
		return
	}
	req, ok := decodeJSON[struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}](w, r)
	if !ok {
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
	userID, ok := requireContextUserID(w, r)
	if !ok {
		return
	}
	req, ok := decodeJSON[struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		MFARequired *bool  `json:"mfa_required"`
	}](w, r)
	if !ok {
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
	userID, ok := requireContextUserID(w, r)
	if !ok {
		return
	}
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
	userID, ok := requireContextUserID(w, r)
	if !ok {
		return
	}
	req, ok := decodeJSON[struct {
		UserID string `json:"user_id"`
		RoleID string `json:"role_id"`
	}](w, r)
	if !ok {
		return
	}
	if err := h.ws.AddMember(r.Context(), chi.URLParam(r, "id"), req.UserID, req.RoleID, userID); err != nil {
		writeError(w, r, http.StatusForbidden, err)
		return
	}
	w.WriteHeader(http.StatusCreated)
}

func (h *RBACHandler) handleUpdateMemberRole(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireContextUserID(w, r)
	if !ok {
		return
	}
	req, ok := decodeJSON[struct {
		RoleID string `json:"role_id"`
	}](w, r)
	if !ok {
		return
	}
	if err := h.ws.UpdateMemberRole(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "userId"), req.RoleID, userID); err != nil {
		writeError(w, r, http.StatusForbidden, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *RBACHandler) handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireContextUserID(w, r)
	if !ok {
		return
	}
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
	userID, ok := requireContextUserID(w, r)
	if !ok {
		return
	}
	req, ok := decodeJSON[struct {
		DatasourceID string `json:"datasource_id"`
		AccessLevel  string `json:"access_level"`
	}](w, r)
	if !ok {
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
	userID, ok := requireContextUserID(w, r)
	if !ok {
		return
	}
	if err := h.ws.DetachDatasource(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "dsId"), userID); err != nil {
		writeError(w, r, http.StatusForbidden, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// === Datasource access ===

func (h *RBACHandler) handleListMyDatasources(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireContextUserID(w, r)
	if !ok {
		return
	}
	ids, err := h.dsAccess.ListAccessibleDatasourceIDs(r.Context(), userID)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"datasource_ids": ids})
}

func (h *RBACHandler) handleCheckMyDatasource(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireContextUserID(w, r)
	if !ok {
		return
	}
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
	userID, ok := requireContextUserID(w, r)
	if !ok {
		return
	}
	err := h.audit.Log(r.Context(), &userID, "datasource.request_access", new("datasource"), new(chi.URLParam(r, "id")), nil, nil)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

// === Sharing ===

func (h *RBACHandler) handleCreateShare(w http.ResponseWriter, r *http.Request) {
	userID, ok := requireContextUserID(w, r)
	if !ok {
		return
	}
	req, ok := decodeJSON[workspace.ShareRequest](w, r)
	if !ok {
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
	userID, ok := requireContextUserID(w, r)
	if !ok {
		return
	}
	resourceType := r.URL.Query().Get("resource_type")
	mode := r.URL.Query().Get("mode")

	var list []workspace.ResourceShare
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
	userID, ok := requireContextUserID(w, r)
	if !ok {
		return
	}
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
	caller, ok := contextUserID(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, errors.New("unauthorized"))
		return
	}
	req, ok := decodeJSON[struct {
		UserID       string `json:"user_id"`
		DatasourceID string `json:"datasource_id"`
		AccessLevel  string `json:"access_level"`
	}](w, r)
	if !ok {
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
	caller, ok := contextUserID(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, errors.New("unauthorized"))
		return
	}
	req, ok := decodeJSON[struct {
		AccessLevel string `json:"access_level"`
	}](w, r)
	if !ok {
		return
	}

	accessID := chi.URLParam(r, "id")

	// Look up the existing grant to verify caller has permission on its datasource.
	access, err := h.dsAccess.GetByID(r.Context(), accessID)
	if errors.Is(err, sql.ErrNoRows) {
		writeError(w, r, http.StatusNotFound, errors.New("access grant not found"))
		return
	}
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}

	ok, err = h.rbac.Check(r.Context(), rbac.PermissionCheck{
		UserID:     caller,
		Permission: "datasource:grant_access",
		ScopeType:  rbac.ScopeDatasource,
		ScopeID:    access.DatasourceID,
	})
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	if !ok {
		writeError(w, r, http.StatusForbidden, errors.New("insufficient permissions"))
		return
	}

	if err := h.dsAccess.UpdateLevel(r.Context(), accessID, req.AccessLevel); err != nil {
		writeError(w, r, http.StatusBadRequest, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *RBACHandler) handleAdminRevokeAccess(w http.ResponseWriter, r *http.Request) {
	req, ok := decodeJSON[struct {
		UserID       string `json:"user_id"`
		DatasourceID string `json:"datasource_id"`
	}](w, r)
	if !ok {
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

func (h *RBACHandler) handleAdminGetRolePermissions(w http.ResponseWriter, r *http.Request) {
	roleID := chi.URLParam(r, "roleId")
	ids, err := h.rbacRepo.GetRolePermissionIDs(r.Context(), roleID)
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"permission_ids": ids})
}

func (h *RBACHandler) handleAdminSetRolePermissions(w http.ResponseWriter, r *http.Request) {
	roleID := chi.URLParam(r, "roleId")
	req, ok := decodeJSON[struct {
		PermissionIDs []string `json:"permission_ids"`
	}](w, r)
	if !ok {
		return
	}
	if err := h.rbacRepo.SetRolePermissions(r.Context(), roleID, req.PermissionIDs); err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	h.rbac.InvalidateAllCache()
	w.WriteHeader(http.StatusNoContent)
}

func (h *RBACHandler) handleAdminAssignRole(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	caller, ok := contextUserID(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, errors.New("authentication required"))
		return
	}
	if err := h.rbacRepo.EnforceSelfModificationGuard(r.Context(), caller, userID, "role.assign"); err != nil {
		h.auditSoD(r, caller, "role.assign")
		writeError(w, r, http.StatusForbidden, err)
		return
	}
	req, ok := decodeJSON[struct {
		RoleID    string  `json:"role_id"`
		ScopeType *string `json:"scope_type,omitempty"`
		ScopeID   *string `json:"scope_id,omitempty"`
	}](w, r)
	if !ok {
		return
	}
	if err := h.rbacRepo.EnforcePrivilegedRoleAssignmentGuard(r.Context(), caller, req.RoleID); err != nil {
		h.auditSoD(r, caller, "role.assign")
		writeError(w, r, http.StatusForbidden, err)
		return
	}
	if err := h.rbacRepo.AssignRole(r.Context(), userID, req.RoleID, req.ScopeType, req.ScopeID); err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	h.rbac.InvalidateUserCache(userID)
	if h.audit != nil {
		if err := h.audit.Log(r.Context(), &caller, auth.AuditRoleAssigned, new("user_role"), &userID,
			map[string]any{"role_id": req.RoleID}, nil); err != nil {
			slog.WarnContext(r.Context(), "auth audit log failed", "action", auth.AuditRoleAssigned, "error", err)
		}
	}
	w.WriteHeader(http.StatusCreated)
}

func (h *RBACHandler) auditSoD(r *http.Request, caller, action string) {
	if h.audit == nil {
		return
	}
	if err := h.audit.Log(r.Context(), &caller, auth.AuditAdminBlockSod, new("user"), &caller,
		map[string]any{"blocked_action": action}, nil); err != nil {
		slog.WarnContext(r.Context(), "auth audit log failed", "action", auth.AuditAdminBlockSod, "error", err)
	}
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

	caller, ok := contextUserID(r)
	if !ok {
		writeError(w, r, http.StatusUnauthorized, errors.New("authentication required"))
		return
	}
	if err := h.audit.Log(r.Context(), &caller, auth.AuditAuditExport, new("audit_log"), nil,
		map[string]any{"format": "json", "count": len(entries)}, nil); err != nil {
		slog.Warn("audit export event failed", "error", err)
	}

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

func auditFilterFromQuery(r *http.Request) (auth.AuditFilter, error) {
	q := r.URL.Query()
	pagination := bimw.PaginationFromContext(r.Context())
	if pagination.PageSize <= 0 {
		pagination.PageSize = 10
	}

	filter := auth.AuditFilter{
		UserID: q.Get("user_id"),
		Action: q.Get("action"),
		Limit:  pagination.PageSize,
		Offset: pagination.Offset,
	}
	if v := q.Get("from"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return filter, errors.New("from must be RFC3339")
		}
		filter.From = new(t)
	}
	if v := q.Get("to"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		if err != nil {
			return filter, errors.New("to must be RFC3339")
		}
		filter.To = new(t)
	}
	return filter, nil
}

func writeAuditCSV(w http.ResponseWriter, entries []auth.AuditEntry) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename=audit_log.csv")
	cw := csv.NewWriter(w)
	defer cw.Flush()
	if err := cw.Write([]string{"id", "created_at", "user_id", "action", "resource", "resource_id", "ip_address", "metadata"}); err != nil {
		return
	}
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
		if err := cw.Write(row); err != nil {
			return
		}
	}
}

func strOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func (h *RBACHandler) handleAdminRemoveRole(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "id")
	roleID := chi.URLParam(r, "roleId")
	caller := bimw.UserID(r.Context())
	if err := h.rbacRepo.EnforceSelfModificationGuard(r.Context(), caller, userID, "role.remove"); err != nil {
		h.auditSoD(r, caller, "role.remove")
		writeError(w, r, http.StatusForbidden, err)
		return
	}
	if err := h.rbacRepo.EnforcePrivilegedRoleAssignmentGuard(r.Context(), caller, roleID); err != nil {
		h.auditSoD(r, caller, "role.remove")
		writeError(w, r, http.StatusForbidden, err)
		return
	}
	if err := h.rbacRepo.RemoveRole(r.Context(), userID, roleID); err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	h.rbac.InvalidateUserCache(userID)
	if h.audit != nil {
		if err := h.audit.Log(r.Context(), &caller, auth.AuditRoleRemoved, new("user_role"), &userID,
			map[string]any{"role_id": roleID}, nil); err != nil {
			slog.Warn("audit role removal failed", "error", err)
		}
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
	start := time.Now()
	req, ok := decodeJSON[CheckPermissionRequest](w, r)
	if !ok {
		return
	}
	scopeID := req.ScopeID
	if scopeID == "" {
		scopeID = req.WorkspaceID
	}
	allowed, err := h.rbac.Check(r.Context(), rbac.PermissionCheck{
		UserID:     req.UserID,
		Permission: req.Permission,
		ScopeType:  rbac.ScopeType(req.ScopeType),
		ScopeID:    scopeID,
	})
	if err != nil {
		auth.MetricPermissionCheckDuration.WithLabelValues("error").Observe(time.Since(start).Seconds())
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	result := "denied"
	if allowed {
		result = "allowed"
	}
	auth.MetricPermissionCheckDuration.WithLabelValues(result).Observe(time.Since(start).Seconds())
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
	req, ok := decodeJSON[CheckDSAccessRequest](w, r)
	if !ok {
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

func (h *RBACHandler) handleInternalGetUser(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	u, err := h.userRepo.GetUserByID(r.Context(), id)
	if errors.Is(err, auth.ErrUserNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	} else if err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":    u.ID,
		"email": u.Email,
	})
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
	req, ok := decodeJSON[struct {
		UserID string `json:"user_id"`
		Scope  string `json:"scope"`
	}](w, r)
	if !ok {
		return
	}
	if err := h.dsAccess.InvalidateCache(r.Context(), req.UserID); err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *RBACHandler) handleInternalPublicKey(w http.ResponseWriter, r *http.Request) {
	pem, err := h.jwtMgr.PublicKeyPEM()
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
	response.WriteJSON(w, status, data)
}

func writeError(w http.ResponseWriter, r *http.Request, status int, err error) {
	if status >= http.StatusInternalServerError {
		ctx := r.Context()
		var args []any
		if userID := bimw.UserID(ctx); userID != "" {
			args = append(args, "user_id", userID)
		}
		response.WriteInternalError(ctx, w, status, "rbac handler internal error", err, args...)
	} else {
		response.WriteError(w, status, err.Error())
	}
}

func (h *RBACHandler) handleAdminListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.userRepo.ListUsersForAdmin(r.Context())
	if err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}

	search := strings.ToLower(r.URL.Query().Get("search"))
	status := r.URL.Query().Get("status")

	var filtered []auth.UserResponse
	for _, row := range users {
		matchesSearch := true
		if search != "" {
			emailMatch := strings.Contains(strings.ToLower(row.Email), search)
			usernameMatch := row.Username != nil && strings.Contains(strings.ToLower(*row.Username), search)
			displayNameMatch := row.DisplayName != nil && strings.Contains(strings.ToLower(*row.DisplayName), search)
			matchesSearch = emailMatch || usernameMatch || displayNameMatch
		}

		matchesStatus := true
		switch status {
		case "active":
			matchesStatus = row.IsActive
		case "inactive":
			matchesStatus = !row.IsActive
		}

		if matchesSearch && matchesStatus {
			hasPassword := row.PasswordHash != nil && *row.PasswordHash != ""
			filtered = append(filtered, auth.UserResponse{
				ID:            row.ID,
				Email:         row.Email,
				Username:      row.Username,
				DisplayName:   row.DisplayName,
				AvatarURL:     row.AvatarURL,
				IsActive:      row.IsActive,
				EmailVerified: row.EmailVerified,
				HasPassword:   hasPassword,
				MFAEnabled:    row.MFAEnabled,
				MFAPending:    row.MFAPending,
				PasskeyCount:  row.PasskeyCount,
				CreatedAt:     row.CreatedAt,
				UpdatedAt:     row.UpdatedAt,
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
	if errors.Is(err, auth.ErrUserNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "user not found"})
		return
	} else if err != nil {
		writeError(w, r, http.StatusInternalServerError, err)
		return
	}
	resp := auth.UserResponse{
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
	caller := bimw.UserID(r.Context())
	req, ok := decodeJSON[struct {
		IsActive bool `json:"is_active"`
	}](w, r)
	if !ok {
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
	pagination := bimw.PaginationFromContext(r.Context())
	if !pagination.Requested {
		return items, total
	}
	if pagination.PageSize <= 0 {
		pagination.PageSize = 10
	}

	start := pagination.Offset
	end := start + pagination.PageSize

	if start > total {
		return []T{}, total
	}
	if end > total {
		end = total
	}

	paginated = items[start:end]
	if paginated == nil {
		paginated = []T{}
	}
	return paginated, total
}
