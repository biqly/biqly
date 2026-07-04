package metadata

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/biqly/biqly/internal/platform/db/pgarray"
)

// ErrReportScheduleNotFound is returned when a schedule id does not exist.
var ErrReportScheduleNotFound = errors.New("report schedule not found")

// ReportScheduleRow is a recurring digest: a set of skills run on a cadence
// with the results emailed to the recipients.
type ReportScheduleRow struct {
	ID         string
	Name       string
	SkillIDs   []string
	Recipients []string
	Cadence    string
	HourUTC    int
	Weekday    int
	DayOfMonth int
	IsActive   bool
	LastRunAt  *time.Time
	LastStatus string
	LastError  string
	CreatedBy  string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// ReportScheduleInput is the input for creating or updating a schedule.
type ReportScheduleInput struct {
	Name       string
	SkillIDs   []string
	Recipients []string
	Cadence    string
	HourUTC    int
	Weekday    int
	DayOfMonth int
	IsActive   bool
	CreatedBy  string
}

const reportScheduleColumns = `id, name, skill_ids, recipients, cadence, hour_utc, weekday, day_of_month,
	is_active, last_run_at, last_status, last_error, created_by, created_at, updated_at`

// InsertReportSchedule stores a new schedule and returns its generated id.
func (r *Repository) InsertReportSchedule(ctx context.Context, in ReportScheduleInput) (string, error) {
	var id string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO report_schedules (name, skill_ids, recipients, cadence, hour_utc, weekday, day_of_month, is_active, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`, in.Name, pgarray.Strings(in.SkillIDs), pgarray.Strings(in.Recipients), in.Cadence,
		in.HourUTC, in.Weekday, in.DayOfMonth, in.IsActive, in.CreatedBy).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert report schedule: %w", err)
	}
	return id, nil
}

// ListReportSchedules returns all schedules, newest-updated first.
func (r *Repository) ListReportSchedules(ctx context.Context) ([]ReportScheduleRow, error) {
	return r.listReportSchedules(ctx, "list report schedules",
		`SELECT `+reportScheduleColumns+` FROM report_schedules ORDER BY updated_at DESC`)
}

// ListActiveReportSchedules returns active schedules for the due check.
func (r *Repository) ListActiveReportSchedules(ctx context.Context) ([]ReportScheduleRow, error) {
	return r.listReportSchedules(ctx, "list active report schedules",
		`SELECT `+reportScheduleColumns+` FROM report_schedules WHERE is_active ORDER BY created_at`)
}

func (r *Repository) listReportSchedules(ctx context.Context, label, query string) ([]ReportScheduleRow, error) {
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", label, err)
	}
	defer func() { _ = rows.Close() }()

	var out []ReportScheduleRow
	for rows.Next() {
		row, err := scanReportSchedule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("%s rows: %w", label, err)
	}
	return out, nil
}

// GetReportSchedule returns a single schedule by id.
func (r *Repository) GetReportSchedule(ctx context.Context, id string) (ReportScheduleRow, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+reportScheduleColumns+` FROM report_schedules WHERE id = $1`, id)
	sched, err := scanReportSchedule(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ReportScheduleRow{}, fmt.Errorf("report schedule %s: %w", id, ErrReportScheduleNotFound)
		}
		return ReportScheduleRow{}, err
	}
	return sched, nil
}

// UpdateReportSchedule replaces the editable fields of a schedule.
func (r *Repository) UpdateReportSchedule(ctx context.Context, id string, in ReportScheduleInput) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE report_schedules
		SET name = $2, skill_ids = $3, recipients = $4, cadence = $5, hour_utc = $6,
			weekday = $7, day_of_month = $8, is_active = $9, updated_at = now()
		WHERE id = $1
	`, id, in.Name, pgarray.Strings(in.SkillIDs), pgarray.Strings(in.Recipients), in.Cadence,
		in.HourUTC, in.Weekday, in.DayOfMonth, in.IsActive)
	if err != nil {
		return fmt.Errorf("update report schedule: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update report schedule affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("report schedule %s: %w", id, ErrReportScheduleNotFound)
	}
	return nil
}

// DeleteReportSchedule removes a schedule.
func (r *Repository) DeleteReportSchedule(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM report_schedules WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete report schedule: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete report schedule affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("report schedule %s: %w", id, ErrReportScheduleNotFound)
	}
	return nil
}

// MarkReportScheduleRun records the outcome of a schedule run.
func (r *Repository) MarkReportScheduleRun(ctx context.Context, id, status, errMsg string, ranAt time.Time) error {
	if _, err := r.db.ExecContext(ctx, `
		UPDATE report_schedules SET last_run_at = $2, last_status = $3, last_error = $4 WHERE id = $1
	`, id, ranAt, status, errMsg); err != nil {
		return fmt.Errorf("mark report schedule run: %w", err)
	}
	return nil
}

func scanReportSchedule(s interface{ Scan(...any) error }) (ReportScheduleRow, error) {
	var row ReportScheduleRow
	var skillIDs, recipients pgarray.StringArray
	err := s.Scan(&row.ID, &row.Name, &skillIDs, &recipients, &row.Cadence, &row.HourUTC,
		&row.Weekday, &row.DayOfMonth, &row.IsActive, &row.LastRunAt, &row.LastStatus,
		&row.LastError, &row.CreatedBy, &row.CreatedAt, &row.UpdatedAt)
	if err != nil {
		return row, fmt.Errorf("scan report schedule: %w", err)
	}
	row.SkillIDs = skillIDs
	row.Recipients = recipients
	return row, nil
}
