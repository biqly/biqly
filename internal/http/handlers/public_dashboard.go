package handlers

import (
	"errors"
	"net/http"

	"github.com/biqly/biqly/internal/dashboard"
)

// PublicDashboardHandler serves anonymous, sanitized dashboard metadata.
type PublicDashboardHandler struct {
	resolver   *dashboard.PublicResolver
	killSwitch workspaceSharingChecker
}

// NewPublicDashboardHandler creates a PublicDashboardHandler.
func NewPublicDashboardHandler(resolver *dashboard.PublicResolver, killSwitch workspaceSharingChecker) *PublicDashboardHandler {
	return &PublicDashboardHandler{resolver: resolver, killSwitch: killSwitch}
}

// Get handles GET /api/public/dashboards/{token}. Every failure mode returns
// the same 404 so the endpoint leaks nothing about token validity.
func (h *PublicDashboardHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	token, ok := requireURLParam(w, r, "token")
	if !ok {
		return
	}
	view, err := h.resolver.ResolveDashboard(ctx, token)
	if err != nil {
		if errors.Is(err, dashboard.ErrShareNotFound) {
			writeEntityNotFound(w, "dashboard")
			return
		}
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to resolve share", err)
		return
	}
	enabled, err := h.killSwitch.WorkspacePublicSharingEnabled(ctx, view.Share.WorkspaceID)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to check sharing policy", err)
		return
	}
	if !enabled {
		writeEntityNotFound(w, "dashboard")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":          view.Dashboard.ID,
		"name":        view.Dashboard.Name,
		"description": view.Dashboard.Description,
		"widgets":     view.Dashboard.Widgets,
	})
}
