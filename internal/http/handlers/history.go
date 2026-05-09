package handlers

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
	"github.com/google/uuid"
)

const (
	queryStatusSuccess = "success"
	queryStatusFailed  = "failed"
)

func (h *QueryHandler) recordQueryHistory(
	ctx context.Context,
	lq query.LogicalQuery,
	model *semantic.SemanticModel,
	cq *query.CompiledQuery,
	result *query.QueryResult,
	status string,
	queryErr error,
) {
	entry, err := buildQueryHistoryEntry(lq, model, cq, result, status, queryErr)
	if err != nil {
		slog.ErrorContext(ctx, "build query history failed", "error", err)
		return
	}
	if err := h.deps.MetaRepo.CreateQueryHistory(ctx, entry); err != nil {
		slog.ErrorContext(ctx, "create query history failed", "error", err)
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
	if err := h.deps.MetaRepo.CreateAIQueryHistory(ctx, entry); err != nil {
		slog.ErrorContext(ctx, "create AI query history failed", "error", err)
	}
}

func (h *AIHandler) recordAIQueryHistory(
	ctx context.Context,
	lq query.LogicalQuery,
	model *semantic.SemanticModel,
	cq *query.CompiledQuery,
	result *query.QueryResult,
	status string,
	queryErr error,
) {
	entry, err := buildQueryHistoryEntry(lq, model, cq, result, status, queryErr)
	if err != nil {
		slog.ErrorContext(ctx, "build AI query history failed", "error", err)
		return
	}
	if err := h.deps.MetaRepo.CreateQueryHistory(ctx, entry); err != nil {
		slog.ErrorContext(ctx, "create AI query history failed", "error", err)
	}
}

func buildQueryHistoryEntry(
	lq query.LogicalQuery,
	model *semantic.SemanticModel,
	cq *query.CompiledQuery,
	result *query.QueryResult,
	status string,
	queryErr error,
) (*query.HistoryEntry, error) {
	entry := &query.HistoryEntry{
		DatasourceID: lq.DatasourceID,
		ModelID:      historyModelID(model),
		LogicalQuery: lq,
		Status:       status,
	}
	if cq != nil {
		entry.CompiledSQL = &cq.SQL
		sqlArgs, err := marshalSQLArgs(cq.Args)
		if err != nil {
			return nil, err
		}
		entry.SQLArgs = sqlArgs
	}
	if result != nil {
		rowCount := result.Stats.RowCount
		durationMs := int(result.Stats.DurationMs)
		entry.RowCount = &rowCount
		entry.DurationMs = &durationMs
	}
	if queryErr != nil {
		msg := queryErr.Error()
		entry.ErrorMessage = &msg
	}
	return entry, nil
}

func buildAIHistoryEntry(
	req aiQueryRequest,
	model *semantic.SemanticModel,
	routing *ai.TableRoutingResult,
	resp *ai.Response,
) *metadata.AIQueryHistoryEntry {
	entry := &metadata.AIQueryHistoryEntry{
		DatasourceID: req.DatasourceID,
		ModelID:      historyModelID(model),
		Question:     req.Question,
		PromptContext: map[string]any{
			"model_id":       req.ModelID,
			"selected_scope": req.Tables,
			"routing":        routing,
			"prompt":         resp.Prompt,
		},
		AIResponse: map[string]any{
			"response":     resp,
			"raw_response": resp.RawResponse,
		},
		LogicalQuery:    resp.LogicalQuery,
		ConfidenceScore: &resp.Confidence,
		Warnings:        resp.Warnings,
	}
	return entry
}

func historyModelID(model *semantic.SemanticModel) *string {
	if model == nil {
		return nil
	}
	if _, err := uuid.Parse(model.ID); err != nil {
		return nil
	}
	return &model.ID
}

func marshalSQLArgs(args []any) (*string, error) {
	if args == nil {
		return nil, nil
	}
	encoded, err := json.Marshal(args)
	if err != nil {
		return nil, err
	}
	s := string(encoded)
	return &s, nil
}
