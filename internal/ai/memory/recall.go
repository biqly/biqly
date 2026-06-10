package memory

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/ai/prompt"
	"github.com/biqly/biqly/internal/ai/routing"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/platform/observability"
	"github.com/biqly/biqly/internal/semantic"
)

type rankedConfirmed struct {
	row   metadata.ConfirmedQueryRow
	score float64
}

// RecallFewShot selects confirmed query pairs for few-shot injection.
func RecallFewShot(
	ctx context.Context,
	embedder ai.Embedder,
	candidates []metadata.ConfirmedQueryRow,
	question string,
	limit int,
) (examples []prompt.FewShotExample, count int) {
	if limit <= 0 || len(candidates) == 0 {
		return nil, 0
	}
	start := time.Now()
	defer func() {
		observability.Default().RecordMemoryRecallLatency(time.Since(start))
		if len(examples) == 0 {
			observability.Default().RecordMemoryRecallMiss()
		}
	}()
	question = trimQuestion(question)
	questionHash := metadata.QuestionHash(question)

	filtered := make([]metadata.ConfirmedQueryRow, 0, len(candidates))
	for _, c := range candidates {
		if question != "" && metadata.QuestionHash(c.NLQuery) == questionHash {
			continue
		}
		filtered = append(filtered, c)
	}
	if len(filtered) == 0 {
		return nil, 0
	}

	ranked := rankConfirmed(ctx, embedder, filtered, question)
	if len(ranked) == 0 {
		return nil, 0
	}
	if len(ranked) > limit {
		ranked = ranked[:limit]
	}
	out := make([]prompt.FewShotExample, 0, len(ranked))
	for _, item := range ranked {
		out = append(out, prompt.FewShotExample{
			Question:     item.row.NLQuery,
			LogicalQuery: item.row.SQLQuery,
		})
	}
	return out, len(out)
}

func rankConfirmed(
	ctx context.Context,
	embedder ai.Embedder,
	rows []metadata.ConfirmedQueryRow,
	question string,
) []rankedConfirmed {
	if question == "" || embedder == nil {
		out := make([]rankedConfirmed, 0, len(rows))
		for _, row := range rows {
			out = append(out, rankedConfirmed{row: row, score: 0})
		}
		return out
	}

	vecs, err := embedder.Embed(observability.ContextWithEmbeddingOperation(ctx, "memory_store"), []string{question})
	if err != nil || len(vecs) == 0 || len(vecs[0]) == 0 {
		out := make([]rankedConfirmed, 0, len(rows))
		for _, row := range rows {
			out = append(out, rankedConfirmed{row: row, score: 0})
		}
		return out
	}
	qVec := vecs[0]

	ranked := make([]rankedConfirmed, 0, len(rows))
	for _, row := range rows {
		score := 0.0
		if len(row.QuestionEmbedding) > 0 {
			score = routing.CosineSimilarity(qVec, row.QuestionEmbedding)
		}
		ranked = append(ranked, rankedConfirmed{row: row, score: score})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score == ranked[j].score {
			return ranked[i].row.NLQuery < ranked[j].row.NLQuery
		}
		return ranked[i].score > ranked[j].score
	})
	return ranked
}

// SemanticModelHashForModel returns the active hash pin for recall filtering.
func SemanticModelHashForModel(model *semantic.SemanticModel) string {
	if model == nil {
		return ""
	}
	return metadata.SemanticModelHash(model.ID, model.Version)
}

func trimQuestion(question string) string {
	return strings.TrimSpace(question)
}
