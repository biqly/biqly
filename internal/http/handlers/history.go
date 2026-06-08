package handlers

import (
	"context"
	"log/slog"

	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/ai/routing"
	bimw "github.com/biqly/biqly/internal/http/middleware"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
)

const (
	queryStatusSuccess = "success"
	queryStatusFailed  = "failed"
)

func persistQueryHistory(
	ctx context.Context,
	repo *metadata.Repository,
	lq *query.LogicalQuery,
	model *semantic.SemanticModel,
	cq *query.CompiledQuery,
	result *query.Result,
	status string,
	queryErr error,
) {
	entry, err := query.BuildQueryHistoryEntry(lq, model, cq, result, status, queryErr)
	if err != nil {
		slog.ErrorContext(ctx, "AI query history build failed", "error", err)
		return
	}
	if err := repo.CreateQueryHistory(ctx, entry); err != nil {
		slog.ErrorContext(ctx, "AI query history create failed", "error", err)
	}
}

func (h *AIHandler) recordAIHistory(
	ctx context.Context,
	req aiQueryRequest,
	model *semantic.SemanticModel,
	routeResult *routing.TableRoutingResult,
	resp *ai.Response,
) {
	entry := buildAIHistoryEntry(ctx, req, model, routeResult, resp)
	if h.deps.CatalogClient != nil {
		if _, err := h.deps.CatalogClient.CreateAIHistory(ctx, entry); err != nil {
			slog.ErrorContext(ctx, "create AI query history via catalog failed", "error", err)
		}
		return
	}
	if err := h.deps.MetaRepo.CreateAIQueryHistory(ctx, entry); err != nil {
		slog.ErrorContext(ctx, "create AI query history failed", "error", err)
	}
}

func historyUserID(ctx context.Context) string {
	if id := ai.UserIDFromContext(ctx); id != "" {
		return id
	}
	return bimw.UserID(ctx)
}

func buildAIHistoryEntry(
	ctx context.Context,
	req aiQueryRequest,
	model *semantic.SemanticModel,
	routeResult *routing.TableRoutingResult,
	resp *ai.Response,
) *metadata.AIQueryHistoryEntry {
	var prompt string
	var locale string
	var versions map[string]int
	var bundleVer int
	var raw string
	var lq *query.LogicalQuery
	var conf float64
	var warnings []string
	var abExpID, abVarID string

	if resp != nil {
		if resp.Metadata != nil {
			prompt = resp.Metadata.Prompt
			locale = resp.Metadata.PromptTemplateLocale
			versions = resp.Metadata.PromptTemplateVersions
			bundleVer = resp.Metadata.PromptTemplateBundleVersion
			raw = resp.Metadata.RawResponse
			abExpID = resp.Metadata.ABExperimentID
			abVarID = resp.Metadata.ABVariantID
		}
		if resp.Result != nil {
			lq = resp.Result.LogicalQuery
			conf = resp.Result.Confidence
			warnings = resp.Result.Warnings
		}
	}

	entry := &metadata.AIQueryHistoryEntry{
		DatasourceID: req.DatasourceID,
		ModelID:      query.HistoryModelID(model),
		Question:     req.Question,
		PromptContext: map[string]any{
			"model_id":                       req.ModelID,
			"selected_scope":                 req.Tables,
			"routing":                        routeResult,
			"prompt":                         prompt,
			"prompt_template_locale":         locale,
			"prompt_template_versions":       versions,
			"prompt_template_bundle_version": bundleVer,
		},
		AIResponse: map[string]any{
			"response":     resp,
			"raw_response": raw,
		},
		LogicalQuery:    lq,
		ConfidenceScore: &conf,
		Warnings:        warnings,
		ABExperimentID:  nullIfEmpty(abExpID),
		ABVariantID:     nullIfEmpty(abVarID),
	}
	if userID := historyUserID(ctx); userID != "" {
		entry.UserID = new(userID)
	}
	enrichAIHistoryEntry(entry, resp)
	return entry
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
