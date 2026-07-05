package metadata

import (
	"context"
	"testing"

	"github.com/biqly/biqly/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSavedQueriesInsertListAndRecall(t *testing.T) {
	db := testutil.OpenMetadataDB(t)
	ctx := context.Background()
	repo := NewRepository(db)

	const (
		datasourceID = "00000000-0000-0000-0000-0000000059a1"
		modelID      = "00000000-0000-0000-0000-0000000059a2"
		modelHash    = "sq-model@3"
	)
	testutil.EnsureMetadataTestDatasource(ctx, t, db, datasourceID, "saved-query-test")
	testutil.EnsureMetadataTestSemanticModel(ctx, t, db, modelID, datasourceID, "saved-query-model")
	_, err := db.ExecContext(ctx, `DELETE FROM ai_saved_queries WHERE datasource_id = $1::uuid`, datasourceID)
	require.NoError(t, err)

	// An embedding-bearing grounding example.
	exampleID, err := repo.InsertSavedQuery(ctx, SavedQueryInsert{
		DatasourceID:      datasourceID,
		ModelID:           modelID,
		Question:          "monthly revenue",
		QuestionHash:      QuestionHash("monthly revenue"),
		SQLQuery:          `{"select":[{"type":"metric","name":"revenue"}]}`,
		QuestionEmbedding: []float32{0.1, 0.2, 0.3},
		SemanticModelHash: modelHash,
		Source:            "example",
	})
	require.NoError(t, err)
	require.NotEmpty(t, exampleID)

	// A runnable skill.
	skillID, err := repo.InsertSavedQuery(ctx, SavedQueryInsert{
		DatasourceID: datasourceID,
		ModelID:      modelID,
		Name:         "Top customers",
		Description:  "top customers by revenue",
		Question:     "who are the top customers",
		LogicalQuery: []byte(`{"select":[{"type":"dimension","name":"customer"}]}`),
		Parameters:   []byte(`[{"name":"limit"}]`),
		Tags:         []string{"sales"},
		Source:       "skill",
		Runnable:     true,
		CreatedBy:    "u-1",
	})
	require.NoError(t, err)

	// Runnable-only listing returns just the skill.
	runnable, err := repo.ListSavedQueries(ctx, SavedQueryFilter{DatasourceID: datasourceID, RunnableOnly: true})
	require.NoError(t, err)
	require.Len(t, runnable, 1)
	assert.Equal(t, skillID, runnable[0].ID)
	assert.True(t, runnable[0].Runnable)
	assert.Equal(t, "skill", runnable[0].Source)
	assert.Equal(t, []string{"sales"}, runnable[0].Tags)
	assert.NotEmpty(t, runnable[0].LogicalQuery)

	// Unfiltered listing returns both records.
	all, err := repo.ListSavedQueries(ctx, SavedQueryFilter{DatasourceID: datasourceID})
	require.NoError(t, err)
	assert.Len(t, all, 2)

	// Get + Update the skill.
	got, err := repo.GetSavedQuery(ctx, skillID)
	require.NoError(t, err)
	assert.Equal(t, "Top customers", got.Name)

	require.NoError(t, repo.UpdateSavedQuery(ctx, skillID, SavedQueryUpdate{
		Name:         "Top customers v2",
		Description:  got.Description,
		Question:     got.Question,
		LogicalQuery: got.LogicalQuery,
		Parameters:   got.Parameters,
		Tags:         []string{"sales", "vip"},
		IsActive:     true,
	}))
	got, err = repo.GetSavedQuery(ctx, skillID)
	require.NoError(t, err)
	assert.Equal(t, "Top customers v2", got.Name)
	assert.Equal(t, 2, got.Version)

	// TouchVerified + DatasourceForSavedQuery.
	require.NoError(t, repo.TouchSavedQueryVerified(ctx, skillID))
	ds, err := repo.DatasourceForSavedQuery(ctx, skillID)
	require.NoError(t, err)
	assert.Equal(t, datasourceID, ds)

	// Recall returns only the embedding-bearing example (skills excluded).
	examples, err := repo.ListActiveSavedQueryExamples(ctx, datasourceID, modelID, modelHash, 50)
	require.NoError(t, err)
	require.Len(t, examples, 1)
	assert.Equal(t, exampleID, examples[0].ID)
	assert.Equal(t, "monthly revenue", examples[0].NLQuery)
	assert.Equal(t, []float32{0.1, 0.2, 0.3}, examples[0].QuestionEmbedding)

	// Delete cleans up.
	require.NoError(t, repo.DeleteSavedQuery(ctx, skillID))
	_, err = repo.GetSavedQuery(ctx, skillID)
	assert.ErrorIs(t, err, ErrSavedQueryNotFound)
}

// TestUpsertSavedQueryExampleIsIdempotent verifies the positive-feedback
// dual-write path: re-confirming the same question updates the single example
// row (ON CONFLICT on the example partial index) rather than duplicating recall
// rows, and refreshes the SQL/embedding.
func TestUpsertSavedQueryExampleIsIdempotent(t *testing.T) {
	db := testutil.OpenMetadataDB(t)
	ctx := context.Background()
	repo := NewRepository(db)

	const (
		datasourceID = "00000000-0000-0000-0000-0000000059b1"
		modelID      = "00000000-0000-0000-0000-0000000059b2"
		modelHash    = "sq-upsert@1"
	)
	testutil.EnsureMetadataTestDatasource(ctx, t, db, datasourceID, "saved-query-upsert-test")
	testutil.EnsureMetadataTestSemanticModel(ctx, t, db, modelID, datasourceID, "saved-query-upsert-model")
	_, err := db.ExecContext(ctx, `DELETE FROM ai_saved_queries WHERE datasource_id = $1::uuid`, datasourceID)
	require.NoError(t, err)

	up := ConfirmedQueryUpsert{
		DatasourceID:      datasourceID,
		ModelID:           modelID,
		UserID:            "u-9",
		QuestionHash:      QuestionHash("weekly tweets"),
		NLQuery:           "weekly tweets",
		SQLQuery:          `{"select":[{"type":"metric","name":"count"}]}`,
		SemanticModelHash: modelHash,
		QuestionEmbedding: []float32{0.4, 0.5, 0.6},
	}
	require.NoError(t, repo.UpsertSavedQueryExample(ctx, up))

	// Re-confirm the same question with a refreshed payload.
	up.SQLQuery = `{"select":[{"type":"metric","name":"row_count"}]}`
	up.QuestionEmbedding = []float32{0.7, 0.8, 0.9}
	require.NoError(t, repo.UpsertSavedQueryExample(ctx, up))

	// Exactly one example row, and it recalls (embedding-bearing) with the update.
	examples, err := repo.ListActiveSavedQueryExamples(ctx, datasourceID, modelID, modelHash, 10)
	require.NoError(t, err)
	require.Len(t, examples, 1)
	assert.Equal(t, `{"select":[{"type":"metric","name":"row_count"}]}`, examples[0].SQLQuery)
}
