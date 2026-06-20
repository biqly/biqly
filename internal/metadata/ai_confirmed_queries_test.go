package metadata

import (
	"context"
	"strconv"
	"sync"
	"testing"

	"github.com/biqly/biqly/internal/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQuestionHashNormalizesCaseAndWhitespace(t *testing.T) {
	a := QuestionHash("  Total Revenue  ")
	b := QuestionHash("total revenue")
	assert.Equal(t, a, b)
}

func TestSemanticModelHashIncludesVersion(t *testing.T) {
	assert.Equal(t, "model-1@3", SemanticModelHash("model-1", 3))
	assert.NotEqual(t, SemanticModelHash("model-1", 2), SemanticModelHash("model-1", 3))
}

func TestUpsertConfirmedQueryConcurrentSameKey(t *testing.T) {
	db := testutil.OpenMetadataDB(t)
	ctx := context.Background()
	repo := NewRepository(db)

	const (
		datasourceID      = "00000000-0000-0000-0000-00000000c001"
		modelID           = "00000000-0000-0000-0000-00000000c002"
		semanticModelHash = "model@7"
	)
	questionHash := QuestionHash("Total revenue by month")
	_, err := db.ExecContext(ctx, `
		DELETE FROM ai_confirmed_queries
		WHERE datasource_id = $1::uuid
		  AND question_hash = $2
		  AND semantic_model_hash = $3
	`, datasourceID, questionHash, semanticModelHash)
	require.NoError(t, err)

	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for i := range 16 {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			errs <- repo.UpsertConfirmedQuery(ctx, ConfirmedQueryUpsert{
				DatasourceID:      datasourceID,
				ModelID:           modelID,
				QuestionHash:      questionHash,
				NLQuery:           "total revenue by month",
				SQLQuery:          "select col_" + strconv.Itoa(i),
				SemanticModelHash: semanticModelHash,
				QuestionEmbedding: []float32{1, 2, 3},
			})
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	var count int
	err = db.QueryRowContext(ctx, `
		SELECT COUNT(*)::int
		FROM ai_confirmed_queries
		WHERE datasource_id = $1::uuid
		  AND model_id = $2::uuid
		  AND question_hash = $3
		  AND semantic_model_hash = $4
	`, datasourceID, modelID, questionHash, semanticModelHash).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
}
