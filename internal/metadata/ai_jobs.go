package metadata

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/lib/pq"
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
	DatasourceID    *string         `json:"datasource_id,omitempty"`
	ScopeSchemas    []string        `json:"scope_schemas,omitempty"`
	ProgressJSON    json.RawMessage `json:"progress_json,omitempty"`
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
			id, client_session_id, kind, status, phase, phase_message, progress_pct,
			datasource_id, scope_schemas, request_json
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8::uuid, $9, $10::jsonb)`,
		job.ID, job.ClientSessionID, job.Kind, job.Status, job.Phase, job.PhaseMessage, job.ProgressPct,
		job.DatasourceID, pq.Array(job.ScopeSchemas), job.RequestJSON,
	)
	if err != nil {
		return fmt.Errorf("create ai job: %w", err)
	}
	return nil
}

func (r *Repository) GetAIJob(ctx context.Context, id string) (*AIJob, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, client_session_id, kind, status, phase, phase_message, progress_pct,
		       datasource_id, scope_schemas, progress_json,
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
		       datasource_id, scope_schemas, progress_json,
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
	defer func() { _ = rows.Close() }()

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
	return r.UpdateAIJobProgressDetail(ctx, id, status, phase, phaseMessage, progressPct, nil)
}

func (r *Repository) UpdateAIJobProgressDetail(
	ctx context.Context,
	id, status, phase, phaseMessage string,
	progressPct int,
	progressJSON json.RawMessage,
) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE ai_jobs
		SET status = $2, phase = $3, phase_message = $4, progress_pct = $5,
		    progress_json = COALESCE($6::jsonb, progress_json), updated_at = NOW()
		WHERE id = $1`, id, status, phase, phaseMessage, progressPct, nullableRawJSON(progressJSON))
	if err != nil {
		return fmt.Errorf("update ai job progress: %w", err)
	}
	return nil
}

func (r *Repository) FindConflictingDescribeBatch(
	ctx context.Context,
	datasourceID string,
	scopeSchemas []string,
) (*AIJob, error) {
	if datasourceID == "" || len(scopeSchemas) == 0 {
		return nil, nil
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT id, client_session_id, kind, status, phase, phase_message, progress_pct,
		       datasource_id, scope_schemas, progress_json,
		       request_json, result_json, error_message, created_at, updated_at, started_at, finished_at
		FROM ai_jobs
		WHERE kind = 'describe_batch'
		  AND status IN ('pending', 'queued', 'running')
		  AND datasource_id = $1::uuid
		  AND scope_schemas && $2::text[]
		ORDER BY created_at ASC
		LIMIT 1`, datasourceID, pq.Array(scopeSchemas))
	job, err := scanAIJob(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("find conflicting describe batch: %w", err)
	}
	return job, nil
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

func (r *Repository) CancelAIJob(ctx context.Context, id string) (bool, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE ai_jobs
		SET status = $2, phase = 'cancelled', phase_message = 'cancelled by user',
		    progress_pct = 0, finished_at = NOW(), updated_at = NOW()
		WHERE id = $1 AND status IN ($3, $4, $5)`,
		id, AIJobStatusCancelled, AIJobStatusPending, AIJobStatusQueued, AIJobStatusRunning)
	if err != nil {
		return false, fmt.Errorf("cancel ai job: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("cancel ai job rows: %w", err)
	}
	return n > 0, nil
}

func (r *Repository) ListStaleAIJobs(ctx context.Context, sessionID string, olderThan time.Duration, limit int) ([]AIJob, error) {
	if limit <= 0 {
		limit = 100
	}
	cutoff := time.Now().Add(-olderThan)
	q := `
		SELECT id, client_session_id, kind, status, phase, phase_message, progress_pct,
		       datasource_id, scope_schemas, progress_json,
		       request_json, result_json, error_message, created_at, updated_at, started_at, finished_at
		FROM ai_jobs
		WHERE status IN ($1, $2, $3) AND updated_at < $4`
	args := []any{AIJobStatusPending, AIJobStatusQueued, AIJobStatusRunning, cutoff}
	if sessionID != "" {
		q += ` AND client_session_id = $5 ORDER BY updated_at ASC LIMIT $6`
		args = append(args, sessionID, limit)
	} else {
		q += ` ORDER BY updated_at ASC LIMIT $5`
		args = append(args, limit)
	}

	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list stale ai jobs: %w", err)
	}
	defer func() { _ = rows.Close() }()

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

func (r *Repository) CancelAIJobs(ctx context.Context, ids []string) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE ai_jobs
		SET status = $1, phase = 'cancelled', phase_message = 'cancelled by user',
		    progress_pct = 0, finished_at = NOW(), updated_at = NOW()
		WHERE id = ANY($2) AND status IN ($3, $4, $5)`,
		AIJobStatusCancelled, pq.Array(ids), AIJobStatusPending, AIJobStatusQueued, AIJobStatusRunning)
	if err != nil {
		return 0, fmt.Errorf("cancel ai jobs: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("cancel ai jobs rows: %w", err)
	}
	return int(n), nil
}

func (r *Repository) CancelActiveAIJobsBySession(ctx context.Context, sessionID string) (int, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE ai_jobs
		SET status = $1, phase = 'cancelled', phase_message = 'cancelled by user',
		    progress_pct = 0, finished_at = NOW(), updated_at = NOW()
		WHERE client_session_id = $2 AND status IN ($3, $4, $5)`,
		AIJobStatusCancelled, sessionID, AIJobStatusPending, AIJobStatusQueued, AIJobStatusRunning)
	if err != nil {
		return 0, fmt.Errorf("cancel active ai jobs: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("cancel active ai jobs rows: %w", err)
	}
	return int(n), nil
}

func (r *Repository) TryMarkAIJobRunning(ctx context.Context, id string) (bool, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE ai_jobs
		SET status = $2, phase = 'routing', phase_message = '', progress_pct = 5,
		    started_at = COALESCE(started_at, NOW()), updated_at = NOW()
		WHERE id = $1 AND status IN ($3, $4)`, id, AIJobStatusRunning, AIJobStatusPending, AIJobStatusQueued)
	if err != nil {
		return false, fmt.Errorf("mark ai job running: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("mark ai job running rows: %w", err)
	}
	return n > 0, nil
}

func nullableRawJSON(raw json.RawMessage) any {
	if len(raw) == 0 {
		return nil
	}
	return raw
}

func scanAIJob(row *sql.Row) (*AIJob, error) {
	var job AIJob
	var result []byte
	var progress []byte
	var errMsg sql.NullString
	var dsID sql.NullString
	var scope pq.StringArray
	var started, finished sql.NullTime
	err := row.Scan(
		&job.ID, &job.ClientSessionID, &job.Kind, &job.Status, &job.Phase, &job.PhaseMessage, &job.ProgressPct,
		&dsID, &scope, &progress,
		&job.RequestJSON, &result, &errMsg, &job.CreatedAt, &job.UpdatedAt, &started, &finished,
	)
	if err != nil {
		return nil, err
	}
	if len(result) > 0 {
		job.ResultJSON = json.RawMessage(result)
	}
	if len(progress) > 0 {
		job.ProgressJSON = json.RawMessage(progress)
	}
	if dsID.Valid {
		s := dsID.String
		job.DatasourceID = &s
	}
	if len(scope) > 0 {
		job.ScopeSchemas = []string(scope)
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
	var progress []byte
	var errMsg sql.NullString
	var dsID sql.NullString
	var scope pq.StringArray
	var started, finished sql.NullTime
	err := rows.Scan(
		&job.ID, &job.ClientSessionID, &job.Kind, &job.Status, &job.Phase, &job.PhaseMessage, &job.ProgressPct,
		&dsID, &scope, &progress,
		&job.RequestJSON, &result, &errMsg, &job.CreatedAt, &job.UpdatedAt, &started, &finished,
	)
	if err != nil {
		return nil, err
	}
	if len(result) > 0 {
		job.ResultJSON = json.RawMessage(result)
	}
	if len(progress) > 0 {
		job.ProgressJSON = json.RawMessage(progress)
	}
	if dsID.Valid {
		s := dsID.String
		job.DatasourceID = &s
	}
	if len(scope) > 0 {
		job.ScopeSchemas = []string(scope)
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
