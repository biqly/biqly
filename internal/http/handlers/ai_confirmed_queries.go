package handlers

import (
	"net/http"
	"time"

	"github.com/biqly/biqly/internal/core"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// confirmedQueriesAdminLimit caps the admin listing page size.
const confirmedQueriesAdminLimit = 200

// confirmedQueryAdminResponse is the wire shape of one confirmed NL→SQL pair
// in the admin listing.
type confirmedQueryAdminResponse struct {
	ID                string    `json:"id"`
	DatasourceID      string    `json:"datasource_id"`
	ModelID           string    `json:"model_id,omitempty"`
	UserID            string    `json:"user_id,omitempty"`
	NLQuery           string    `json:"nl_query"`
	SQLQuery          string    `json:"sql_query"`
	SemanticModelHash string    `json:"semantic_model_hash,omitempty"`
	IsActive          bool      `json:"is_active"`
	ConfirmedAt       time.Time `json:"confirmed_at"`
}

// AdminListConfirmedQueries returns the newest confirmed NL→SQL pairs for a
// datasource, including deactivated rows so admins can audit the memory store.
func (h *AIHandler) AdminListConfirmedQueries(w http.ResponseWriter, r *http.Request) {
	datasourceID := r.URL.Query().Get("datasource_id")
	if datasourceID == "" {
		writeError(w, http.StatusBadRequest, core.MsgDatasourceIDRequired)
		return
	}
	if _, err := uuid.Parse(datasourceID); err != nil {
		writeError(w, http.StatusBadRequest, "datasource_id must be a valid UUID")
		return
	}
	ctx := r.Context()
	rows, err := h.deps.MetaRepo.ListConfirmedQueriesForAdmin(ctx, datasourceID, confirmedQueriesAdminLimit)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to list confirmed queries", err)
		return
	}
	out := make([]confirmedQueryAdminResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, confirmedQueryAdminResponse{
			ID:                row.ID,
			DatasourceID:      row.DatasourceID,
			ModelID:           row.ModelID,
			UserID:            row.UserID,
			NLQuery:           row.NLQuery,
			SQLQuery:          row.SQLQuery,
			SemanticModelHash: row.SemanticModelHash,
			IsActive:          row.IsActive,
			ConfirmedAt:       row.ConfirmedAt,
		})
	}
	writeJSON(w, http.StatusOK, out)
}

// AdminDeactivateConfirmedQuery removes one confirmed pair from few-shot recall.
func (h *AIHandler) AdminDeactivateConfirmedQuery(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		writeError(w, http.StatusBadRequest, "id must be a valid UUID")
		return
	}
	ctx := r.Context()
	n, err := h.deps.MetaRepo.SetConfirmedQueryActive(ctx, id, false)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to deactivate confirmed query", err)
		return
	}
	if n == 0 {
		writeError(w, http.StatusNotFound, "confirmed query not found")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deactivated"})
}
