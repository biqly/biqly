package metadata

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

const (
	AIJobStatusPending   = "pending"
	AIJobStatusQueued    = "queued"
	AIJobStatusRunning   = "running"
	AIJobStatusSucceeded = "succeeded"
	AIJobStatusFailed    = "failed"
	AIJobStatusCancelled = "cancelled"
)

type AIJob struct {
	ID              string          `json:"id"`
	ClientSessionID string          `json:"client_session_id"`
	Kind            string          `json:"kind"`
	Status          string          `json:"status"`
	Phase           string          `json:"phase"`
	PhaseMessage    string          `json:"phase_message"`
	ProgressPct     int             `json:"progress_pct"`
	RequestJSON     json.RawMessage `json:"request_json"`
	ResultJSON      json.RawMessage `json:"result_json,omitempty"`
	ErrorMessage    string          `json:"error_message,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
	UpdatedAt       time.Time       `json:"updated_at"`
	StartedAt       *time.Time      `json:"started_at,omitempty"`
	FinishedAt      *time.Time      `json:"finished_at,omitempty"`
}

func (r *Repository) CreateAIJob(ctx context.Context, job *AIJob) error {
	if job == nil {
		return fmt.Errorf("job is nil")
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO ai_jobs (
			id, client_session_id, kind, status, phase, phase_message, progress_pct, request_json
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8::jsonb)`,
		job.ID, job.ClientSessionID, job.Kind, job.Status, job.Phase, job.PhaseMessage, job.ProgressPct, job.RequestJSON,
	)
	if err != nil {
		return fmt.Errorf("create ai job: %w", err)
	}
	return nil
}

func (r *Repository) GetAIJob(ctx context.Context, id string) (*AIJob, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, client_session_id, kind, status, phase, phase_message, progress_pct,
		       request_json, result_json, error_message, created_at, updated_at, started_at, finished_at
		FROM ai_jobs WHERE id = $1`, id)
	return scanAIJob(row)
}

func (r *Repository) ListAIJobsBySession(ctx context.Context, sessionID string, activeOnly bool, limit int) ([]AIJob, error) {
	if limit <= 0 {
		limit = 50
	}
	q := `
		SELECT id, client_session_id, kind, status, phase, phase_message, progress_pct,
		       request_json, result_json, error_message, created_at, updated_at, started_at, finished_at
		FROM ai_jobs
		WHERE client_session_id = $1`
	if activeOnly {
		q += ` AND status IN ('pending', 'queued', 'running')`
	}
	q += ` ORDER BY created_at DESC LIMIT $2`

	rows, err := r.db.QueryContext(ctx, q, sessionID, limit)
	if err != nil {
		return nil, fmt.Errorf("list ai jobs: %w", err)
	}
	defer rows.Close()

	var out []AIJob
	for rows.Next() {
		job, err := scanAIJobRows(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *job)
	}
	return out, rows.Err()
}

func (r *Repository) UpdateAIJobProgress(ctx context.Context, id, status, phase, phaseMessage string, progressPct int) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE ai_jobs
		SET status = $2, phase = $3, phase_message = $4, progress_pct = $5, updated_at = NOW()
		WHERE id = $1`, id, status, phase, phaseMessage, progressPct)
	if err != nil {
		return fmt.Errorf("update ai job progress: %w", err)
	}
	return nil
}

func (r *Repository) MarkAIJobRunning(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE ai_jobs
		SET status = $2, phase = 'routing', phase_message = '', progress_pct = 5,
		    started_at = COALESCE(started_at, NOW()), updated_at = NOW()
		WHERE id = $1 AND status IN ('pending', 'queued')`, id, AIJobStatusRunning)
	if err != nil {
		return fmt.Errorf("mark ai job running: %w", err)
	}
	return nil
}

func (r *Repository) CompleteAIJob(ctx context.Context, id string, result json.RawMessage) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE ai_jobs
		SET status = $2, phase = 'completed', phase_message = '', progress_pct = 100,
		    result_json = $3::jsonb, error_message = NULL, finished_at = NOW(), updated_at = NOW()
		WHERE id = $1`, id, AIJobStatusSucceeded, result)
	if err != nil {
		return fmt.Errorf("complete ai job: %w", err)
	}
	return nil
}

func (r *Repository) FailAIJob(ctx context.Context, id, message string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE ai_jobs
		SET status = $2, phase = 'failed', phase_message = '', error_message = $3,
		    finished_at = NOW(), updated_at = NOW()
		WHERE id = $1`, id, AIJobStatusFailed, message)
	if err != nil {
		return fmt.Errorf("fail ai job: %w", err)
	}
	return nil
}

func scanAIJob(row *sql.Row) (*AIJob, error) {
	var job AIJob
	var result []byte
	var errMsg sql.NullString
	var started, finished sql.NullTime
	err := row.Scan(
		&job.ID, &job.ClientSessionID, &job.Kind, &job.Status, &job.Phase, &job.PhaseMessage, &job.ProgressPct,
		&job.RequestJSON, &result, &errMsg, &job.CreatedAt, &job.UpdatedAt, &started, &finished,
	)
	if err != nil {
		return nil, err
	}
	if len(result) > 0 {
		job.ResultJSON = json.RawMessage(result)
	}
	if errMsg.Valid {
		job.ErrorMessage = errMsg.String
	}
	if started.Valid {
		t := started.Time
		job.StartedAt = &t
	}
	if finished.Valid {
		t := finished.Time
		job.FinishedAt = &t
	}
	return &job, nil
}

type aiJobScanner interface {
	Scan(dest ...any) error
}

func scanAIJobRows(rows aiJobScanner) (*AIJob, error) {
	var job AIJob
	var result []byte
	var errMsg sql.NullString
	var started, finished sql.NullTime
	err := rows.Scan(
		&job.ID, &job.ClientSessionID, &job.Kind, &job.Status, &job.Phase, &job.PhaseMessage, &job.ProgressPct,
		&job.RequestJSON, &result, &errMsg, &job.CreatedAt, &job.UpdatedAt, &started, &finished,
	)
	if err != nil {
		return nil, err
	}
	if len(result) > 0 {
		job.ResultJSON = json.RawMessage(result)
	}
	if errMsg.Valid {
		job.ErrorMessage = errMsg.String
	}
	if started.Valid {
		t := started.Time
		job.StartedAt = &t
	}
	if finished.Valid {
		t := finished.Time
		job.FinishedAt = &t
	}
	return &job, nil
}
