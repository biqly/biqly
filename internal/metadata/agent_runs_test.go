package metadata

import (
	"context"
	"testing"

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
