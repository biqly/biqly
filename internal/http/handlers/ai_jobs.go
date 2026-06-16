package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/biqly/biqly/internal/audit"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/go-chi/chi/v5"
)

type AIJobsHandler struct {
	svc   *AIJobService
	audit *audit.Logger
}

func NewAIJobsHandler(svc *AIJobService, auditLog *audit.Logger) *AIJobsHandler {
	return &AIJobsHandler{svc: svc, audit: auditLog}
}

// aiJobsAdminRole reports whether the caller may manage other users' jobs.
func aiJobsAdminRole(ctx context.Context) bool {
	return bimw.HasRole(ctx, bimw.RoleSuperAdmin) || bimw.HasRole(ctx, "admin")
}

// aiJobOwnedBy matches a job to its creator: by user id, or by client session
// for legacy jobs enqueued without one.
func aiJobOwnedBy(job *metadata.AIJob, userID, sessionID string) bool {
	if userID != "" && job.UserID != nil && *job.UserID == userID {
		return true
	}
	return sessionID != "" && job.ClientSessionID == sessionID
}

func (h *AIJobsHandler) Create(w http.ResponseWriter, r *http.Request) {
	var req createAIJobRequest
	parsed, ok := decodeJSON[createAIJobRequest](w, r)
	if !ok {
		return
	}
	req = *parsed
	job, err := h.svc.Enqueue(r.Context(), req.Kind, req.ClientSessionID, bimw.UserID(r.Context()), req.Request)
	if err != nil {
		if conflict, ok := errors.AsType[*AIJobConflictError](err); ok {
			writeAIJobConflict(w, conflict)
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusAccepted, job)
}

func (h *AIJobsHandler) DescribeBatchConflict(w http.ResponseWriter, r *http.Request) {
	datasourceID, ok := requireQueryParam(w, r, "datasource_id")
	if !ok {
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
	ctx := r.Context()
	job, err := h.svc.repo.GetAIJob(ctx, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	isAdmin := aiJobsAdminRole(ctx)
	owned := aiJobOwnedBy(job, bimw.UserID(ctx), r.URL.Query().Get("client_session_id"))
	if !isAdmin && !owned {
		writeError(w, http.StatusForbidden, "not allowed to view this job")
		return
	}
	writeJSON(w, http.StatusOK, job)
}

func (h *AIJobsHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	ctx := r.Context()
	existing, err := h.svc.repo.GetAIJob(ctx, id)
	if err != nil {
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	isAdmin := aiJobsAdminRole(ctx)
	owned := aiJobOwnedBy(existing, bimw.UserID(ctx), r.URL.Query().Get("client_session_id"))
	if !isAdmin && !owned {
		writeError(w, http.StatusForbidden, "not allowed to cancel this job")
		return
	}
	job, err := h.svc.Cancel(ctx, id)
	if err != nil {
		if err.Error() == "job cannot be cancelled" {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusNotFound, "job not found")
		return
	}
	if isAdmin && !owned {
		h.logAdminCancel(ctx, existing)
	}
	writeJSON(w, http.StatusOK, job)
}

// logAdminCancel records an audit event when an admin cancels another user's
// job — admin interventions on foreign jobs must stay traceable.
func (h *AIJobsHandler) logAdminCancel(ctx context.Context, job *metadata.AIJob) {
	if h.audit == nil {
		return
	}
	details := map[string]any{"job_id": job.ID, "kind": job.Kind}
	if job.UserID != nil {
		details["owner_user_id"] = *job.UserID
	}
	h.audit.Log(ctx, audit.Event{
		UserID:    bimw.UserID(ctx),
		EventType: audit.EventAIJobCancelled,
		Details:   details,
	})
}

func (h *AIJobsHandler) List(w http.ResponseWriter, r *http.Request) {
	activeOnly := r.URL.Query().Get("active") == "true"
	var (
		jobs []metadata.AIJob
		err  error
	)
	if r.URL.Query().Get("scope") == "user" {
		// User scope spans every client session of the caller, so a refreshed
		// page or a second tab can re-attach to jobs started elsewhere.
		userID := bimw.UserID(r.Context())
		if userID == "" {
			writeError(w, http.StatusBadRequest, "authenticated user required for scope=user")
			return
		}
		jobs, err = h.svc.repo.ListAIJobsByUser(r.Context(), userID, activeOnly, 50)
	} else {
		sessionID, ok := requireQueryParam(w, r, "client_session_id")
		if !ok {
			return
		}
		jobs, err = h.svc.repo.ListAIJobsBySession(r.Context(), sessionID, activeOnly, 50)
	}
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
	if n, ok := bimw.ParsePositiveIntQueryParam(r, "older_minutes"); ok {
		mins = n
	}
	return time.Duration(mins) * time.Minute
}

func (h *AIJobsHandler) ListStale(w http.ResponseWriter, r *http.Request) {
	sessionID := r.URL.Query().Get("client_session_id")
	limit := bimw.PaginationFromContext(r.Context()).Limit
	if limit <= 0 {
		limit = 100
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
	// ClientSessionID lets callers claim legacy jobs that have no user_id.
	ClientSessionID string `json:"client_session_id,omitempty"`
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
	ctx := r.Context()
	var (
		n   int
		err error
	)
	if aiJobsAdminRole(ctx) {
		n, err = h.svc.repo.CancelAIJobs(ctx, req.IDs)
	} else {
		// Non-admins can only cancel jobs they own.
		n, err = h.svc.repo.CancelAIJobsOwned(ctx, req.IDs, bimw.UserID(ctx), req.ClientSessionID)
	}
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "cancel jobs failed", err)
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

// AdminList returns jobs across all users and sessions, active first, for the
// admin AI jobs panel. Guarded by AdminAccessMiddleware on the route.
func (h *AIJobsHandler) AdminList(w http.ResponseWriter, r *http.Request) {
	pag := bimw.PaginationFromContext(r.Context())
	limit := pag.Limit
	if limit <= 0 {
		limit = 25
	}
	filter := metadata.AIJobsAdminFilter{
		Status: r.URL.Query().Get("status"),
		Kind:   r.URL.Query().Get("kind"),
		UserID: r.URL.Query().Get("user_id"),
		Limit:  limit,
		Offset: pag.Offset,
	}
	jobs, err := h.svc.repo.ListAIJobsAdmin(r.Context(), filter)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "list jobs failed", err)
		return
	}
	if jobs == nil {
		jobs = []metadata.AIJob{}
	}
	total, err := h.svc.repo.CountAIJobsAdmin(r.Context(), filter)
	if err != nil {
		writeInternalError(r.Context(), w, http.StatusInternalServerError, "count jobs failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"jobs": jobs, "total": total})
}

func (h *AIJobsHandler) AdminListStale(w http.ResponseWriter, r *http.Request) {
	limit := bimw.PaginationFromContext(r.Context()).Limit
	if limit <= 0 {
		limit = 200
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
