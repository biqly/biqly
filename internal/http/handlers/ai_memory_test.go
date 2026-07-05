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

type memoryMetricsStub struct {
	recallHits int
}

func (*memoryMetricsStub) RecordAIRequest(int64, bool, int, bool)  {}
func (*memoryMetricsStub) RecordAIStep(string, int64)              {}
func (*memoryMetricsStub) RecordLLMRequest(int64, int, int, int64) {}
func (*memoryMetricsStub) RecordAmbiguityAnalysis(int64, string, bool) {
}
func (*memoryMetricsStub) RecordAmbiguityTier(string)                {}
func (*memoryMetricsStub) RecordAmbiguityClarified()                 {}
func (*memoryMetricsStub) RecordAmbiguityRoundCapReached()           {}
func (*memoryMetricsStub) RecordAIRepair(bool, int, []string)        {}
func (*memoryMetricsStub) RecordMemoryStoreConfirmed()               {}
func (m *memoryMetricsStub) RecordMemoryStoreRecall(count int)       { m.recallHits += count }
func (*memoryMetricsStub) RecordMemoryRecallFeedback(bool, string)   {}
func (*memoryMetricsStub) RecordEnrichContextGaps(int)               {}
func (*memoryMetricsStub) RecordEnrichContextApplied(int)            {}
func (*memoryMetricsStub) RecordEnrichContextApplyErrors(int)        {}
func (*memoryMetricsStub) RecordMemoryStoreConfirmedEmbeddingError() {}
func (*memoryMetricsStub) RecordAmbiguityClarificationRound(int)     {}
func (*memoryMetricsStub) RecordAmbiguityResolution(string)          {}
func (*memoryMetricsStub) RecordFeedbackSubmitted(string)            {}

func TestAppendConfirmedFewShotAddsRecalledExamples(t *testing.T) {
	db, state := setupMockDB(t)
	repo := metadata.NewRepository(db)
	ctx := context.Background()
	model := &semantic.SemanticModel{
		ID:           "m-1",
		DatasourceID: "ds-1",
		Version:      2,
	}
	modelHash := metadata.SemanticModelHash(model.ID, model.Version)

	state.execs = []execMock{
		{Pattern: "UPDATE ai_confirmed_queries", RowsAffected: 0},
		{Pattern: "INSERT INTO ai_confirmed_queries", RowsAffected: 1},
	}
	err := repo.UpsertConfirmedQuery(ctx, metadata.ConfirmedQueryUpsert{
		DatasourceID:      "ds-1",
		ModelID:           "m-1",
		QuestionHash:      metadata.QuestionHash("monthly sales"),
		NLQuery:           "monthly sales",
		SQLQuery:          `{"select":[{"type":"metric","name":"revenue"}]}`,
		SemanticModelHash: modelHash,
	})
	require.NoError(t, err)

	state.queries = []queryMock{
		{
			Pattern: "FROM ai_saved_queries",
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
		},
	}

	metrics := &memoryMetricsStub{}
	h := &AIHandler{
		deps:    (&app.Dependencies{MetaRepo: repo}).AIDeps(),
		metrics: metrics,
	}
	out, hits := h.appendConfirmedFewShot(ctx, model, "show monthly sales trend", nil)
	require.Len(t, out, 1)
	assert.Equal(t, "monthly sales", out[0].Question)
	assert.Equal(t, 1, hits)
	assert.Equal(t, 1, metrics.recallHits)
}

// A recall_enabled=false runtime override turns confirmed-query recall off
// without touching the few-shot pipeline.
func TestAppendConfirmedFewShotRespectsRecallDisabled(t *testing.T) {
	h := &AIHandler{deps: (&app.Dependencies{MetaRepo: metadata.NewRepository(nil)}).AIDeps()}
	h.memoryOverridesCache.cached = memoryOverrides{RecallEnabled: new(false)}
	h.memoryOverridesCache.expires = time.Now().Add(time.Minute)

	out, hits := h.appendConfirmedFewShot(context.Background(), &semantic.SemanticModel{ID: "m-1"}, "monthly sales", nil)
	assert.Empty(t, out)
	assert.Zero(t, hits)
}
