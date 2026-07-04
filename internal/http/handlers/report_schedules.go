package handlers

import (
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/biqly/biqly/internal/app"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/biqly/biqly/internal/metadata"
)

// ReportSchedulesHandler serves the scheduled-report admin surface: recurring
// digests that run saved skills on a cadence and mail the results.
type ReportSchedulesHandler struct {
	deps *app.AIDeps
}

// NewReportSchedulesHandler creates a ReportSchedulesHandler.
func NewReportSchedulesHandler(deps *app.AIDeps) *ReportSchedulesHandler {
	return &ReportSchedulesHandler{deps: deps}
}

var reportCadences = []string{"daily", "weekly", "monthly"}

type reportSchedulePayload struct {
	Name       string   `json:"name"`
	SkillIDs   []string `json:"skill_ids"`
	Recipients []string `json:"recipients"`
	Cadence    string   `json:"cadence"`
	HourUTC    int      `json:"hour_utc"`
	Weekday    int      `json:"weekday"`
	DayOfMonth int      `json:"day_of_month"`
	IsActive   *bool    `json:"is_active,omitempty"`
}

type reportScheduleResponse struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	SkillIDs   []string `json:"skill_ids"`
	Recipients []string `json:"recipients"`
	Cadence    string   `json:"cadence"`
	HourUTC    int      `json:"hour_utc"`
	Weekday    int      `json:"weekday"`
	DayOfMonth int      `json:"day_of_month"`
	IsActive   bool     `json:"is_active"`
	LastRunAt  *string  `json:"last_run_at,omitempty"`
	LastStatus string   `json:"last_status"`
	LastError  string   `json:"last_error"`
	CreatedBy  string   `json:"created_by"`
	CreatedAt  string   `json:"created_at"`
	UpdatedAt  string   `json:"updated_at"`
}

func reportScheduleRowToResponse(row metadata.ReportScheduleRow) reportScheduleResponse {
	resp := reportScheduleResponse{
		ID:         row.ID,
		Name:       row.Name,
		SkillIDs:   row.SkillIDs,
		Recipients: row.Recipients,
		Cadence:    row.Cadence,
		HourUTC:    row.HourUTC,
		Weekday:    row.Weekday,
		DayOfMonth: row.DayOfMonth,
		IsActive:   row.IsActive,
		LastStatus: row.LastStatus,
		LastError:  row.LastError,
		CreatedBy:  row.CreatedBy,
		CreatedAt:  row.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:  row.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if resp.SkillIDs == nil {
		resp.SkillIDs = []string{}
	}
	if resp.Recipients == nil {
		resp.Recipients = []string{}
	}
	if row.LastRunAt != nil {
		resp.LastRunAt = new(row.LastRunAt.UTC().Format(time.RFC3339))
	}
	return resp
}

func (p *reportSchedulePayload) validate() string {
	if strings.TrimSpace(p.Name) == "" {
		return "name is required"
	}
	if len(p.SkillIDs) == 0 {
		return "at least one skill is required"
	}
	if len(p.Recipients) == 0 {
		return "at least one recipient is required"
	}
	for _, rcpt := range p.Recipients {
		if !strings.Contains(rcpt, "@") {
			return "invalid recipient email: " + rcpt
		}
	}
	if !slices.Contains(reportCadences, p.Cadence) {
		return "cadence must be one of: daily, weekly, monthly"
	}
	if p.HourUTC < 0 || p.HourUTC > 23 {
		return "hour_utc must be between 0 and 23"
	}
	if p.Weekday < 0 || p.Weekday > 6 {
		return "weekday must be between 0 (Sunday) and 6 (Saturday)"
	}
	if p.DayOfMonth < 1 || p.DayOfMonth > 28 {
		return "day_of_month must be between 1 and 28"
	}
	return ""
}

func (p *reportSchedulePayload) toInput(createdBy string) metadata.ReportScheduleInput {
	active := true
	if p.IsActive != nil {
		active = *p.IsActive
	}
	return metadata.ReportScheduleInput{
		Name:       strings.TrimSpace(p.Name),
		SkillIDs:   p.SkillIDs,
		Recipients: p.Recipients,
		Cadence:    p.Cadence,
		HourUTC:    p.HourUTC,
		Weekday:    p.Weekday,
		DayOfMonth: p.DayOfMonth,
		IsActive:   active,
		CreatedBy:  createdBy,
	}
}

// List returns all report schedules.
func (h *ReportSchedulesHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rows, err := h.deps.MetaRepo.ListReportSchedules(ctx)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to list report schedules", err)
		return
	}
	out := make([]reportScheduleResponse, 0, len(rows))
	for _, row := range rows {
		out = append(out, reportScheduleRowToResponse(row))
	}
	writeJSON(w, http.StatusOK, map[string]any{"schedules": out})
}

// Get returns a single report schedule.
func (h *ReportSchedulesHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	row, err := h.deps.MetaRepo.GetReportSchedule(ctx, chi.URLParam(r, "id"))
	if err != nil {
		if errors.Is(err, metadata.ErrReportScheduleNotFound) {
			writeEntityNotFound(w, "report schedule")
			return
		}
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to load report schedule", err)
		return
	}
	writeJSON(w, http.StatusOK, reportScheduleRowToResponse(row))
}

// Create stores a new report schedule.
func (h *ReportSchedulesHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	payload, ok := decodeJSON[reportSchedulePayload](w, r)
	if !ok {
		return
	}
	if msg := payload.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	id, err := h.deps.MetaRepo.InsertReportSchedule(ctx, payload.toInput(bimw.UserID(ctx)))
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to create report schedule", err)
		return
	}
	row, err := h.deps.MetaRepo.GetReportSchedule(ctx, id)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to load report schedule", err)
		return
	}
	writeJSON(w, http.StatusCreated, reportScheduleRowToResponse(row))
}

// Update replaces the editable fields of a report schedule.
func (h *ReportSchedulesHandler) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	payload, ok := decodeJSON[reportSchedulePayload](w, r)
	if !ok {
		return
	}
	if msg := payload.validate(); msg != "" {
		writeError(w, http.StatusBadRequest, msg)
		return
	}
	id := chi.URLParam(r, "id")
	if err := h.deps.MetaRepo.UpdateReportSchedule(ctx, id, payload.toInput("")); err != nil {
		if errors.Is(err, metadata.ErrReportScheduleNotFound) {
			writeEntityNotFound(w, "report schedule")
			return
		}
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to update report schedule", err)
		return
	}
	row, err := h.deps.MetaRepo.GetReportSchedule(ctx, id)
	if err != nil {
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to load report schedule", err)
		return
	}
	writeJSON(w, http.StatusOK, reportScheduleRowToResponse(row))
}

// Delete removes a report schedule.
func (h *ReportSchedulesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if err := h.deps.MetaRepo.DeleteReportSchedule(ctx, chi.URLParam(r, "id")); err != nil {
		if errors.Is(err, metadata.ErrReportScheduleNotFound) {
			writeEntityNotFound(w, "report schedule")
			return
		}
		writeInternalError(ctx, w, http.StatusInternalServerError, "failed to delete report schedule", err)
		return
	}
	writeOK(w)
}
