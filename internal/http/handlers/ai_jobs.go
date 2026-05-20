package handlers

import (
	"net/http"

	"github.com/biqly/biqly/internal/metadata"
	"github.com/go-chi/chi/v5"
)

type AIJobsHandler struct {
	svc *AIJobService
}

func NewAIJobsHandler(svc *AIJobService) *AIJobsHandler {
	return &AIJobsHandler{svc: svc}
}

func (h *AIJobsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createAIJobRequest
	parsed, ok := decodeJSON[createAIJobRequest](w, r)
	if !ok {
		return
	}
	req = *parsed
	job, err := h.svc.Enqueue(r.Context(), req.Kind, req.ClientSessionID, req.Request)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (h *AIJobsHandler) Get(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	job, err := h.svc.repo.GetAIJob(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (h *AIJobsHandler) List(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("client_session_id")
	if sessionID == "" {
		writeError(w, http.StatusBadRequest, "client_session_id is required")
		return
	}
	activeOnly := r.URL.Query().Get("active") == "true"
	jobs, err := h.svc.repo.ListAIJobsBySession(r.Context(), sessionID, activeOnly, 50)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "list jobs failed", err)
		return
	}
	if jobs == nil {
		jobs = []metadata.AIJob{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": jobs})
}
