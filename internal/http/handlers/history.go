package handlers

import (
	"context"
	"log/slog"

	"github.com/biqly/biqly/internal/ai"
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
	routing *ai.TableRoutingResult,
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
	routing *ai.TableRoutingResult,
	resp *ai.Response,
) *metadata.AIQueryHistoryEntry {
	entry := &metadata.AIQueryHistoryEntry{
		DatasourceID: req.DatasourceID,
		ModelID:      query.HistoryModelID(model),
		Question:     req.Question,
		PromptContext: map[string]any{
			"model_id":                       req.ModelID,
			"selected_scope":                 req.Tables,
			"routing":                        routing,
			"prompt":                         resp.Prompt,
			"prompt_template_locale":         resp.PromptTemplateLocale,
			"prompt_template_versions":       resp.PromptTemplateVersions,
			"prompt_template_bundle_version": resp.PromptTemplateBundleVersion,
		},
		AIResponse: map[string]any{
			"response":     resp,
			"raw_response": resp.RawResponse,
		},
		LogicalQuery:    resp.LogicalQuery,
		ConfidenceScore: &resp.Confidence,
		Warnings:        resp.Warnings,
	}
	enrichAIHistoryEntry(entry, resp)
	return entry
}
