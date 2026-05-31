package handlers

import (
	"context"
	"log/slog"

	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/ai/routing"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/semantic"
)

func deriveAIOutcome(resp *ai.Response) string {
	if resp == nil {
		return metadata.AIOutcomeFailed
	}
	if resp.NeedsClarification {
		return metadata.AIOutcomeClarification
	}
	if resp.LogicalQuery == nil {
		return metadata.AIOutcomeFailed
	}
	if resp.Confidence >= 0.7 && len(resp.Warnings) == 0 {
		return metadata.AIOutcomeSuccess
	}
	return metadata.AIOutcomePartial
}

func (h *AIHandler) observeAIRequest(
	ctx context.Context,
	req aiQueryRequest,
	model *semantic.SemanticModel,
	routing *routing.TableRoutingResult,
	resp *ai.Response,
	procErr error,
	latencyMs int64,
) *ai.Response {
	if procErr != nil {
		resp = failedAIResponse(procErr)
	}
	if resp == nil {
		resp = failedAIResponse(procErr)
	}
	if resp.LatencyMs == 0 && latencyMs > 0 {
		resp.LatencyMs = int(latencyMs)
	}

	outcome := deriveAIOutcome(resp)
	success := outcome == metadata.AIOutcomeSuccess
	if h.metrics != nil {
		h.metrics.RecordAIRequest(latencyMs, success, resp.RetryCount, resp.NeedsClarification)
		tokensUsed := 0
		if resp.TokenUsage != nil {
			tokensUsed = resp.TokenUsage.Total
		}
		promptBuildMs := int64(0)
		if resp.PromptStats != nil {
			promptBuildMs = resp.PromptStats.PromptBuildDurationMs
		}
		h.metrics.RecordLLMRequest(latencyMs, tokensUsed, promptBuildMs)
	}

	logArgs := []any{
		"datasource_id", req.DatasourceID,
		"outcome", outcome,
		"retry_count", resp.RetryCount,
		"confidence", resp.Confidence,
		"latency_ms", latencyMs,
		"needs_clarification", resp.NeedsClarification,
		"warnings", len(resp.Warnings),
	}
	if model != nil {
		logArgs = append(logArgs, "model", model.Name)
	}
	slog.InfoContext(ctx, "ai query completed", logArgs...)

	h.recordAIHistory(ctx, req, model, routing, resp)
	return resp
}

func enrichAIHistoryEntry(entry *metadata.AIQueryHistoryEntry, resp *ai.Response) {
	if entry == nil || resp == nil {
		return
	}
	entry.OutcomeStatus = deriveAIOutcome(resp)
	entry.RetryCount = resp.RetryCount
	entry.NeedsClarification = resp.NeedsClarification
	if resp.ModelUsed != "" {
		entry.ModelUsed = &resp.ModelUsed
	}
	if resp.LatencyMs > 0 {
		ms := resp.LatencyMs
		entry.LatencyMs = &ms
	}
	if resp.CostUSD > 0 {
		cost := resp.CostUSD
		entry.CostUSD = &cost
	}
	if resp.TokenUsage != nil && resp.TokenUsage.Total > 0 {
		tokens := resp.TokenUsage.Total
		entry.TokenCount = &tokens
	}
}
