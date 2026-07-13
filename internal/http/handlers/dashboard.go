package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"database/sql"

	"github.com/biqly/biqly/internal/dashboard"
	bimw "github.com/biqly/biqly/internal/http/middleware"
)

type createDashboardRequest struct {
	Name        string          `json:"name"`
	Description *string         `json:"description,omitempty"`
	Widgets     json.RawMessage `json:"widgets"`
}

type updateDashboardRequest struct {
	Name        string          `json:"name"`
	Description *string         `json:"description,omitempty"`
	Widgets     json.RawMessage `json:"widgets"`
}

// DashboardHandler handles dashboard API requests.
type DashboardHandler struct {
	repo *dashboard.Repository
}

// NewDashboardHandler creates a new DashboardHandler.
func NewDashboardHandler(repo *dashboard.Repository) *DashboardHandler {
	return &DashboardHandler{repo: repo}
}

// dashboardScope resolves the workspace filter to apply to a dashboard request
// and whether the caller is authorized to proceed. The dashboard repository
// treats an empty workspace filter as "unscoped" (all workspaces + global
// dashboards); that is only legitimate for a super admin acting without an
// active workspace. A regular caller with no active workspace must NOT receive
// that cross-workspace view, so it returns ok=false — callers translate that to
// an empty result (list) or not-found (single-resource) rather than granting
// full access. All other cases preserve the prior behavior: a workspace-scoped
// caller (super admin or not) is scoped to that workspace.
func dashboardScope(ctx context.Context) (workspaceID string, ok bool) {
	wsID := bimw.WorkspaceID(ctx)
	if wsID == "" && !bimw.HasRole(ctx, bimw.RoleSuperAdmin) {
		return "", false
	}
	return wsID, true
}

// Create handles POST /api/dashboards
func (h *DashboardHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	input, ok := decodeJSON[createDashboardRequest](w, r)
	if !ok {
		return
	}
	if input.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	widgets := input.Widgets
	if len(widgets) == 0 {
		widgets = json.RawMessage("[]")
	}

	scope, ok := dashboardScope(ctx)
	if !ok {
		writeError(w, http.StatusForbidden, "an active workspace is required")
		return
	}
	var wsID *string
	if scope != "" {
		wsID = &scope
	}

	d := &dashboard.Dashboard{
		WorkspaceID: wsID,
		Name:        input.Name,
		Description: input.Description,
		Widgets:     widgets,
	}

	if err := h.repo.Create(ctx, d); err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to create dashboard", err)
		return
	}

	writeJSON(w, http.StatusCreated, d)
}

// List handles GET /api/dashboards
func (h *DashboardHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	wsID, ok := dashboardScope(ctx)
	if !ok {
		// Regular caller with no active workspace: no cross-workspace view.
		writeJSON(w, http.StatusOK, []dashboard.Dashboard{})
		return
	}

	dashboards, err := h.repo.List(ctx, wsID)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to list dashboards", err)
		return
	}

	writeJSON(w, http.StatusOK, dashboards)
}

// Get handles GET /api/dashboards/{id}
func (h *DashboardHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}

	wsID, ok := dashboardScope(ctx)
	if !ok {
		writeEntityNotFound(w, "dashboard")
		return
	}
	d, err := h.repo.Get(ctx, id, wsID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeEntityNotFound(w, "dashboard")
			return
		}
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to get dashboard", err)
		return
	}

	writeJSON(w, http.StatusOK, d)
}

// Update handles PUT /api/dashboards/{id}
func (h *DashboardHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	input, ok := decodeJSON[updateDashboardRequest](w, r)
	if !ok {
		return
	}
	if input.Name == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	widgets := input.Widgets
	if len(widgets) == 0 {
		widgets = json.RawMessage("[]")
	}

	wsID, ok := dashboardScope(ctx)
	if !ok {
		writeEntityNotFound(w, "dashboard")
		return
	}

	d := &dashboard.Dashboard{
		ID:          id,
		Name:        input.Name,
		Description: input.Description,
		Widgets:     widgets,
	}

	if err := h.repo.Update(ctx, d, wsID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeEntityNotFound(w, "dashboard")
			return
		}
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to update dashboard", err)
		return
	}

	writeJSON(w, http.StatusOK, d)
}

// Delete handles DELETE /api/dashboards/{id}
func (h *DashboardHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}

	wsID, ok := dashboardScope(ctx)
	if !ok {
		writeEntityNotFound(w, "dashboard")
		return
	}

	if err := h.repo.Delete(ctx, id, wsID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeEntityNotFound(w, "dashboard")
			return
		}
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to delete dashboard", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
