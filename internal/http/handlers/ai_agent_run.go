package handlers

import (
	"context"
	"log/slog"

	"github.com/biqly/biqly/internal/ai"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/semantic"
)

// persistAgentRun durably records this question's run + step trace (Agentic
// Runtime A1). It is BEST-EFFORT: any error logs a warning and never fails the
// query. It sets resp.Metadata.RunID so the frontend can fetch the persisted
// run. Call it once per request after resp is fully assembled (with the
// executed result/answer for the run phase, or the clarification for a
// waiting round). Callers pass req AFTER ProcessContext.ApplyToRequest, so
// req.Question is the resolved question and req.ClarificationChoice is cleared;
// req.ClarificationRound still reflects the round the client echoed.
func (h *AIHandler) persistAgentRun(ctx context.Context, req aiQueryRequest, model *semantic.SemanticModel, resp *ai.Response) {
	if h == nil || h.deps == nil || h.deps.MetaRepo == nil || resp == nil {
		return
	}
	// datasource_id is a NOT NULL FK; without it there is nothing to anchor the
	// run to.
	if req.DatasourceID == "" {
		return
	}
	if resp.Metadata == nil {
		resp.Metadata = &ai.AIMetadata{}
	}
	// Idempotent within a request: if a run was already resolved for this resp
	// (e.g. helper invoked twice), reuse it rather than creating a duplicate.
	runID := resp.Metadata.RunID

	question := req.Question
	questionHash := metadata.QuestionHash(question)
	convID := req.ConversationID
	// A clarification-answer request continues the conversation's open run. The
	// resolved question text mutates across rounds, so resume by open status
	// rather than by question hash.
	isContinuation := req.ClarificationRound > 0

	status, confidence, answer := agentRunOutcome(resp)

	if runID == "" {
		lookupHash := questionHash
		if isContinuation {
			lookupHash = ""
		}
		if existing, ok, err := h.deps.MetaRepo.FindOpenRun(ctx, convID, lookupHash); err != nil {
			slog.WarnContext(ctx, "agent run lookup failed", "error", err)
		} else if ok {
			runID = existing.ID
		}
	}

	if runID == "" {
		id, err := h.deps.MetaRepo.CreateAgentRun(ctx, metadata.AgentRunInsert{
			ConversationID: convID,
			DatasourceID:   req.DatasourceID,
			ModelID:        agentRunModelID(req, model),
			UserID:         bimw.UserID(ctx),
			Question:       question,
			QuestionHash:   questionHash,
			Status:         status,
			Confidence:     confidence,
			Answer:         answer,
		})
		if err != nil {
			slog.WarnContext(ctx, "create agent run failed", "error", err)
			return
		}
		runID = id
	} else {
		if err := h.deps.MetaRepo.UpdateAgentRunStatus(ctx, runID, status, confidence, answer); err != nil {
			slog.WarnContext(ctx, "update agent run failed", "error", err)
		}
	}
	resp.Metadata.RunID = runID

	if steps := agentStepsFromResponse(resp); len(steps) > 0 {
		if err := h.deps.MetaRepo.ReplaceAgentSteps(ctx, runID, steps); err != nil {
			slog.WarnContext(ctx, "persist agent steps failed", "error", err)
		}
	}
}

// agentRunOutcome derives the persisted status/confidence/answer from the
// assembled response.
func agentRunOutcome(resp *ai.Response) (status string, confidence float64, answer string) {
	switch {
	case resp.Clarification != nil && resp.Clarification.NeedsClarification:
		status = metadata.AgentRunStatusWaitingClarification
	case resp.Result == nil:
		status = metadata.AgentRunStatusFailed
	default:
		status = metadata.AgentRunStatusCompleted
	}
	if resp.Result != nil {
		confidence = resp.Result.Confidence
		answer = resp.Result.Answer
	}
	return status, confidence, answer
}

// agentRunModelID picks the semantic model id to record, staying FK-safe: never
// a composite (synthetic id), preferring the explicit request model id and
// falling back to the loaded model's real id.
func agentRunModelID(req aiQueryRequest, model *semantic.SemanticModel) string {
	if req.CompositeID != "" {
		return ""
	}
	if req.ModelID != "" {
		return req.ModelID
	}
	if model != nil {
		return model.ID
	}
	return ""
}

func agentStepsFromResponse(resp *ai.Response) []metadata.AgentStepRow {
	if resp.Metadata == nil || len(resp.Metadata.RunSteps) == 0 {
		return nil
	}
	steps := make([]metadata.AgentStepRow, 0, len(resp.Metadata.RunSteps))
	for _, s := range resp.Metadata.RunSteps {
		steps = append(steps, metadata.AgentStepRow{
			Seq:        s.Seq,
			Kind:       s.Kind,
			Status:     s.Status,
			Attempt:    s.Attempt,
			DurationMs: int(s.DurationMs),
			Detail:     s.Detail,
		})
	}
	return steps
}
