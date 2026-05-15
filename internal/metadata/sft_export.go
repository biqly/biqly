package metadata

import (
	"context"
	"encoding/json"
	"fmt"

	platformdb "github.com/biqly/biqly/internal/platform/db"
)

// SFTCandidateRow is a labeled NL → LogicalQuery pair for SFT dataset export.
type SFTCandidateRow struct {
	Source          string
	Question        string
	LogicalQuery    json.RawMessage
	DatasourceID    string
	SemanticModelID string
	Dialect         string
}

// ListFewShotSFTCandidates returns curated few-shot examples suitable for training.
func (r *Repository) ListFewShotSFTCandidates(ctx context.Context) ([]SFTCandidateRow, error) {
	rows, err := r.ListFewShotCurated(ctx, "", "")
	if err != nil {
		return nil, err
	}
	out := make([]SFTCandidateRow, 0, len(rows))
	for _, row := range rows {
		if row.Question == "" || len(row.LogicalQuery) == 0 {
			continue
		}
		out = append(out, SFTCandidateRow{
			Source:          "few_shot",
			Question:        row.Question,
			LogicalQuery:    row.LogicalQuery,
			DatasourceID:    row.DatasourceID,
			SemanticModelID: row.ModelID,
			Dialect:         row.Dialect,
		})
	}
	return out, nil
}

// ListPositiveAIHistorySFTCandidates returns successful AI query history rows for SFT.
// Rows with negative user_rating are excluded.
func (r *Repository) ListPositiveAIHistorySFTCandidates(ctx context.Context, minConfidence float64) ([]SFTCandidateRow, error) {
	if minConfidence <= 0 {
		minConfidence = 0.7
	}
	const q = `
		SELECT question, logical_query, datasource_id::text, COALESCE(model_id::text, '')
		FROM ai_query_history
		WHERE logical_query IS NOT NULL
		  AND question <> ''
		  AND (user_rating IS NULL OR user_rating = 'positive')
		  AND confidence_score >= $1
		  AND (warnings IS NULL OR cardinality(warnings) = 0)
		ORDER BY created_at DESC
	`
	candidates, err := platformdb.QuerySliceErr(ctx, r.db, "list positive AI history for SFT", q, []any{minConfidence}, scanPositiveAIHistorySFTRow)
	if err != nil {
		return nil, err
	}
	out := make([]SFTCandidateRow, 0, len(candidates))
	for _, row := range candidates {
		if len(row.LogicalQuery) == 0 {
			continue
		}
		out = append(out, row)
	}
	return out, nil
}

func scanPositiveAIHistorySFTRow(s platformdb.Scanner) (SFTCandidateRow, error) {
	var question string
	var lq json.RawMessage
	var dsID, modelID string
	if err := s.Scan(&question, &lq, &dsID, &modelID); err != nil {
		return SFTCandidateRow{}, fmt.Errorf("scan AI history SFT row: %w", err)
	}
	return SFTCandidateRow{
		Source:          "history_positive",
		Question:        question,
		LogicalQuery:    lq,
		DatasourceID:    dsID,
		SemanticModelID: modelID,
	}, nil
}
