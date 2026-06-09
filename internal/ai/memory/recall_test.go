package memory

import (
	"context"
	"testing"

	"github.com/biqly/biqly/internal/metadata"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubEmbedder struct {
	vecs [][]float32
}

func (s stubEmbedder) Embed(_ context.Context, texts []string) ([][]float32, error) {
	if len(s.vecs) == 0 {
		out := make([][]float32, len(texts))
		for i := range texts {
			out[i] = []float32{1, 0}
		}
		return out, nil
	}
	return s.vecs, nil
}

func (stubEmbedder) Model() string { return "stub" }

func TestRecallFewShotPrefersSimilarEmbedding(t *testing.T) {
	candidates := []metadata.ConfirmedQueryRow{
		{NLQuery: "orders by region", SQLQuery: `{"select":[]}`, QuestionEmbedding: []float32{0, 1}},
		{NLQuery: "total revenue", SQLQuery: `{"select":["revenue"]}`, QuestionEmbedding: []float32{1, 0}},
	}
	out, hits := RecallFewShot(context.Background(), stubEmbedder{}, candidates, "show total revenue", 1)
	require.Equal(t, 1, hits)
	require.Len(t, out, 1)
	assert.Equal(t, "total revenue", out[0].Question)
}

func TestRecallFewShotSkipsSameQuestion(t *testing.T) {
	candidates := []metadata.ConfirmedQueryRow{
		{NLQuery: "total revenue", SQLQuery: `{"select":["revenue"]}`},
	}
	out, hits := RecallFewShot(context.Background(), nil, candidates, "total revenue", 3)
	assert.Empty(t, out)
	assert.Zero(t, hits)
}

func TestRecallFewShotFallsBackToRecentWithoutEmbedder(t *testing.T) {
	candidates := []metadata.ConfirmedQueryRow{
		{NLQuery: "first", SQLQuery: `{}`},
		{NLQuery: "second", SQLQuery: `{}`},
	}
	out, hits := RecallFewShot(context.Background(), nil, candidates, "unrelated question", 2)
	require.Equal(t, 2, hits)
	require.Len(t, out, 2)
}
