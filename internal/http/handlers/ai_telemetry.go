package handlers

import (
	"context"
	"log/slog"
	"strings"

	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/ai/routing"
	bimw "github.com/biqly/biqly/internal/http/middleware"
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
	routeResult *routing.TableRoutingResult,
	resp *ai.Response,
	latencyMs int64,
	pc *ProcessContext,
) *ai.Response {
	if resp == nil {
		resp = failedAIResponse(nil)
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
	var promptTokens, completionTokens int
	var promptBuildMs int64
	var confidence float64
	var totalWarnings int
	var repairAttempts int
	var repairErrorCodes []string

	if resp.Metadata != nil {
		retryCount = resp.Metadata.RetryCount
		if resp.Metadata.TokenUsage != nil {
			promptTokens = resp.Metadata.TokenUsage.Prompt
			completionTokens = resp.Metadata.TokenUsage.Completion
		}
		if resp.Metadata.PromptStats != nil {
			promptBuildMs = resp.Metadata.PromptStats.PromptBuildDurationMs
		}
		if len(resp.Metadata.RepairDetails) > 0 {
			repairAttempts = len(resp.Metadata.RepairDetails)
			for _, rd := range resp.Metadata.RepairDetails {
				repairErrorCodes = append(repairErrorCodes, rd.ErrorCodes...)
			}
		}
	}
	if resp.Clarification != nil {
		needsClarification = resp.Clarification.NeedsClarification
	}
	if resp.Result != nil {
		confidence = resp.Result.Confidence
		totalWarnings = len(resp.Result.Warnings)
	}

	llmMs := latencyMs
	if resp.Metadata != nil && resp.Metadata.LLMGenerateDurationMs > 0 {
		llmMs = int64(resp.Metadata.LLMGenerateDurationMs)
	}

	h.recordMetricsAndState(ctx, req, resp, pc, latencyMs, llmMs, promptBuildMs, success, needsClarification, retryCount, promptTokens, completionTokens, repairAttempts, repairErrorCodes)

	logArgs := []any{
		"datasource_id", req.DatasourceID,
		"outcome", outcome,
		"retry_count", retryCount,
		"confidence", confidence,
		"latency_ms", latencyMs,
		"needs_clarification", needsClarification,
		"warnings", totalWarnings,
	}
	if repairAttempts > 0 {
		logArgs = append(logArgs, "repair_attempts", repairAttempts, "repair_errors", strings.Join(repairErrorCodes, ","))
	}
	if model != nil {
		logArgs = append(logArgs, "model", model.Name)
	}
	slog.InfoContext(ctx, "ai query completed", logArgs...)

	attachGenerationTrace(routeResult, model, resp)

	h.recordAIHistory(ctx, req, model, routeResult, resp, pc)
	return resp
}

func (h *AIHandler) recordMetricsAndState(
	ctx context.Context,
	req aiQueryRequest,
	resp *ai.Response,
	pc *ProcessContext,
	latencyMs, llmMs, promptBuildMs int64,
	success, needsClarification bool,
	retryCount, promptTokens, completionTokens, repairAttempts int,
	repairErrorCodes []string,
) {
	if h.metrics == nil {
		return
	}
	h.metrics.RecordAIRequest(latencyMs, success, retryCount, needsClarification)
	h.metrics.RecordLLMRequest(llmMs, promptTokens, completionTokens, promptBuildMs)
	if repairAttempts > 0 {
		h.metrics.RecordAIRepair(success, repairAttempts, repairErrorCodes)
	}
	if isAmbiguityAnalyzerClarification(resp) {
		h.metrics.RecordAmbiguityClarificationRound(pc.nextAmbiguityClarificationRound())
		userID := bimw.UserID(ctx)
		if userID != "" {
			h.activeClarifications.Store(userID, clarificationState{
				Question: req.Question,
				Round:    pc.nextAmbiguityClarificationRound(),
			})
		}
	} else if req.ClarificationChoice != "" && !needsClarification {
		h.metrics.RecordAmbiguityResolution("resolved")
		userID := bimw.UserID(ctx)
		if userID != "" {
			h.activeClarifications.Delete(userID)
		}
	}
}

func attachGenerationTrace(routeResult *routing.TableRoutingResult, model *semantic.SemanticModel, resp *ai.Response) {
	if resp == nil {
		return
	}
	trace := ai.BuildGenerationTrace(routeResult, model, resp)
	if trace == nil {
		return
	}
	if resp.Metadata == nil {
		resp.Metadata = &ai.AIMetadata{}
	}
	resp.Metadata.GenerationTrace = trace
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
		entry.ModelUsed = new(modelUsed)
	}
	if latencyMs > 0 {
		entry.LatencyMs = new(latencyMs)
	}
	if costUSD > 0 {
		entry.CostUSD = new(costUSD)
	}
	if promptTokens > 0 {
		entry.PromptTokens = new(promptTokens)
	}
	if completionTokens > 0 {
		entry.CompletionTokens = new(completionTokens)
	}
	if totalTokens > 0 {
		entry.TokenCount = new(totalTokens)
	}
}
