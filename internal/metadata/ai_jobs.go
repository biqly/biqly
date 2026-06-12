package metadata

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/biqly/biqly/internal/platform/db/pgarray"
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
	UserID          *string         `json:"user_id,omitempty"`
	Locale          string          `json:"locale,omitempty"`
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
		return errors.New("job is nil")
	}
	scopeSchemas := job.ScopeSchemas
	if scopeSchemas == nil {
		scopeSchemas = []string{}
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO ai_jobs (
			id, client_session_id, kind, status, phase, phase_message, progress_pct,
			datasource_id, scope_schemas, request_json, user_id, locale
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8::uuid, $9, $10::jsonb, $11, $12)`,
		job.ID, job.ClientSessionID, job.Kind, job.Status, job.Phase, job.PhaseMessage, job.ProgressPct,
		job.DatasourceID, pgarray.Strings(scopeSchemas), job.RequestJSON, job.UserID, nullableLocale(job.Locale),
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
		       request_json, result_json, error_message, created_at, updated_at, started_at, finished_at, user_id, locale
		FROM ai_jobs WHERE id = $1`, id)
	return scanAIJob(row)
}

const aiJobsSelect = `
	SELECT id, client_session_id, kind, status, phase, phase_message, progress_pct,
	       datasource_id, scope_schemas, progress_json,
	       request_json, result_json, error_message, created_at, updated_at, started_at, finished_at, user_id, locale
	FROM ai_jobs`

const (
	aiJobsBySessionQuery = aiJobsSelect + ` WHERE client_session_id = $1`
	aiJobsByUserQuery    = aiJobsSelect + ` WHERE user_id = $1`
)

func (r *Repository) ListAIJobsBySession(ctx context.Context, sessionID string, activeOnly bool, limit int) ([]AIJob, error) {
	return r.listAIJobs(ctx, aiJobsBySessionQuery, sessionID, activeOnly, limit)
}

// ListAIJobsByUser returns the user's jobs across every client session, so a
// page refresh (new tab, new browser) can re-attach to work started elsewhere.
func (r *Repository) ListAIJobsByUser(ctx context.Context, userID string, activeOnly bool, limit int) ([]AIJob, error) {
	return r.listAIJobs(ctx, aiJobsByUserQuery, userID, activeOnly, limit)
}

func (r *Repository) listAIJobs(ctx context.Context, baseQuery, value string, activeOnly bool, limit int) ([]AIJob, error) {
	if limit <= 0 {
		limit = 50
	}
	q := baseQuery
	if activeOnly {
		q += ` AND status IN ('pending', 'queued', 'running')`
	}
	q += ` ORDER BY created_at DESC LIMIT $2`
	return r.queryAIJobs(ctx, limit, q, value, limit)
}

// AIJobsAdminFilter narrows the cross-user admin job listing. Empty fields
// match everything.
type AIJobsAdminFilter struct {
	Status string
	Kind   string
	UserID string
	Limit  int
	Offset int
}

// ListAIJobsAdmin returns jobs across all users and sessions for the admin
// panel: active jobs first, newest first within each group.
func (r *Repository) ListAIJobsAdmin(ctx context.Context, f AIJobsAdminFilter) ([]AIJob, error) {
	limit := f.Limit
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	offset := f.Offset
	if offset < 0 {
		offset = 0
	}
	q := aiJobsSelect + `
	WHERE ($1 = '' OR status = $1)
	  AND ($2 = '' OR kind = $2)
	  AND ($3 = '' OR user_id = $3)
	ORDER BY (status IN ('pending', 'queued', 'running')) DESC, created_at DESC
	LIMIT $4 OFFSET $5`
	return r.queryAIJobs(ctx, limit, q, f.Status, f.Kind, f.UserID, limit, offset)
}

// CountAIJobsAdmin returns how many jobs match the admin list filters.
func (r *Repository) CountAIJobsAdmin(ctx context.Context, f AIJobsAdminFilter) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM ai_jobs
		WHERE ($1 = '' OR status = $1)
		  AND ($2 = '' OR kind = $2)
		  AND ($3 = '' OR user_id = $3)`,
		f.Status, f.Kind, f.UserID,
	).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count ai jobs: %w", err)
	}
	return n, nil
}

func (r *Repository) queryAIJobs(ctx context.Context, capacity int, q string, args ...any) ([]AIJob, error) {
	rows, err := r.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, fmt.Errorf("list ai jobs: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			_ = closeErr
		}
	}()

	out := make([]AIJob, 0, capacity)
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
		return nil, nil //nolint:nilnil // optional result
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT id, client_session_id, kind, status, phase, phase_message, progress_pct,
		       datasource_id, scope_schemas, progress_json,
		       request_json, result_json, error_message, created_at, updated_at, started_at, finished_at, user_id, locale
		FROM ai_jobs
		WHERE kind = 'describe_batch'
		  AND status IN ('pending', 'queued', 'running')
		  AND datasource_id = $1::uuid
		  AND scope_schemas && $2::text[]
		ORDER BY created_at ASC
		LIMIT 1`, datasourceID, pgarray.Strings(scopeSchemas))
	job, err := scanAIJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil //nolint:nilnil // optional result
	}
	if err != nil {
		return nil, fmt.Errorf("find conflicting describe batch: %w", err)
	}
	return job, nil
}

// FindConflictingEmbedMetadata returns a currently-active embedding refresh job
// for the same datasource and semantic model scope (modelID).
//
// modelID is compared against request_json.model_id (missing/empty treated as "").
func (r *Repository) FindConflictingEmbedMetadata(
	ctx context.Context,
	datasourceID string,
	modelID string,
) (*AIJob, error) {
	datasourceID = strings.TrimSpace(datasourceID)
	modelID = strings.TrimSpace(modelID)
	if datasourceID == "" {
		return nil, nil //nolint:nilnil // optional result
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT id, client_session_id, kind, status, phase, phase_message, progress_pct,
		       datasource_id, scope_schemas, progress_json,
		       request_json, result_json, error_message, created_at, updated_at, started_at, finished_at, user_id, locale
		FROM ai_jobs
		WHERE kind = 'embed_metadata'
		  AND status IN ('pending', 'queued', 'running')
		  AND datasource_id = $1::uuid
		  AND COALESCE(request_json->>'model_id', '') = $2
		ORDER BY created_at ASC
		LIMIT 1`, datasourceID, modelID)
	job, err := scanAIJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil //nolint:nilnil // optional result
	}
	if err != nil {
		return nil, fmt.Errorf("find conflicting embed metadata: %w", err)
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
		       request_json, result_json, error_message, created_at, updated_at, started_at, finished_at, user_id, locale
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
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			_ = closeErr
		}
	}()

	out := make([]AIJob, 0, limit)
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
		AIJobStatusCancelled, pgarray.Strings(ids), AIJobStatusPending, AIJobStatusQueued, AIJobStatusRunning)
	if err != nil {
		return 0, fmt.Errorf("cancel ai jobs: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("cancel ai jobs rows: %w", err)
	}
	return int(n), nil
}

// CancelAIJobsOwned cancels only the subset of ids the caller owns — matched
// by user_id or, for legacy jobs without one, by client session id. Used by
// the non-admin batch-cancel endpoint so users cannot cancel others' jobs.
func (r *Repository) CancelAIJobsOwned(ctx context.Context, ids []string, userID, sessionID string) (int, error) {
	if len(ids) == 0 || (userID == "" && sessionID == "") {
		return 0, nil
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE ai_jobs
		SET status = $1, phase = 'cancelled', phase_message = 'cancelled by user',
		    progress_pct = 0, finished_at = NOW(), updated_at = NOW()
		WHERE id = ANY($2) AND status IN ($3, $4, $5)
		  AND (($6 <> '' AND user_id = $6) OR ($7 <> '' AND client_session_id = $7))`,
		AIJobStatusCancelled, pgarray.Strings(ids), AIJobStatusPending, AIJobStatusQueued, AIJobStatusRunning,
		userID, sessionID)
	if err != nil {
		return 0, fmt.Errorf("cancel owned ai jobs: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("cancel owned ai jobs rows: %w", err)
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

type AIQueueStatus struct {
	TotalPending int    `json:"total_pending"`
	MyPosition   *int   `json:"my_position,omitempty"`
	MyJobID      string `json:"my_job_id,omitempty"`
	MyJobStatus  string `json:"my_job_status"`
}

// GetAIQueueStatus returns the global pending count plus the caller's position
// in the queue. Position is 1-based; nil means the caller has no job in queue.
func (r *Repository) GetAIQueueStatus(ctx context.Context, sessionID string) (*AIQueueStatus, error) {
	status := &AIQueueStatus{MyJobStatus: "idle"}

	if sessionID == "" {
		err := r.db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM ai_jobs WHERE status IN ($1, $2)
		`, AIJobStatusPending, AIJobStatusQueued).Scan(&status.TotalPending)
		if err != nil {
			return nil, fmt.Errorf("count pending: %w", err)
		}
		return status, nil
	}

	var myJobID sql.NullString
	var myStatus sql.NullString
	var position sql.NullInt64
	err := r.db.QueryRowContext(ctx, `
		WITH my_job AS (
			SELECT id, status, created_at FROM ai_jobs
			WHERE client_session_id = $1
			  AND status IN ($2, $3, $4)
			ORDER BY created_at ASC
			LIMIT 1
		),
		pending_count AS (
			SELECT COUNT(*) AS total_pending
			FROM ai_jobs
			WHERE status IN ($2, $3)
		)
		SELECT pending_count.total_pending,
		       my_job.id,
		       my_job.status,
		       CASE
		           WHEN my_job.status IN ($2, $3) THEN (
		               SELECT COUNT(*) + 1
		               FROM ai_jobs
		               WHERE status IN ($2, $3)
		                 AND created_at < my_job.created_at
		           )
		       END AS position
		FROM pending_count
		LEFT JOIN my_job ON TRUE
	`, sessionID, AIJobStatusPending, AIJobStatusQueued, AIJobStatusRunning).Scan(&status.TotalPending, &myJobID, &myStatus, &position)
	if err != nil {
		return nil, fmt.Errorf("get ai queue status: %w", err)
	}

	if !myJobID.Valid {
		return status, nil
	}
	status.MyJobID = myJobID.String
	status.MyJobStatus = myStatus.String
	if position.Valid {
		pos := int(position.Int64)
		status.MyPosition = new(pos)
	}

	return status, nil
}

func (r *Repository) TryMarkAIJobRunning(ctx context.Context, id string) (bool, error) {
	res, err := r.db.ExecContext(ctx, `
		UPDATE ai_jobs
		SET status = $2, phase = 'routing', phase_message = '', progress_pct = 5,
		    started_at = COALESCE(started_at, NOW()), updated_at = NOW()
		WHERE id = $1 AND status IN ($3, $4, $2)`, id, AIJobStatusRunning, AIJobStatusPending, AIJobStatusQueued)
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

func nullableLocale(locale string) any {
	if strings.TrimSpace(locale) == "" {
		return nil
	}
	return strings.TrimSpace(locale)
}

type aiJobScanner interface {
	Scan(dest ...any) error
}

func scanAIJobFromScanner(scanner aiJobScanner) (*AIJob, error) {
	var job AIJob
	var result []byte
	var progress []byte
	var errMsg sql.NullString
	var dsID sql.NullString
	var scope pgarray.StringArray
	var started, finished sql.NullTime
	var userID sql.NullString
	var locale sql.NullString
	err := scanner.Scan(
		&job.ID, &job.ClientSessionID, &job.Kind, &job.Status, &job.Phase, &job.PhaseMessage, &job.ProgressPct,
		&dsID, &scope, &progress,
		&job.RequestJSON, &result, &errMsg, &job.CreatedAt, &job.UpdatedAt, &started, &finished, &userID, &locale,
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
		job.DatasourceID = new(dsID.String)
	}
	if len(scope) > 0 {
		job.ScopeSchemas = []string(scope)
	}
	if errMsg.Valid {
		job.ErrorMessage = errMsg.String
	}
	if started.Valid {
		job.StartedAt = new(started.Time)
	}
	if finished.Valid {
		job.FinishedAt = new(finished.Time)
	}
	if userID.Valid {
		job.UserID = new(userID.String)
	}
	if locale.Valid {
		job.Locale = locale.String
	}
	return &job, nil
}

func scanAIJob(row *sql.Row) (*AIJob, error) {
	return scanAIJobFromScanner(row)
}

func scanAIJobRows(rows aiJobScanner) (*AIJob, error) {
	return scanAIJobFromScanner(rows)
}
