package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"time"

	"github.com/biqly/biqly/internal/audit"
	"github.com/biqly/biqly/internal/dashboard"
	bimw "github.com/biqly/biqly/internal/http/middleware"
)

// workspaceSharingChecker is the kill-switch lookup; *bimw.AuthClient satisfies it.
type workspaceSharingChecker interface {
	WorkspacePublicSharingEnabled(ctx context.Context, workspaceID string) (bool, error)
}

// DashboardShareHandler manages public share links for dashboards.
type DashboardShareHandler struct {
	shares     *dashboard.ShareRepository
	dashes     *dashboard.Repository
	killSwitch workspaceSharingChecker
	auditLog   *audit.Logger
}

// NewDashboardShareHandler creates a DashboardShareHandler.
func NewDashboardShareHandler(shares *dashboard.ShareRepository, dashes *dashboard.Repository, killSwitch workspaceSharingChecker, auditLog *audit.Logger) *DashboardShareHandler {
	return &DashboardShareHandler{shares: shares, dashes: dashes, killSwitch: killSwitch, auditLog: auditLog}
}

// shareScope authorizes the caller for a dashboard and returns its workspace.
// Public shares require a concrete workspace: global dashboards (NULL
// workspace_id) and unscoped super-admin calls are rejected.
func (h *DashboardShareHandler) shareScope(w http.ResponseWriter, r *http.Request) (dashboardID, workspaceID string, ok bool) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return "", "", false
	}
	wsID, ok := dashboardScope(r.Context())
	if !ok || wsID == "" {
		writeEntityNotFound(w, "dashboard")
		return "", "", false
	}
	d, err := h.dashes.Get(r.Context(), id, wsID)
	if err != nil {
		writeEntityNotFound(w, "dashboard")
		return "", "", false
	}
	if d.WorkspaceID == nil || *d.WorkspaceID == "" {
		writeError(w, http.StatusConflict, "global dashboards cannot be shared publicly")
		return "", "", false
	}
	return id, *d.WorkspaceID, true
}

// Create handles POST /api/dashboards/{id}/public-share (create or rotate).
func (h *DashboardShareHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, wsID, ok := h.shareScope(w, r)
	if !ok {
		return
	}
	enabled, err := h.killSwitch.WorkspacePublicSharingEnabled(ctx, wsID)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to check workspace sharing policy", err)
		return
	}
	if !enabled {
		writeError(w, http.StatusConflict, "public sharing is disabled for this workspace")
		return
	}
	token, err := dashboard.GenerateShareToken()
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to generate share token", err)
		return
	}
	share := &dashboard.PublicShare{
		DashboardID: id,
		WorkspaceID: wsID,
		TokenHash:   dashboard.HashShareToken(token),
		CreatedBy:   bimw.UserID(ctx),
	}
	if err := h.shares.Rotate(ctx, share); err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to create share", err)
		return
	}
	h.auditLog.Log(ctx, audit.Event{
		UserID:    bimw.UserID(ctx),
		EventType: audit.EventDashboardShareCreated,
		Details:   map[string]any{"dashboard_id": id, "share_id": share.ID},
	})
	writeJSON(w, http.StatusCreated, map[string]any{
		"token":      token,
		"url_path":   "/public/dashboard/" + token,
		"created_at": share.CreatedAt.Format(time.RFC3339),
	})
}

// Status handles GET /api/dashboards/{id}/public-share.
func (h *DashboardShareHandler) Status(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, wsID, ok := h.shareScope(w, r)
	if !ok {
		return
	}
	share, err := h.shares.GetActive(ctx, id, wsID)
	if errors.Is(err, sql.ErrNoRows) {
		writeJSON(w, http.StatusOK, map[string]any{"active": false})
		return
	} else if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to load share", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"active":     true,
		"created_at": share.CreatedAt.Format(time.RFC3339),
		"expires_at": share.ExpiresAt,
	})
}

// Revoke handles DELETE /api/dashboards/{id}/public-share.
func (h *DashboardShareHandler) Revoke(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, wsID, ok := h.shareScope(w, r)
	if !ok {
		return
	}
	if err := h.shares.Revoke(ctx, id, wsID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeEntityNotFound(w, "share")
			return
		}
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to revoke share", err)
		return
	}
	h.auditLog.Log(ctx, audit.Event{
		UserID:    bimw.UserID(ctx),
		EventType: audit.EventDashboardShareRevoked,
		Details:   map[string]any{"dashboard_id": id},
	})
	w.WriteHeader(http.StatusNoContent)
}
