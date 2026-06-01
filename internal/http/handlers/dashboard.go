package handlers

import (
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

	var wsID *string
	if val := bimw.WorkspaceID(ctx); val != "" {
		wsID = &val
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
	wsID := bimw.WorkspaceID(ctx)

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

	d, err := h.repo.Get(ctx, id)
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

	d := &dashboard.Dashboard{
		ID:          id,
		Name:        input.Name,
		Description: input.Description,
		Widgets:     widgets,
	}

	if err := h.repo.Update(ctx, d); err != nil {
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

	if err := h.repo.Delete(ctx, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeEntityNotFound(w, "dashboard")
			return
		}
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to delete dashboard", err)
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}
