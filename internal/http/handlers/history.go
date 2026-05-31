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
	result *query.QueryResult,
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
	routing *routing.TableRoutingResult,
	resp *ai.Response,
) {
	entry := buildAIHistoryEntry(req, model, routing, resp)
	if h.deps.CatalogClient != nil {
		if _, err := h.deps.CatalogClient.CreateAIHistory(ctx, *entry); err != nil {
			slog.ErrorContext(ctx, "create AI query history via catalog failed", "error", err)
		}
		return
	}
	if err := h.deps.MetaRepo.CreateAIQueryHistory(ctx, entry); err != nil {
		slog.ErrorContext(ctx, "create AI query history failed", "error", err)
	}
}

func buildAIHistoryEntry(
	req aiQueryRequest,
	model *semantic.SemanticModel,
	routing *routing.TableRoutingResult,
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

	if resp != nil {
		if resp.Metadata != nil {
			prompt = resp.Metadata.Prompt
			locale = resp.Metadata.PromptTemplateLocale
			versions = resp.Metadata.PromptTemplateVersions
			bundleVer = resp.Metadata.PromptTemplateBundleVersion
			raw = resp.Metadata.RawResponse
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
			"routing":                        routing,
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
	}
	enrichAIHistoryEntry(entry, resp)
	return entry
}
