package handlers

import (
	"net/http"
	"slices"
	"strconv"

	bimw "github.com/biqly/biqly/internal/http/middleware"
	pkgmetadata "github.com/biqly/biqly/pkg/metadata"
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

	// Fetch a larger window to support server-side pagination of filtered rows
	limit := 1000

	repoFilterUser := userID
	if hasViewDetails || userID == "" {
		repoFilterUser = ""
	}

	rows, err := h.deps.MetaRepo.ListAIQueryHistory(r.Context(), repoFilterUser, limit)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "list AI history failed", err)
		return
	}

	if wsFilter, applied := resolveWorkspaceDatasourceFilter(r.Context(), h.deps); applied {
		rows = FilterAIHistoryByDatasources(rows, wsFilter)
	}
	filtered := FilterAIHistoryForUser(rows, userID, perms)

	total := len(filtered)
	q := r.URL.Query()
	page := 1
	if v := q.Get("page"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			page = n
		}
	}
	pageSize := 10
	if v := q.Get("page_size"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			pageSize = n
		}
	}

	start := (page - 1) * pageSize
	end := start + pageSize

	var paginated []pkgmetadata.AIQueryHistoryEntry
	if start < total {
		if end > total {
			end = total
		}
		paginated = filtered[start:end]
	} else {
		paginated = []pkgmetadata.AIQueryHistoryEntry{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"entries": paginated,
		"total":   total,
	})
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

	wsFilter, wsApplied := resolveWorkspaceDatasourceFilter(r.Context(), h.deps)

	id := r.URL.Query().Get("id")
	for _, row := range rows {
		if row.ID != id {
			continue
		}
		if !hasViewDetails && userID != "" && (row.UserID == nil || *row.UserID != userID) {
			writeError(w, http.StatusForbidden, "not owner of this entry")
			return
		}
		if wsApplied {
			if _, ok := wsFilter[row.DatasourceID]; !ok {
				writeError(w, http.StatusNotFound, "entry not found")
				return
			}
		}
		writeJSON(w, http.StatusOK, row)
		return
	}
	writeError(w, http.StatusNotFound, "entry not found")
}
