package handlers

import (
	"context"
	"log/slog"

	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/ai/routing"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
)

func deriveAIOutcome(resp *ai.Response) string {
	if resp == nil {
		return metadata.AIOutcomeFailed
	}
	needsClarification := resp.Clarification != nil && resp.Clarification.NeedsClarification
	if needsClarification {
		return metadata.AIOutcomeClarification
	}
	var logicalQuery *query.LogicalQuery
	var confidence float64
	var warnings []string
	if resp.Result != nil {
		logicalQuery = resp.Result.LogicalQuery
		confidence = resp.Result.Confidence
		warnings = resp.Result.Warnings
	}
	if logicalQuery == nil {
		return metadata.AIOutcomeFailed
	}
	if confidence >= 0.7 && len(warnings) == 0 {
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
	if resp.Metadata == nil {
		resp.Metadata = &ai.AIMetadata{}
	}
	if resp.Metadata.LatencyMs == 0 && latencyMs > 0 {
		resp.Metadata.LatencyMs = int(latencyMs)
	}

	outcome := deriveAIOutcome(resp)
	success := outcome == metadata.AIOutcomeSuccess

	var retryCount int
	var needsClarification bool
	var totalTokens int
	var promptBuildMs int64
	var confidence float64
	var totalWarnings int

	if resp.Metadata != nil {
		retryCount = resp.Metadata.RetryCount
		if resp.Metadata.TokenUsage != nil {
			totalTokens = resp.Metadata.TokenUsage.Total
		}
		if resp.Metadata.PromptStats != nil {
			promptBuildMs = resp.Metadata.PromptStats.PromptBuildDurationMs
		}
	}
	if resp.Clarification != nil {
		needsClarification = resp.Clarification.NeedsClarification
	}
	if resp.Result != nil {
		confidence = resp.Result.Confidence
		totalWarnings = len(resp.Result.Warnings)
	}

	if h.metrics != nil {
		h.metrics.RecordAIRequest(latencyMs, success, retryCount, needsClarification)
		h.metrics.RecordLLMRequest(latencyMs, totalTokens, promptBuildMs)
	}

	logArgs := []any{
		"datasource_id", req.DatasourceID,
		"outcome", outcome,
		"retry_count", retryCount,
		"confidence", confidence,
		"latency_ms", latencyMs,
		"needs_clarification", needsClarification,
		"warnings", totalWarnings,
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

	needsClarification := false
	var retryCount int
	var modelUsed string
	var latencyMs int
	var costUSD float64
	var totalTokens, promptTokens, completionTokens int

	if resp.Clarification != nil {
		needsClarification = resp.Clarification.NeedsClarification
	}
	if resp.Metadata != nil {
		retryCount = resp.Metadata.RetryCount
		modelUsed = resp.Metadata.ModelUsed
		latencyMs = resp.Metadata.LatencyMs
		costUSD = resp.Metadata.CostUSD
		if resp.Metadata.TokenUsage != nil {
			totalTokens = resp.Metadata.TokenUsage.Total
			promptTokens = resp.Metadata.TokenUsage.Prompt
			completionTokens = resp.Metadata.TokenUsage.Completion
		}
	}

	entry.RetryCount = retryCount
	entry.NeedsClarification = needsClarification
	if modelUsed != "" {
		entry.ModelUsed = &modelUsed
	}
	if latencyMs > 0 {
		ms := latencyMs
		entry.LatencyMs = &ms
	}
	if costUSD > 0 {
		cost := costUSD
		entry.CostUSD = &cost
	}
	if promptTokens > 0 {
		v := promptTokens
		entry.PromptTokens = &v
	}
	if completionTokens > 0 {
		v := completionTokens
		entry.CompletionTokens = &v
	}
	if totalTokens > 0 {
		tokens := totalTokens
		entry.TokenCount = &tokens
	}
}
