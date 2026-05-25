package handlers

import (
	"net/http"
	"slices"
	"strconv"

	bimw "github.com/biqly/biqly/internal/http/middleware"
)

// AIHistory returns the AI query history list, filtered by user when the
// caller lacks ai:queue:view_details. Heavy fields (prompt context, AI
// response, logical query) are masked for non-admin views.
//
// When the auth feature flag is off, falls back to legacy "all rows"
// behavior — the API key alone is the access control.
func (h *AIHandler) AIHistory(w http.ResponseWriter, r *http.Request) {
	userID := bimw.UserID(r.Context())
	perms := bimw.Permissions(r.Context())
	hasViewDetails := slices.Contains(perms, PermissionAIViewDetails)

	limit := 50
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}

	// Admin path: pass empty userID to fetch all rows. Legacy mode (no userID
	// in context) also falls through here for backward compat — the API key
	// already gates access.
	repoFilterUser := userID
	if hasViewDetails || userID == "" {
		repoFilterUser = ""
	}

	rows, err := h.deps.MetaRepo.ListAIQueryHistory(r.Context(), repoFilterUser, limit)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "list AI history failed", err)
		return
	}

	filtered := FilterAIHistoryForUser(rows, userID, perms)
	writeJSON(w, http.StatusOK, map[string]any{"entries": filtered})
}

// AIHistoryDetail returns a single entry. Non-admin users can only see their
// own entries; otherwise sensitive fields are stripped.
func (h *AIHandler) AIHistoryDetail(w http.ResponseWriter, r *http.Request) {
	userID := bimw.UserID(r.Context())
	perms := bimw.Permissions(r.Context())
	hasViewDetails := slices.Contains(perms, PermissionAIViewDetails)

	rows, err := h.deps.MetaRepo.ListAIQueryHistory(r.Context(), "", 500)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "list AI history failed", err)
		return
	}

	id := r.URL.Query().Get("id")
	for _, row := range rows {
		if row.ID != id {
			continue
		}
		if !hasViewDetails && userID != "" && (row.UserID == nil || *row.UserID != userID) {
			writeError(w, http.StatusForbidden, "not owner of this entry")
			return
		}
		writeJSON(w, http.StatusOK, row)
		return
	}
	writeError(w, http.StatusNotFound, "entry not found")
}
