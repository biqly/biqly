package metadata

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	platformdb "github.com/biqly/biqly/internal/platform/db"
)

// ErrAgentRunNotFound is returned when an agent-run id does not exist.
var ErrAgentRunNotFound = errors.New("agent run not found")

// ErrAgentRunAlreadyTerminal is returned when a caller tries to complete a
// run that already reached a terminal state — a terminal result is
// immutable once set (terminal_version only ever advances 0 -> 1).
var ErrAgentRunAlreadyTerminal = errors.New("agent run already reached a terminal state")

// Agent run status values. Non-terminal runs (running, waiting_clarification)
// are resumable across clarification rounds; terminal runs are not.
const (
	AgentRunStatusRunning              = "running"
	AgentRunStatusWaitingClarification = "waiting_clarification"
	AgentRunStatusCompleted            = "completed"
	AgentRunStatusFailed               = "failed"
)

// AgentRunRow is one persisted AI query run. It mirrors the request scope
// (conversation/datasource/model/user) plus the resolved outcome
// (status/confidence/answer). ConversationID and ModelID are empty when the
// underlying column is NULL (ad-hoc run / auto-selected or composite model).
type AgentRunRow struct {
	ID             string
	ConversationID string
	DatasourceID   string
	ModelID        string
	UserID         string
	Question       string
	QuestionHash   string
	Mode           string
	Status         string
	Confidence     float64
	Answer         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// AgentStepRow is one recorded pipeline phase, mirroring ai.RunStep.
type AgentStepRow struct {
	Seq        int
	Kind       string
	Status     string
	Attempt    int
	DurationMs int
	Detail     string
}

// AgentRunInsert is the input for creating an agent run.
type AgentRunInsert struct {
	ConversationID string
	DatasourceID   string
	ModelID        string
	UserID         string
	Question       string
	QuestionHash   string
	Mode           string
	Status         string
	Confidence     float64
	Answer         string
}

const agentRunColumns = `id::text, COALESCE(conversation_id, ''), datasource_id::text,
	COALESCE(model_id::text, ''), user_id, question, question_hash, mode, status,
	confidence, answer, created_at, updated_at`

// CreateAgentRun inserts a new run and returns its generated id.
func (r *Repository) CreateAgentRun(ctx context.Context, in AgentRunInsert) (string, error) {
	mode := in.Mode
	if mode == "" {
		mode = "interactive"
	}
	status := in.Status
	if status == "" {
		status = AgentRunStatusRunning
	}
	var id string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO agent_runs (
			conversation_id, datasource_id, model_id, user_id, question,
			question_hash, mode, status, confidence, answer
		) VALUES (
			$1, $2::uuid, $3, $4, $5,
			$6, $7, $8, $9, $10
		)
		RETURNING id::text
	`,
		platformdb.NullIfEmpty(in.ConversationID),
		in.DatasourceID,
		platformdb.NullUUIDPtr(&in.ModelID),
		in.UserID,
		in.Question,
		in.QuestionHash,
		mode,
		status,
		in.Confidence,
		in.Answer,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("create agent run: %w", err)
	}
	return id, nil
}

// UpdateAgentRunStatus sets the resolved outcome of a run.
func (r *Repository) UpdateAgentRunStatus(ctx context.Context, id, status string, confidence float64, answer string) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE agent_runs
		SET status = $2, confidence = $3, answer = $4, updated_at = now()
		WHERE id = $1::uuid
	`, id, status, confidence, answer)
	if err != nil {
		return fmt.Errorf("update agent run status: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update agent run status affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("agent run %s: %w", id, ErrAgentRunNotFound)
	}
	return nil
}

// ReplaceAgentSteps atomically replaces a run's step timeline (delete + insert)
// so re-persisting after each clarification round or phase leaves exactly one
// ordered set of steps.
func (r *Repository) ReplaceAgentSteps(ctx context.Context, runID string, steps []AgentStepRow) (err error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("replace agent steps begin: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(ctx, `DELETE FROM agent_steps WHERE run_id = $1::uuid`, runID); err != nil {
		return fmt.Errorf("replace agent steps delete: %w", err)
	}
	for _, s := range steps {
		status := s.Status
		if status == "" {
			status = "ok"
		}
		if _, err = tx.ExecContext(ctx, `
			INSERT INTO agent_steps (run_id, seq, kind, status, attempt, duration_ms, detail)
			VALUES ($1::uuid, $2, $3, $4, $5, $6, $7)
		`, runID, s.Seq, s.Kind, status, s.Attempt, s.DurationMs, s.Detail); err != nil {
			return fmt.Errorf("replace agent steps insert: %w", err)
		}
	}
	if err = tx.Commit(); err != nil {
		return fmt.Errorf("replace agent steps commit: %w", err)
	}
	return nil
}

// GetAgentRun returns a run and its ordered steps.
func (r *Repository) GetAgentRun(ctx context.Context, id string) (AgentRunRow, []AgentStepRow, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+agentRunColumns+` FROM agent_runs WHERE id = $1::uuid`, id)
	run, err := scanAgentRunRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AgentRunRow{}, nil, fmt.Errorf("agent run %s: %w", id, ErrAgentRunNotFound)
		}
		return AgentRunRow{}, nil, err
	}
	steps, err := r.listAgentSteps(ctx, id)
	if err != nil {
		return AgentRunRow{}, nil, err
	}
	return run, steps, nil
}

func (r *Repository) listAgentSteps(ctx context.Context, runID string) ([]AgentStepRow, error) {
	return platformdb.QuerySliceErr(ctx, r.db, "list agent steps", `
		SELECT seq, kind, status, attempt, duration_ms, detail
		FROM agent_steps
		WHERE run_id = $1::uuid
		ORDER BY seq ASC
	`, []any{runID}, func(s platformdb.Scanner) (AgentStepRow, error) {
		var step AgentStepRow
		if err := s.Scan(&step.Seq, &step.Kind, &step.Status, &step.Attempt, &step.DurationMs, &step.Detail); err != nil {
			return step, fmt.Errorf("scan agent step: %w", err)
		}
		return step, nil
	})
}

// ListAgentRuns returns a conversation's runs, newest first.
func (r *Repository) ListAgentRuns(ctx context.Context, conversationID string) ([]AgentRunRow, error) {
	if conversationID == "" {
		return nil, nil
	}
	return platformdb.QuerySliceErr(ctx, r.db, "list agent runs",
		`SELECT `+agentRunColumns+` FROM agent_runs WHERE conversation_id = $1 ORDER BY created_at DESC`,
		[]any{conversationID}, scanAgentRunRow)
}

// FindOpenRun returns the most-recent non-terminal (running or
// waiting_clarification) run for a conversation, so a clarification-answer
// request can resume the SAME run instead of creating a new one. When
// questionHash is non-empty it also constrains the match; pass "" to resume any
// open run in the conversation (used when the resolved question text has
// mutated across clarification rounds).
func (r *Repository) FindOpenRun(ctx context.Context, conversationID, questionHash string) (AgentRunRow, bool, error) {
	if conversationID == "" {
		return AgentRunRow{}, false, nil
	}
	row := r.db.QueryRowContext(ctx, `
		SELECT `+agentRunColumns+`
		FROM agent_runs
		WHERE conversation_id = $1
		  AND status IN ('running', 'waiting_clarification')
		  AND ($2 = '' OR question_hash = $2)
		ORDER BY created_at DESC
		LIMIT 1
	`, conversationID, questionHash)
	run, err := scanAgentRunRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AgentRunRow{}, false, nil
		}
		return AgentRunRow{}, false, err
	}
	return run, true, nil
}

// DatasourceForAgentRun resolves a run id to its owning datasource for
// access-control middleware.
func (r *Repository) DatasourceForAgentRun(ctx context.Context, id string) (string, error) {
	var datasourceID string
	err := r.db.QueryRowContext(ctx, `SELECT datasource_id::text FROM agent_runs WHERE id = $1::uuid`, id).Scan(&datasourceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("agent run %s: %w", id, ErrAgentRunNotFound)
		}
		return "", fmt.Errorf("datasource for agent run: %w", err)
	}
	return datasourceID, nil
}

// CreateAgentRunForJob is CreateAgentRun for a job-driven (NATS) run: it
// records job_id so a redelivered message can find the same run instead of
// creating a duplicate. The unique index on job_id makes this call itself
// idempotent — a second insert for the same job_id fails, and the caller
// should fall back to GetAgentRunByJobID.
func (r *Repository) CreateAgentRunForJob(ctx context.Context, jobID string, in AgentRunInsert) (string, error) {
	mode := in.Mode
	if mode == "" {
		mode = "interactive"
	}
	status := in.Status
	if status == "" {
		status = AgentRunStatusRunning
	}
	var id string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO agent_runs (
			job_id, conversation_id, datasource_id, model_id, user_id, question,
			question_hash, mode, status, confidence, answer
		) VALUES (
			$1::uuid, $2, $3::uuid, $4, $5, $6,
			$7, $8, $9, $10, $11
		)
		RETURNING id::text
	`,
		jobID,
		platformdb.NullIfEmpty(in.ConversationID),
		in.DatasourceID,
		platformdb.NullUUIDPtr(&in.ModelID),
		in.UserID,
		in.Question,
		in.QuestionHash,
		mode,
		status,
		in.Confidence,
		in.Answer,
	).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("create agent run for job: %w", err)
	}
	return id, nil
}

// GetAgentRunByJobID returns the run already created for a job, if any.
// Callers use this to make job processing idempotent under NATS redelivery:
// check first, and only call CreateAgentRunForJob when nothing is found.
func (r *Repository) GetAgentRunByJobID(ctx context.Context, jobID string) (AgentRunRow, bool, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+agentRunColumns+` FROM agent_runs WHERE job_id = $1::uuid`, jobID)
	run, err := scanAgentRunRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AgentRunRow{}, false, nil
		}
		return AgentRunRow{}, false, fmt.Errorf("get agent run by job id: %w", err)
	}
	return run, true, nil
}

// SaveAgentRuntimeState persists the resumable planner/tool-loop snapshot
// for a run. Safe to call repeatedly as the loop progresses; each call
// overwrites the prior snapshot with the latest one.
func (r *Repository) SaveAgentRuntimeState(ctx context.Context, runID string, state []byte) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE agent_runs SET runtime_state = $2, updated_at = now()
		WHERE id = $1::uuid
	`, runID, state)
	if err != nil {
		return fmt.Errorf("save agent runtime state: %w", err)
	}
	return nil
}

// LoadAgentRuntimeState returns the persisted runtime-state snapshot for a
// run ("{}" for a run that has not yet taken a step).
func (r *Repository) LoadAgentRuntimeState(ctx context.Context, runID string) ([]byte, error) {
	var raw []byte
	err := r.db.QueryRowContext(ctx, `SELECT runtime_state FROM agent_runs WHERE id = $1::uuid`, runID).Scan(&raw)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("agent run %s: %w", runID, ErrAgentRunNotFound)
		}
		return nil, fmt.Errorf("load agent runtime state: %w", err)
	}
	return raw, nil
}

// MarkAgentRunQueryExecuteStarted flips query_execute_started to true.
// Idempotent — safe to call more than once for the same run.
func (r *Repository) MarkAgentRunQueryExecuteStarted(ctx context.Context, runID string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE agent_runs SET query_execute_started = true, updated_at = now()
		WHERE id = $1::uuid
	`, runID)
	if err != nil {
		return fmt.Errorf("mark agent run query execute started: %w", err)
	}
	return nil
}

// CompleteAgentRunTerminal records a run's terminal outcome and its final
// runtime-state snapshot, advancing terminal_version 0 -> 1. Returns
// ErrAgentRunAlreadyTerminal without modifying anything if the run already
// has a terminal_version > 0 — a terminal result cannot be overwritten.
func (r *Repository) CompleteAgentRunTerminal(
	ctx context.Context,
	runID, status string,
	confidence float64,
	answer string,
	state []byte,
) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE agent_runs
		SET status = $2, confidence = $3, answer = $4, runtime_state = $5,
		    terminal_version = terminal_version + 1, updated_at = now()
		WHERE id = $1::uuid AND terminal_version = 0
	`, runID, status, confidence, answer, state)
	if err != nil {
		return fmt.Errorf("complete agent run terminal: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("complete agent run terminal affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("agent run %s: %w", runID, ErrAgentRunAlreadyTerminal)
	}
	return nil
}

func scanAgentRunRow(s platformdb.Scanner) (AgentRunRow, error) {
	var row AgentRunRow
	if err := s.Scan(
		&row.ID, &row.ConversationID, &row.DatasourceID, &row.ModelID, &row.UserID,
		&row.Question, &row.QuestionHash, &row.Mode, &row.Status, &row.Confidence, &row.Answer,
		&row.CreatedAt, &row.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return row, err
		}
		return row, fmt.Errorf("scan agent run: %w", err)
	}
	return row, nil
}
