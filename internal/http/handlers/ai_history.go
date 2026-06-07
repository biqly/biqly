package handlers

import (
	"context"
	"database/sql"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/biqly/biqly/internal/metadata"
)

// isAnonymousRequest reports whether the request carries no authenticated
// identity at all — e.g. OptionalJWTAuth dropped an expired or invalid token.
// Handlers use it to answer 401 instead of 403 so clients re-authenticate.
func isAnonymousRequest(ctx context.Context) bool {
	return bimw.UserID(ctx) == "" && len(bimw.UserRoles(ctx)) == 0
}

func canViewAIHistoryDetails(ctx context.Context, authClient *bimw.AuthClient, userID string) bool {
	if authClient == nil {
		return true
	}
	if bimw.HasRole(ctx, bimw.RoleSuperAdmin) {
		return true
	}
	if userID == "" {
		return false
	}

	workspaceID := bimw.WorkspaceID(ctx)
	allowed, err := authClient.CheckPermission(ctx, userID, PermissionAIViewDetails, "workspace", workspaceID)
	return err == nil && allowed
}

// AIHistory returns a paginated AI query history list. Filtering and pagination
// are applied in the database; heavy fields are masked per permission rules.
//
//nolint:gocognit
func (h *AIHandler) AIHistory(w http.ResponseWriter, r *http.Request) {
	userID := bimw.UserID(r.Context())
	hasViewDetails := canViewAIHistoryDetails(r.Context(), h.authClient, userID)
	var perms []string
	if hasViewDetails {
		perms = []string{PermissionAIViewDetails}
	}

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
	} else if v := q.Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			pageSize = n
		}
	}
	if pageSize > 100 {
		pageSize = 100
	}

	filter := metadata.AIHistoryListFilter{
		DatasourceID: q.Get("datasource_id"),
		ModelID:      q.Get("model_id"),
		Status:       q.Get("status"),
		Search:       q.Get("search"),
		Page:         page,
		PageSize:     pageSize,
	}

	if hasViewDetails {
		if _, ok := q["show_all"]; ok && q.Get("show_all") != "true" && userID != "" {
			filter.UserID = userID
		}
	} else if userID != "" {
		filter.UserID = userID
	}

	if wsFilter, applied := resolveWorkspaceDatasourceFilter(r.Context(), h.deps.Config); applied { //nolint:nestif
		if len(wsFilter) == 0 {
			writeJSON(w, http.StatusOK, map[string]any{"entries": []metadata.AIQueryHistoryEntry{}, "total": 0})
			return
		}
		if filter.DatasourceID != "" {
			if _, ok := wsFilter[filter.DatasourceID]; !ok {
				writeJSON(w, http.StatusOK, map[string]any{"entries": []metadata.AIQueryHistoryEntry{}, "total": 0})
				return
			}
		} else {
			filter.DatasourceIDs = mapKeys(wsFilter)
		}
	}

	result, err := h.deps.MetaRepo.ListAIQueryHistoryFiltered(r.Context(), filter)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "list AI history failed", err)
		return
	}

	entries := result.Entries
	if !hasViewDetails && userID != "" {
		entries = FilterAIHistoryForUser(entries, userID, perms)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"entries": entries,
		"total":   result.Total,
	})
}

// recentQueryItem is a lightweight projection of an AI query history row for
// the home page "recent queries" widget — it omits heavy/sensitive fields.
type recentQueryItem struct {
	ID              string   `json:"id"`
	Question        string   `json:"question"`
	DatasourceID    string   `json:"datasource_id"`
	OutcomeStatus   string   `json:"outcome_status"`
	ConfidenceScore *float64 `json:"confidence_score,omitempty"`
	CreatedAt       string   `json:"created_at"`
}

// QueryHistory returns the current user's most recent AI queries (lightweight).
// Always user-scoped: it never returns other users' rows regardless of role.
func (h *AIHandler) QueryHistory(w http.ResponseWriter, r *http.Request) {
	userID := bimw.UserID(r.Context())
	if userID == "" {
		writeJSON(w, http.StatusOK, map[string]any{"entries": []recentQueryItem{}})
		return
	}

	limit := 10
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 50 {
			limit = n
		}
	}

	rows, err := h.deps.MetaRepo.ListAIQueryHistory(r.Context(), userID, limit)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "list AI history failed", err)
		return
	}
	if wsFilter, applied := resolveWorkspaceDatasourceFilter(r.Context(), h.deps.Config); applied {
		rows = FilterAIHistoryByDatasources(rows, wsFilter)
	}

	items := make([]recentQueryItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, recentQueryItem{
			ID:              row.ID,
			Question:        row.Question,
			DatasourceID:    row.DatasourceID,
			OutcomeStatus:   row.OutcomeStatus,
			ConfidenceScore: row.ConfidenceScore,
			CreatedAt:       row.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"entries": items})
}

// AIHistoryDetail returns a single entry. Non-admin users can only see their
// own entries; otherwise sensitive fields are stripped.
func (h *AIHandler) AIHistoryDetail(w http.ResponseWriter, r *http.Request) {
	userID := bimw.UserID(r.Context())
	hasViewDetails := canViewAIHistoryDetails(r.Context(), h.authClient, userID)

	id := chi.URLParam(r, "id")
	if id == "" {
		id = r.URL.Query().Get("id")
	}
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	row, err := h.deps.MetaRepo.GetAIQueryHistoryByID(r.Context(), id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "entry not found")
			return
		}
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "get AI history failed", err)
		return
	}

	if !hasViewDetails && userID != "" && (row.UserID == nil || *row.UserID != userID) {
		writeError(w, http.StatusForbidden, "not owner of this entry")
		return
	}
	if wsFilter, applied := resolveWorkspaceDatasourceFilter(r.Context(), h.deps.Config); applied {
		if _, ok := wsFilter[row.DatasourceID]; !ok {
			writeError(w, http.StatusNotFound, "entry not found")
			return
		}
	}

	writeJSON(w, http.StatusOK, row)
}

func mapKeys(m map[string]struct{}) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
