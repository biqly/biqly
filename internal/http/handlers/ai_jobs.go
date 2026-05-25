package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

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
		var conflict *AIJobConflictError
		if errors.As(err, &conflict) {
			writeAIJobConflict(w, conflict)
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (h *AIJobsHandler) DescribeBatchConflict(w http.ResponseWriter, r *http.Request) {
	datasourceID := r.URL.Query().Get("datasource_id")
	if datasourceID == "" {
		writeError(w, http.StatusBadRequest, "datasource_id is required")
		return
	}
	rawSchemas := r.URL.Query()["schemas"]
	if len(rawSchemas) == 1 && strings.Contains(rawSchemas[0], ",") {
		rawSchemas = strings.Split(rawSchemas[0], ",")
	}
	schemas := make([]string, 0, len(rawSchemas))
	seen := make(map[string]struct{})
	for _, s := range rawSchemas {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		schemas = append(schemas, s)
	}
	if len(schemas) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{"conflict": false})
		return
	}
	existing, err := h.svc.repo.FindConflictingDescribeBatch(r.Context(), datasourceID, schemas)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "failed to check describe batch conflict", err)
		return
	}
	if existing == nil {
		writeJSON(w, http.StatusOK, map[string]any{"conflict": false})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"conflict":        true,
		"existing_job_id": existing.ID,
		"existing_job":    existing,
		"scope_schemas":   existing.ScopeSchemas,
	})
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

func (h *AIJobsHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	job, err := h.svc.Cancel(r.Context(), id)
	if err != nil {
		if err.Error() == "job cannot be cancelled" {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
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

// QueueStatus returns the AI queue snapshot — pending count plus the caller's
// position. Designed for all authenticated users: leaks only their own
// position and the global count, never other users' details.
func (h *AIJobsHandler) QueueStatus(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("client_session_id")
	status, err := h.svc.repo.GetAIQueueStatus(r.Context(), sessionID)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "queue status failed", err)
		return
	}
	writeJSON(w, http.StatusOK, status)
}

func staleJobOlderThan(r *http.Request) time.Duration {
	mins := 15
	if v := r.URL.Query().Get("older_minutes"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			mins = n
		}
	}
	return time.Duration(mins) * time.Minute
}

func (h *AIJobsHandler) ListStale(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("client_session_id")
	limit := 100
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	jobs, err := h.svc.repo.ListStaleAIJobs(r.Context(), sessionID, staleJobOlderThan(r), limit)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "list stale jobs failed", err)
		return
	}
	if jobs == nil {
		jobs = []metadata.AIJob{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": jobs})
}

type cancelAIJobsRequest struct {
	IDs []string `json:"ids"`
}

func (h *AIJobsHandler) CancelBatch(w http.ResponseWriter, r *http.Request) {
	var req cancelAIJobsRequest
	parsed, ok := decodeJSON[cancelAIJobsRequest](w, r)
	if !ok {
		return
	}
	req = *parsed
	if len(req.IDs) == 0 {
		writeError(w, http.StatusBadRequest, "ids is required")
		return
	}
	n, err := h.svc.repo.CancelAIJobs(r.Context(), req.IDs)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "cancel jobs failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"cancelled": n})
}

type cancelActiveAIJobsRequest struct {
	ClientSessionID string `json:"client_session_id"`
}

func (h *AIJobsHandler) CancelActive(w http.ResponseWriter, r *http.Request) {
	var req cancelActiveAIJobsRequest
	parsed, ok := decodeJSON[cancelActiveAIJobsRequest](w, r)
	if !ok {
		return
	}
	req = *parsed
	if req.ClientSessionID == "" {
		writeError(w, http.StatusBadRequest, "client_session_id is required")
		return
	}
	n, err := h.svc.repo.CancelActiveAIJobsBySession(r.Context(), req.ClientSessionID)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "cancel active jobs failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"cancelled": n})
}

func (h *AIJobsHandler) AdminListStale(w http.ResponseWriter, r *http.Request) {
	limit := 200
	if v := r.URL.Query().Get("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 && n <= 500 {
			limit = n
		}
	}
	jobs, err := h.svc.repo.ListStaleAIJobs(r.Context(), "", staleJobOlderThan(r), limit)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "list stale jobs failed", err)
		return
	}
	if jobs == nil {
		jobs = []metadata.AIJob{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": jobs})
}

func (h *AIJobsHandler) AdminCancelAllStale(w http.ResponseWriter, r *http.Request) {
	jobs, err := h.svc.repo.ListStaleAIJobs(r.Context(), "", staleJobOlderThan(r), 500)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "list stale jobs failed", err)
		return
	}
	ids := make([]string, 0, len(jobs))
	for _, j := range jobs {
		ids = append(ids, j.ID)
	}
	n, err := h.svc.repo.CancelAIJobs(r.Context(), ids)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "cancel jobs failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"cancelled": n, "matched": len(ids)})
}
