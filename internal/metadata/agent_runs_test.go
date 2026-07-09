package metadata

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"

	"github.com/biqly/biqly/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAgentRunsLifecycle(t *testing.T) {
	db := testutil.OpenMetadataDB(t)
	ctx := context.Background()
	repo := NewRepository(db)

	const (
		datasourceID = "00000000-0000-0000-0000-0000000061a1"
		modelID      = "00000000-0000-0000-0000-0000000061a2"
		userID       = "u-agent-run"
	)
	testutil.EnsureMetadataTestDatasource(ctx, t, db, datasourceID, "agent-run-test")
	testutil.EnsureMetadataTestSemanticModel(ctx, t, db, modelID, datasourceID, "agent-run-model")

	// A persisted conversation to anchor the run to.
	conv := &AIConversation{UserID: userID, DatasourceID: datasourceID, ModelID: new(modelID), ContextEnabled: true}
	require.NoError(t, repo.CreateAIConversation(ctx, conv))
	require.NotEmpty(t, conv.ID)

	// Create → Get.
	runID, err := repo.CreateAgentRun(ctx, AgentRunInsert{
		ConversationID: conv.ID,
		DatasourceID:   datasourceID,
		ModelID:        modelID,
		UserID:         userID,
		Question:       "monthly revenue",
		QuestionHash:   QuestionHash("monthly revenue"),
		Status:         AgentRunStatusRunning,
	})
	require.NoError(t, err)
	require.NotEmpty(t, runID)

	run, steps, err := repo.GetAgentRun(ctx, runID)
	require.NoError(t, err)
	assert.Equal(t, conv.ID, run.ConversationID)
	assert.Equal(t, datasourceID, run.DatasourceID)
	assert.Equal(t, modelID, run.ModelID)
	assert.Equal(t, AgentRunStatusRunning, run.Status)
	assert.Equal(t, "interactive", run.Mode)
	assert.Empty(t, steps)

	// ReplaceAgentSteps writes an ordered timeline.
	require.NoError(t, repo.ReplaceAgentSteps(ctx, runID, []AgentStepRow{
		{Seq: 1, Kind: "table_route", Status: "ok", DurationMs: 12},
		{Seq: 2, Kind: "llm_generate", Status: "failed", Attempt: 1, DurationMs: 340, Detail: "timeout"},
	}))
	_, steps, err = repo.GetAgentRun(ctx, runID)
	require.NoError(t, err)
	require.Len(t, steps, 2)
	assert.Equal(t, "table_route", steps[0].Kind)
	assert.Equal(t, "llm_generate", steps[1].Kind)
	assert.Equal(t, "failed", steps[1].Status)
	assert.Equal(t, 1, steps[1].Attempt)
	assert.Equal(t, "timeout", steps[1].Detail)

	// Replacing again fully supersedes the prior set.
	require.NoError(t, repo.ReplaceAgentSteps(ctx, runID, []AgentStepRow{
		{Seq: 1, Kind: "sql_dry_run", Status: "ok", DurationMs: 5},
	}))
	_, steps, err = repo.GetAgentRun(ctx, runID)
	require.NoError(t, err)
	require.Len(t, steps, 1)
	assert.Equal(t, "sql_dry_run", steps[0].Kind)

	// FindOpenRun sees a running run (empty hash matches any open run).
	open, ok, err := repo.FindOpenRun(ctx, conv.ID, "")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, runID, open.ID)

	// FindOpenRun with matching hash also resolves it.
	open, ok, err = repo.FindOpenRun(ctx, conv.ID, QuestionHash("monthly revenue"))
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, runID, open.ID)

	// UpdateAgentRunStatus records the resolved outcome and closes the run.
	require.NoError(t, repo.UpdateAgentRunStatus(ctx, runID, AgentRunStatusCompleted, 0.87, "Revenue was $1.2M."))
	run, _, err = repo.GetAgentRun(ctx, runID)
	require.NoError(t, err)
	assert.Equal(t, AgentRunStatusCompleted, run.Status)
	assert.InDelta(t, 0.87, run.Confidence, 1e-9)
	assert.Equal(t, "Revenue was $1.2M.", run.Answer)

	// A completed run is terminal: FindOpenRun no longer returns it.
	_, ok, err = repo.FindOpenRun(ctx, conv.ID, "")
	require.NoError(t, err)
	assert.False(t, ok)

	// ListAgentRuns returns the conversation's runs.
	runs, err := repo.ListAgentRuns(ctx, conv.ID)
	require.NoError(t, err)
	require.Len(t, runs, 1)
	assert.Equal(t, runID, runs[0].ID)

	// DatasourceForAgentRun resolves access control.
	ds, err := repo.DatasourceForAgentRun(ctx, runID)
	require.NoError(t, err)
	assert.Equal(t, datasourceID, ds)

	// ConversationBelongsToUser gates the list endpoint.
	owned, err := repo.ConversationBelongsToUser(ctx, conv.ID, userID)
	require.NoError(t, err)
	assert.True(t, owned)
	owned, err = repo.ConversationBelongsToUser(ctx, conv.ID, "someone-else")
	require.NoError(t, err)
	assert.False(t, owned)

	// Unknown id surfaces the sentinel.
	_, _, err = repo.GetAgentRun(ctx, "00000000-0000-0000-0000-0000000061ff")
	assert.ErrorIs(t, err, ErrAgentRunNotFound)
}

// TestAgentRunJobIdempotency verifies job_id-keyed creation is redelivery-safe:
// a second CreateAgentRunForJob for the same job_id fails on the unique index,
// and GetAgentRunByJobID lets the caller find the existing run instead.
func TestAgentRunJobIdempotency(t *testing.T) {
	db := testutil.OpenMetadataDB(t)
	ctx := context.Background()
	repo := NewRepository(db)

	const (
		datasourceID = "00000000-0000-0000-0000-0000000065a1"
		userID       = "u-agent-job"
		jobID        = "00000000-0000-0000-0000-0000000065aa"
	)
	testutil.EnsureMetadataTestDatasource(ctx, t, db, datasourceID, "agent-job-test")

	// job_id is uniquely constrained; clean up so reruns against a
	// persistent dev DB don't collide with a previous run's row.
	cleanup := func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM agent_runs WHERE job_id = $1::uuid`, jobID)
	}
	cleanup()
	t.Cleanup(cleanup)

	runID, err := repo.CreateAgentRunForJob(ctx, jobID, AgentRunInsert{
		DatasourceID: datasourceID,
		UserID:       userID,
		Question:     "job-driven question",
		QuestionHash: QuestionHash("job-driven question"),
	})
	require.NoError(t, err)
	require.NotEmpty(t, runID)

	// Redelivery: the second create attempt for the same job must fail...
	_, err = repo.CreateAgentRunForJob(ctx, jobID, AgentRunInsert{
		DatasourceID: datasourceID,
		UserID:       userID,
		Question:     "job-driven question",
	})
	assert.Error(t, err)

	// ...and the caller resumes via GetAgentRunByJobID instead of duplicating.
	found, ok, err := repo.GetAgentRunByJobID(ctx, jobID)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, runID, found.ID)

	// An unrelated job_id has no run yet.
	_, ok, err = repo.GetAgentRunByJobID(ctx, "00000000-0000-0000-0000-0000000065ab")
	require.NoError(t, err)
	assert.False(t, ok)
}

// TestAgentRunRuntimeStateAndTerminalImmutability verifies the resumable
// runtime-state snapshot round-trips, query_execute_started is idempotent,
// and a terminal result cannot be overwritten once set.
func TestAgentRunRuntimeStateAndTerminalImmutability(t *testing.T) {
	db := testutil.OpenMetadataDB(t)
	ctx := context.Background()
	repo := NewRepository(db)

	const (
		datasourceID = "00000000-0000-0000-0000-0000000065b1"
		userID       = "u-agent-runtime"
	)
	testutil.EnsureMetadataTestDatasource(ctx, t, db, datasourceID, "agent-runtime-test")

	runID, err := repo.CreateAgentRun(ctx, AgentRunInsert{
		DatasourceID: datasourceID,
		UserID:       userID,
		Question:     "runtime state question",
		QuestionHash: QuestionHash("runtime state question"),
	})
	require.NoError(t, err)

	// A fresh run has the migration's default empty-object snapshot.
	state, err := repo.LoadAgentRuntimeState(ctx, runID)
	require.NoError(t, err)
	assert.JSONEq(t, `{}`, string(state))

	// Save persists a snapshot; Load round-trips it exactly.
	require.NoError(t, repo.SaveAgentRuntimeState(ctx, runID, []byte(`{"steps":[{"seq":1}]}`)))
	state, err = repo.LoadAgentRuntimeState(ctx, runID)
	require.NoError(t, err)
	assert.JSONEq(t, `{"steps":[{"seq":1}]}`, string(state))

	// Marking query_execute_started is idempotent (no error calling it twice).
	require.NoError(t, repo.MarkAgentRunQueryExecuteStarted(ctx, runID))
	require.NoError(t, repo.MarkAgentRunQueryExecuteStarted(ctx, runID))

	// First terminal completion succeeds and records the outcome.
	require.NoError(t, repo.CompleteAgentRunTerminal(ctx, runID, AgentRunStatusCompleted, 0.95, "42",
		[]byte(`{"steps":[{"seq":1}],"terminal":{"kind":"final"}}`)))
	run, _, err := repo.GetAgentRun(ctx, runID)
	require.NoError(t, err)
	assert.Equal(t, AgentRunStatusCompleted, run.Status)
	assert.InDelta(t, 0.95, run.Confidence, 1e-9)
	assert.Equal(t, "42", run.Answer)

	// A second terminal completion attempt is rejected: the result is immutable.
	err = repo.CompleteAgentRunTerminal(ctx, runID, AgentRunStatusFailed, 0, "should not apply", []byte(`{}`))
	assert.ErrorIs(t, err, ErrAgentRunAlreadyTerminal)

	// The rejected attempt did not overwrite the original outcome.
	run, _, err = repo.GetAgentRun(ctx, runID)
	require.NoError(t, err)
	assert.Equal(t, AgentRunStatusCompleted, run.Status)
	assert.Equal(t, "42", run.Answer)

	// Load/save on an unknown run surfaces the not-found sentinel.
	_, err = repo.LoadAgentRuntimeState(ctx, "00000000-0000-0000-0000-0000000065ff")
	assert.ErrorIs(t, err, ErrAgentRunNotFound)
}

// TestRecordShadowComparison verifies inserts succeed with both runs
// present, and with either side empty (a run that never got persisted).
func TestRecordShadowComparison(t *testing.T) {
	db := testutil.OpenMetadataDB(t)
	ctx := context.Background()
	repo := NewRepository(db)

	const (
		datasourceID = "00000000-0000-0000-0000-0000000065c1"
		userID       = "u-agent-shadow"
		jobID        = "00000000-0000-0000-0000-0000000065cc"
	)
	testutil.EnsureMetadataTestDatasource(ctx, t, db, datasourceID, "agent-shadow-test")
	cleanup := func() {
		_, _ = db.ExecContext(ctx, `DELETE FROM agent_shadow_comparisons WHERE job_id = $1::uuid`, jobID)
	}
	cleanup()
	t.Cleanup(cleanup)

	legacyRunID, err := repo.CreateAgentRun(ctx, AgentRunInsert{
		DatasourceID: datasourceID, UserID: userID, Question: "q",
	})
	require.NoError(t, err)
	agentRunID, err := repo.CreateAgentRun(ctx, AgentRunInsert{
		DatasourceID: datasourceID, UserID: userID, Question: "q",
	})
	require.NoError(t, err)

	require.NoError(t, repo.RecordShadowComparison(ctx, jobID, legacyRunID, agentRunID, "match", []byte(`{}`)))
	require.NoError(t, repo.RecordShadowComparison(ctx, jobID, legacyRunID, agentRunID, "result_mismatch", []byte(`{"note":"x"}`)))
	// A comparison can be recorded even when one side never got a persisted run.
	require.NoError(t, repo.RecordShadowComparison(ctx, jobID, "", agentRunID, "legacy_only_failure", []byte(`{}`)))

	var count int
	require.NoError(t, db.QueryRowContext(ctx,
		`SELECT count(*) FROM agent_shadow_comparisons WHERE job_id = $1::uuid`, jobID).Scan(&count))
	assert.Equal(t, 3, count)
}

// TestFindOpenRunResumesAcrossClarification verifies the resume key: a run left
// waiting_clarification is resumable by the conversation's most-recent open run
// even when the resolved question text (and thus its hash) has changed.
func TestFindOpenRunResumesAcrossClarification(t *testing.T) {
	db := testutil.OpenMetadataDB(t)
	ctx := context.Background()
	repo := NewRepository(db)

	const (
		datasourceID = "00000000-0000-0000-0000-0000000061b1"
		userID       = "u-agent-clarify"
	)
	testutil.EnsureMetadataTestDatasource(ctx, t, db, datasourceID, "agent-run-clarify")
	conv := &AIConversation{UserID: userID, DatasourceID: datasourceID, ContextEnabled: true}
	require.NoError(t, repo.CreateAIConversation(ctx, conv))

	runID, err := repo.CreateAgentRun(ctx, AgentRunInsert{
		ConversationID: conv.ID,
		DatasourceID:   datasourceID,
		UserID:         userID,
		Question:       "which revenue?",
		QuestionHash:   QuestionHash("which revenue?"),
		Status:         AgentRunStatusWaitingClarification,
	})
	require.NoError(t, err)

	// A follow-up round keys by "" (resolved question mutated) and resumes it.
	open, ok, err := repo.FindOpenRun(ctx, conv.ID, "")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, runID, open.ID)
	assert.Equal(t, AgentRunStatusWaitingClarification, open.Status)
}

func TestIsForeignKeyViolation(t *testing.T) {
	fk := &pgconn.PgError{Code: "23503", ConstraintName: "agent_runs_conversation_id_fkey"}

	assert.True(t, IsForeignKeyViolation(fk, "agent_runs_conversation_id_fkey"))
	// Wrapped the way repository methods return it.
	assert.True(t, IsForeignKeyViolation(
		fmt.Errorf("create agent run: %w", fk), "agent_runs_conversation_id_fkey"))

	assert.False(t, IsForeignKeyViolation(fk, "agent_runs_datasource_id_fkey"), "different constraint")
	assert.False(t, IsForeignKeyViolation(
		&pgconn.PgError{Code: "23505", ConstraintName: "agent_runs_conversation_id_fkey"},
		"agent_runs_conversation_id_fkey"), "unique violation is not an FK violation")
	assert.False(t, IsForeignKeyViolation(errors.New("plain error"), "agent_runs_conversation_id_fkey"))
	assert.False(t, IsForeignKeyViolation(nil, "agent_runs_conversation_id_fkey"))
}
