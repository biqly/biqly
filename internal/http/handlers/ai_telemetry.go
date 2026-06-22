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

	h.recordMetricsAndState(observeAIRequestParams{
		ctx: ctx, req: req, resp: resp, pc: pc,
		latencyMs: latencyMs, llmMs: llmMs, promptBuildMs: promptBuildMs,
		success: success, needsClarification: needsClarification,
		retryCount: retryCount, promptTokens: promptTokens, completionTokens: completionTokens,
		repairAttempts: repairAttempts, repairErrorCodes: repairErrorCodes,
	})

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

type observeAIRequestParams struct {
	ctx                context.Context
	req                aiQueryRequest
	resp               *ai.Response
	pc                 *ProcessContext
	latencyMs          int64
	llmMs              int64
	promptBuildMs      int64
	success            bool
	needsClarification bool
	retryCount         int
	promptTokens       int
	completionTokens   int
	repairAttempts     int
	repairErrorCodes   []string
}

func (h *AIHandler) recordMetricsAndState(p observeAIRequestParams) {
	if h.metrics == nil {
		return
	}
	h.metrics.RecordAIRequest(p.latencyMs, p.success, p.retryCount, p.needsClarification)
	h.metrics.RecordLLMRequest(p.llmMs, p.promptTokens, p.completionTokens, p.promptBuildMs)
	if p.repairAttempts > 0 {
		h.metrics.RecordAIRepair(p.success, p.repairAttempts, p.repairErrorCodes)
	}
	if isAmbiguityAnalyzerClarification(p.resp) {
		h.metrics.RecordAmbiguityClarificationRound(p.pc.nextAmbiguityClarificationRound())
		userID := bimw.UserID(p.ctx)
		if userID != "" {
			h.activeClarifications.Store(userID, clarificationState{
				Question: p.req.Question,
				Round:    p.pc.nextAmbiguityClarificationRound(),
			})
		}
	} else if p.req.ClarificationChoice != "" && !p.needsClarification {
		h.metrics.RecordAmbiguityResolution("resolved")
		userID := bimw.UserID(p.ctx)
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
