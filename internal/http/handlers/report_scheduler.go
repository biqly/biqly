package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/metadata"
)

const (
	reportDigestMaxRows    = 10
	defaultReportInterval  = time.Minute
	reportSectionRunLimit  = 20
	reportStatusSuccess    = "success"
	reportStatusPartial    = "partial"
	reportStatusSendFailed = "error"
)

// reportDigestSender is the mail-service surface the runner needs.
type reportDigestSender interface {
	SendReportDigest(ctx context.Context, email, reportName, sectionsText string, sections []map[string]any, generatedAt time.Time) error
}

// ReportScheduleRunner periodically executes due report schedules: each
// schedule's skills run through the governed query path and the results are
// mailed to the recipients as a digest.
type ReportScheduleRunner struct {
	deps     *app.AIDeps
	mail     reportDigestSender
	interval time.Duration
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// NewReportScheduleRunner creates a runner; a non-positive interval defaults
// to one minute.
func NewReportScheduleRunner(deps *app.AIDeps, mail reportDigestSender, interval time.Duration) *ReportScheduleRunner {
	if interval <= 0 {
		interval = defaultReportInterval
	}
	return &ReportScheduleRunner{deps: deps, mail: mail, interval: interval}
}

// Start launches the periodic due check until the context is cancelled or
// Stop is called.
func (r *ReportScheduleRunner) Start(ctx context.Context) {
	ctx, r.cancel = context.WithCancel(ctx)
	r.wg.Add(1)
	go func() {
		defer r.wg.Done()
		ticker := time.NewTicker(r.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				r.RunDue(ctx, time.Now())
			}
		}
	}()
}

// Stop cancels the runner and waits for the in-flight tick to finish.
func (r *ReportScheduleRunner) Stop() {
	if r.cancel != nil {
		r.cancel()
	}
	r.wg.Wait()
}

// RunDue executes every active schedule whose slot has passed since its last
// run.
func (r *ReportScheduleRunner) RunDue(ctx context.Context, now time.Time) {
	schedules, err := r.deps.MetaRepo.ListActiveReportSchedules(ctx)
	if err != nil {
		slog.ErrorContext(ctx, "failed to list report schedules", "error", err)
		return
	}
	for _, sched := range schedules {
		if ctx.Err() != nil {
			return
		}
		if !reportScheduleDue(sched, now) {
			continue
		}
		r.runSchedule(ctx, sched, now)
	}
}

func (r *ReportScheduleRunner) runSchedule(ctx context.Context, sched metadata.ReportScheduleRow, now time.Time) {
	sections, sectionsText, hadSectionError := r.buildSections(ctx, sched)
	status := reportStatusSuccess
	if hadSectionError {
		status = reportStatusPartial
	}
	var sendErrs []string
	for _, rcpt := range sched.Recipients {
		if err := r.mail.SendReportDigest(ctx, rcpt, sched.Name, sectionsText, sections, now); err != nil {
			slog.ErrorContext(ctx, "failed to send report digest",
				"schedule_id", sched.ID, "error", err)
			sendErrs = append(sendErrs, err.Error())
		}
	}
	errMsg := strings.Join(sendErrs, "; ")
	if len(sendErrs) > 0 {
		status = reportStatusSendFailed
	}
	if err := r.deps.MetaRepo.MarkReportScheduleRun(ctx, sched.ID, status, errMsg, now); err != nil {
		slog.ErrorContext(ctx, "failed to mark report schedule run",
			"schedule_id", sched.ID, "error", err)
	}
	slog.InfoContext(ctx, "report schedule ran",
		"schedule_id", sched.ID, "name", sched.Name, "status", status, "sections", len(sections))
}

// buildSections runs each skill of the schedule and formats one digest
// section per skill. Failures become error sections instead of aborting the
// whole digest.
func (r *ReportScheduleRunner) buildSections(ctx context.Context, sched metadata.ReportScheduleRow) (sections []map[string]any, sectionsText string, hadError bool) {
	var text strings.Builder
	skillIDs := sched.SkillIDs
	if len(skillIDs) > reportSectionRunLimit {
		skillIDs = skillIDs[:reportSectionRunLimit]
	}
	sections = make([]map[string]any, 0, len(skillIDs))
	for _, skillID := range skillIDs {
		section := r.runSkillSection(ctx, skillID)
		sections = append(sections, section)
		if errText, _ := section["Error"].(string); errText != "" {
			hadError = true
			fmt.Fprintf(&text, "- %s: ERROR %s\n", section["Name"], errText)
			continue
		}
		fmt.Fprintf(&text, "- %s: %d rows\n", section["Name"], section["RowCount"])
	}
	return sections, text.String(), hadError
}

func (r *ReportScheduleRunner) runSkillSection(ctx context.Context, skillID string) map[string]any {
	section := map[string]any{
		"Name":     skillID,
		"Question": "",
		"Columns":  []string{},
		"Rows":     [][]string{},
		"RowCount": 0,
		"Error":    "",
	}
	row, err := r.deps.MetaRepo.GetSkill(ctx, skillID)
	if err != nil {
		section["Error"] = "skill not found"
		return section
	}
	section["Name"] = row.Name
	section["Question"] = row.Question
	if !row.IsActive {
		section["Error"] = "skill is deactivated"
		return section
	}
	resp, err := skillRowToResponse(row)
	if err != nil || resp.LogicalQuery == nil {
		section["Error"] = "failed to decode skill"
		return section
	}
	lq := *resp.LogicalQuery
	if err := applySkillParameters(&lq, resp.Parameters, nil); err != nil {
		section["Error"] = err.Error()
		return section
	}
	run, err := r.deps.QueryClient.RunWithModel(ctx, &lq, nil, 0, 0)
	if err != nil {
		section["Error"] = "query failed: " + err.Error()
		return section
	}
	cols := make([]string, len(run.Columns))
	for i, c := range run.Columns {
		cols[i] = c.Name
	}
	limit := min(len(run.Rows), reportDigestMaxRows)
	rows := make([][]string, 0, limit)
	for _, raw := range run.Rows[:limit] {
		cells := make([]string, len(raw))
		for i, cell := range raw {
			if cell != nil {
				cells[i] = fmt.Sprint(cell)
			}
		}
		rows = append(rows, cells)
	}
	section["Columns"] = cols
	section["Rows"] = rows
	section["RowCount"] = run.RowCount
	section["Truncated"] = run.RowCount > limit
	return section
}

// reportScheduleDue reports whether the schedule's most recent slot has
// passed without a run.
func reportScheduleDue(s metadata.ReportScheduleRow, now time.Time) bool {
	slot := reportSlotAt(s, now)
	if now.Before(slot) {
		return false
	}
	last := s.CreatedAt
	if s.LastRunAt != nil {
		last = *s.LastRunAt
	}
	return last.Before(slot)
}

// reportSlotAt returns the most recent scheduled slot at or before now, in
// UTC. Weekday follows time.Weekday numbering (0 = Sunday). DayOfMonth is
// capped at 28 by validation, so month arithmetic never normalizes.
func reportSlotAt(s metadata.ReportScheduleRow, now time.Time) time.Time {
	now = now.UTC()
	switch s.Cadence {
	case "weekly":
		slot := time.Date(now.Year(), now.Month(), now.Day(), s.HourUTC, 0, 0, 0, time.UTC)
		offset := (int(slot.Weekday()) - s.Weekday + 7) % 7
		slot = slot.AddDate(0, 0, -offset)
		if slot.After(now) {
			slot = slot.AddDate(0, 0, -7)
		}
		return slot
	case "monthly":
		slot := time.Date(now.Year(), now.Month(), s.DayOfMonth, s.HourUTC, 0, 0, 0, time.UTC)
		if slot.After(now) {
			slot = slot.AddDate(0, -1, 0)
		}
		return slot
	default: // daily
		slot := time.Date(now.Year(), now.Month(), now.Day(), s.HourUTC, 0, 0, 0, time.UTC)
		if slot.After(now) {
			slot = slot.AddDate(0, 0, -1)
		}
		return slot
	}
}
