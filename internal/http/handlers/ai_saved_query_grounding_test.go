package handlers

import (
	"context"
	"database/sql/driver"
	"testing"
	"time"

	"github.com/biqly/biqly/internal/app"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/semantic"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// recallExampleMock returns a queryMock for the embedding-RAG recall query
// (ListActiveSavedQueryExamples) yielding a single confirmed example.
func recallExampleMock(modelHash string) queryMock {
	return queryMock{
		Pattern: "question_embedding IS NOT NULL",
		Cols: []string{
			"id", "datasource_id", "model_id", "user_id", "question_hash", "nl_query",
			"sql_query", "semantic_model_hash", "question_embedding", "is_active",
		},
		Rows: [][]driver.Value{
			{
				"cq-1", "ds-1", "m-1", "u-1", metadata.QuestionHash("monthly sales"),
				"monthly sales", `{"select":[{"type":"metric","name":"revenue"}]}`, modelHash, nil, true,
			},
		},
	}
}

// emptyGroundingSideMocks return empty curated/history queries so only the
// source under test contributes examples.
func emptyGroundingSideMocks() []queryMock {
	return []queryMock{
		{Pattern: "FROM few_shot_examples", Cols: []string{"x"}},
		{Pattern: "FROM ai_query_history", Cols: []string{"question", "logical_query"}},
	}
}

// auto_find_skills=false must skip the embedding-RAG recall
// (appendConfirmedFewShot); the default (true) keeps it.
func TestLoadFewShotExamplesAutoFindGate(t *testing.T) {
	db, state := setupMockDB(t)
	repo := metadata.NewRepository(db)
	ctx := context.Background()
	model := &semantic.SemanticModel{ID: "m-1", DatasourceID: "ds-1", Version: 2}
	modelHash := metadata.SemanticModelHash(model.ID, model.Version)

	state.queries = append(emptyGroundingSideMocks(), recallExampleMock(modelHash))

	h := &AIHandler{
		deps:    (&app.Dependencies{MetaRepo: repo}).AIDeps(),
		metrics: &memoryMetricsStub{},
	}

	// auto-find ON → recalled example is injected (current default behavior).
	out, hits := h.loadFewShotExamples(ctx, model, "show monthly sales trend", true, nil)
	require.Len(t, out, 1)
	assert.Equal(t, "monthly sales", out[0].Question)
	assert.Equal(t, 1, hits)

	// auto-find OFF → recall skipped entirely.
	out, hits = h.loadFewShotExamples(ctx, model, "show monthly sales trend", false, nil)
	assert.Empty(t, out)
	assert.Zero(t, hits)
}

// saved_query_ids must inject the selected saved queries as few-shot grounding,
// independent of the auto-find recall.
func TestLoadFewShotExamplesInjectsSavedQueryIDs(t *testing.T) {
	db, state := setupMockDB(t)
	repo := metadata.NewRepository(db)
	ctx := context.Background()
	model := &semantic.SemanticModel{ID: "m-1", DatasourceID: "ds-1", Version: 2}

	now := time.Now()
	state.queries = append(emptyGroundingSideMocks(), queryMock{
		Pattern: "= ANY($2::uuid[])",
		Cols: []string{
			"id", "datasource_id", "model_id", "name", "description", "question",
			"question_hash", "sql_query", "logical_query", "parameters", "question_embedding",
			"semantic_model_hash", "tags", "source", "runnable", "is_active", "created_by",
			"version", "last_verified_at", "created_at", "updated_at",
		},
		Rows: [][]driver.Value{
			{
				"sq-1", "ds-1", "m-1", "Top customers", "top customers", "who are the top customers",
				metadata.QuestionHash("who are the top customers"),
				`{"select":[{"type":"dimension","name":"customer"}]}`, nil, nil, nil,
				"hash", nil, "skill", true, true, "u-1",
				int64(1), nil, now, now,
			},
		},
	})

	h := &AIHandler{
		deps:    (&app.Dependencies{MetaRepo: repo}).AIDeps(),
		metrics: &memoryMetricsStub{},
	}

	// auto-find OFF so the only grounding is the explicitly selected saved query.
	out, hits := h.loadFewShotExamples(ctx, model, "who buys the most", false, []string{"sq-1"})
	require.Len(t, out, 1)
	assert.Equal(t, "who are the top customers", out[0].Question)
	assert.Equal(t, `{"select":[{"type":"dimension","name":"customer"}]}`, out[0].LogicalQuery)
	assert.Zero(t, hits)
}
