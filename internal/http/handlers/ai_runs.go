package handlers

import (
	"errors"
	"net/http"
	"time"

	"github.com/biqly/biqly/internal/metadata"
	"github.com/go-chi/chi/v5"
)

// agentRunDTO is the wire shape of a persisted agent run.
type agentRunDTO struct {
	ID             string    `json:"id"`
	ConversationID string    `json:"conversation_id,omitempty"`
	DatasourceID   string    `json:"datasource_id"`
	ModelID        string    `json:"model_id,omitempty"`
	UserID         string    `json:"user_id,omitempty"`
	Question       string    `json:"question"`
	Mode           string    `json:"mode"`
	Status         string    `json:"status"`
	Confidence     float64   `json:"confidence"`
	Answer         string    `json:"answer,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// agentStepDTO is the wire shape of one recorded pipeline step; it matches the
// in-request ai.RunStep JSON so the frontend can reuse the RunStep renderer.
type agentStepDTO struct {
	Seq        int    `json:"seq"`
	Kind       string `json:"kind"`
	Status     string `json:"status"`
	Attempt    int    `json:"attempt,omitempty"`
	DurationMs int    `json:"duration_ms"`
	Detail     string `json:"detail,omitempty"`
}

type agentRunDetailResponse struct {
	Run   agentRunDTO    `json:"run"`
	Steps []agentStepDTO `json:"steps"`
}

type agentRunListResponse struct {
	Runs []agentRunDTO `json:"runs"`
}

func toAgentRunDTO(row metadata.AgentRunRow) agentRunDTO {
	return agentRunDTO{
		ID:             row.ID,
		ConversationID: row.ConversationID,
		DatasourceID:   row.DatasourceID,
		ModelID:        row.ModelID,
		UserID:         row.UserID,
		Question:       row.Question,
		Mode:           row.Mode,
		Status:         row.Status,
		Confidence:     row.Confidence,
		Answer:         row.Answer,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
}

func toAgentStepDTOs(steps []metadata.AgentStepRow) []agentStepDTO {
	out := make([]agentStepDTO, 0, len(steps))
	for _, s := range steps {
		out = append(out, agentStepDTO{
			Seq:        s.Seq,
			Kind:       s.Kind,
			Status:     s.Status,
			Attempt:    s.Attempt,
			DurationMs: s.DurationMs,
			Detail:     s.Detail,
		})
	}
	return out
}

// GetAgentRun returns a persisted run and its ordered steps. Per-datasource
// access is enforced by the router (RequireResolvedDatasourceAccess +
// DatasourceForAgentRun).
func (h *AIHandler) GetAgentRun(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "run id is required")
		return
	}
	run, steps, err := h.deps.MetaRepo.GetAgentRun(r.Context(), id)
	if err != nil {
		if errors.Is(err, metadata.ErrAgentRunNotFound) {
			writeEntityNotFound(w, "run")
			return
		}
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "get agent run failed", err)
		return
	}
	writeJSON(w, http.StatusOK, agentRunDetailResponse{
		Run:   toAgentRunDTO(run),
		Steps: toAgentStepDTOs(steps),
	})
}

// ListAgentRuns returns the runs for a conversation, newest first. The
// conversation must be owned by the requesting user.
func (h *AIHandler) ListAgentRuns(w http.ResponseWriter, r *http.Request) {
	userID := historyUserID(r.Context())
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "user required")
		return
	}
	conversationID := r.URL.Query().Get("conversation_id")
	if conversationID == "" {
		writeError(w, http.StatusBadRequest, "conversation_id is required")
		return
	}
	owned, err := h.deps.MetaRepo.ConversationBelongsToUser(r.Context(), conversationID, userID)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "list agent runs failed", err)
		return
	}
	if !owned {
		writeEntityNotFound(w, "conversation")
		return
	}
	runs, err := h.deps.MetaRepo.ListAgentRuns(r.Context(), conversationID)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "list agent runs failed", err)
		return
	}
	dtos := make([]agentRunDTO, 0, len(runs))
	for _, run := range runs {
		dtos = append(dtos, toAgentRunDTO(run))
	}
	writeJSON(w, http.StatusOK, agentRunListResponse{Runs: dtos})
}
