package handlers

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"github.com/biqly/biqly/internal/ai/memory"
	"github.com/biqly/biqly/internal/ai/prompt"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/semantic"
)

func (h *AIHandler) appendConfirmedFewShot(
	ctx context.Context,
	model *semantic.SemanticModel,
	question string,
	out []prompt.FewShotExample,
) []prompt.FewShotExample {
	if model == nil || h.deps == nil || h.deps.MetaRepo == nil {
		return out
	}
	remaining := fewShotLimit - len(out)
	if remaining <= 0 {
		return out
	}
	modelHash := memory.SemanticModelHashForModel(model)
	candidates, err := h.deps.MetaRepo.ListActiveConfirmedQueries(ctx, model.DatasourceID, model.ID, modelHash, metadata.ConfirmedQueriesCandidatePool)
	if err != nil {
		slog.WarnContext(ctx, "load confirmed few-shot examples failed", "error", err)
		return out
	}
	recalled, hitCount := memory.RecallFewShot(ctx, h.deps.Embedder, candidates, question, remaining)
	if len(recalled) == 0 {
		return out
	}
	if h.metrics != nil {
		h.metrics.RecordMemoryStoreRecall(hitCount)
	}
	return append(out, recalled...)
}

// storeConfirmedQueryOnPositiveFeedback persists the NL→SQL pair behind a
// thumbs-up. It reports whether the pair was actually stored so the response
// can tell the user the query was learned.
func (h *AIExamplesHandler) storeConfirmedQueryOnPositiveFeedback(
	ctx context.Context,
	datasourceID, userID, question string,
	metrics AIMetricsRecorder,
) bool {
	if h.deps == nil || h.deps.MetaRepo == nil || userID == "" {
		return false
	}
	history, err := h.deps.MetaRepo.GetLatestAIQueryHistoryForFeedback(ctx, datasourceID, userID, question)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			slog.WarnContext(ctx, "load AI history for confirmed query", "error", err)
		}
		return false
	}
	if len(history.LogicalQuery) == 0 {
		return false
	}
	modelHash := ""
	modelID := history.ModelID
	if modelID != "" && h.deps.SemanticRepo != nil {
		model, err := h.deps.SemanticRepo.GetModel(ctx, modelID)
		if err != nil {
			slog.WarnContext(ctx, "load semantic model for confirmed query", "model_id", modelID, "error", err)
		} else if model != nil {
			modelHash = memory.SemanticModelHashForModel(model)
		}
	}
	if modelHash == "" && modelID != "" {
		modelHash = metadata.SemanticModelHash(modelID, 0)
	}

	var embedding []float32
	if h.deps.Embedder != nil {
		vecs, embedErr := h.deps.Embedder.Embed(ctx, []string{question})
		if embedErr != nil {
			slog.WarnContext(ctx, "embed confirmed query question", "error", embedErr)
		} else if len(vecs) > 0 {
			embedding = vecs[0]
		}
	}

	err = h.deps.MetaRepo.UpsertConfirmedQuery(ctx, metadata.ConfirmedQueryUpsert{
		DatasourceID:      datasourceID,
		ModelID:           modelID,
		UserID:            userID,
		QuestionHash:      metadata.QuestionHash(question),
		NLQuery:           question,
		SQLQuery:          string(history.LogicalQuery),
		SemanticModelHash: modelHash,
		QuestionEmbedding: embedding,
	})
	if err != nil {
		slog.WarnContext(ctx, "store confirmed query", "error", err)
		return false
	}
	if metrics != nil {
		metrics.RecordMemoryStoreConfirmed()
	}
	return true
}
