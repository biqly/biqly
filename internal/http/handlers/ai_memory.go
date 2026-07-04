package handlers

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"

	"github.com/biqly/biqly/internal/ai/memory"
	"github.com/biqly/biqly/internal/ai/prompt"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/semantic"
)

func (h *AIHandler) appendConfirmedFewShot(
	ctx context.Context,
	model *semantic.SemanticModel,
	question string,
	out []prompt.FewShotExample,
) ([]prompt.FewShotExample, int) {
	if model == nil || h.deps == nil || h.deps.MetaRepo == nil {
		return out, 0
	}
	memCfg := h.effectiveMemoryConfig(ctx)
	if !memCfg.RecallEnabled {
		return out, 0
	}
	remaining := min(fewShotLimit-len(out), memCfg.RecallLimit)
	if remaining <= 0 {
		return out, 0
	}
	ctx, span := otel.Tracer("biqly/ai").Start(ctx, "ai.MemoryRecall")
	defer span.End()
	modelHash := memory.SemanticModelHashForModel(model)
	candidates, err := h.deps.MetaRepo.ListActiveConfirmedQueries(ctx, model.DatasourceID, model.ID, modelHash, metadata.ConfirmedQueriesCandidatePool)
	if err != nil {
		span.RecordError(err)
		slog.WarnContext(ctx, "load confirmed few-shot examples failed", "error", err)
		return out, 0
	}
	recalled, hitCount := memory.RecallFewShot(ctx, h.deps.Embedder, candidates, question, remaining)
	span.SetAttributes(
		attribute.Int("ai.memory.candidates", len(candidates)),
		attribute.Int("ai.memory.hits", hitCount),
	)
	if len(recalled) == 0 {
		return out, 0
	}
	if h.metrics != nil {
		h.metrics.RecordMemoryStoreRecall(hitCount)
	}
	return append(out, recalled...), hitCount
}

// loadMemoryFacts returns the caller's durable remembered facts for prompt
// injection, newest first, capped so they cannot crowd out schema context.
func (h *AIHandler) loadMemoryFacts(ctx context.Context) []string {
	if h.deps == nil || h.deps.MetaRepo == nil {
		return nil
	}
	userID := bimw.UserID(ctx)
	if userID == "" {
		return nil
	}
	rows, err := h.deps.MetaRepo.ListMemoryEntries(ctx, bimw.WorkspaceID(ctx), userID)
	if err != nil {
		slog.WarnContext(ctx, "load memory entries failed", "error", err)
		return nil
	}
	limit := min(len(rows), 20)
	facts := make([]string, 0, limit)
	for _, row := range rows[:limit] {
		facts = append(facts, row.Content)
	}
	return facts
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
			if metrics != nil {
				metrics.RecordMemoryStoreConfirmedEmbeddingError()
			}
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
