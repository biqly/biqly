package metadata

import (
	"context"
	"fmt"

	platformdb "github.com/biqly/biqly/internal/platform/db"
)

// UpdateColumnPIIDetection records an automated PII detection result on a
// column. Manual review fields are left untouched.
func (r *Repository) UpdateColumnPIIDetection(ctx context.Context, id, piiType string, confidence float64) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE columns
		SET pii_type = $2, pii_confidence = $3, pii_detected_at = now()
		WHERE id = $1
	`, id, piiType, confidence)
	if err != nil {
		return fmt.Errorf("update column pii detection: %w", err)
	}
	return nil
}

// SetColumnPIIReview applies a manual admin review: pins the PII type and
// masking strategy with full confidence and records the reviewer.
func (r *Repository) SetColumnPIIReview(ctx context.Context, id, piiType string, maskingStrategy *string, reviewedBy string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE columns
		SET pii_type = $2, pii_confidence = 1.0, pii_detected_at = now(),
			pii_masking_strategy = $3, pii_reviewed_by = $4
		WHERE id = $1
	`, id, piiType, maskingStrategy, reviewedBy)
	if err != nil {
		return fmt.Errorf("set column pii review: %w", err)
	}
	return nil
}

// ClearColumnPII removes the PII annotation from a column. When reviewedBy is
// non-empty the clear is recorded as a manual review (column confirmed safe);
// otherwise the review marker is also reset so future scans may re-flag it.
func (r *Repository) ClearColumnPII(ctx context.Context, id string, reviewedBy string) error {
	var reviewer *string
	if reviewedBy != "" {
		reviewer = &reviewedBy
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE columns
		SET pii_type = NULL, pii_confidence = NULL, pii_detected_at = NULL,
			pii_masking_strategy = NULL, pii_reviewed_by = $2
		WHERE id = $1
	`, id, reviewer)
	if err != nil {
		return fmt.Errorf("clear column pii: %w", err)
	}
	return nil
}

// ListPIIColumns returns all PII-annotated columns for a datasource.
func (r *Repository) ListPIIColumns(ctx context.Context, datasourceID string) ([]Column, error) {
	query := `
		SELECT ` + columnSelectColumns + `
		FROM columns
		WHERE datasource_id = $1 AND pii_type IS NOT NULL
		ORDER BY schema_name, table_name, column_name
	`
	return platformdb.QuerySliceErr(ctx, r.db, "list pii columns", query, []any{datasourceID}, scanColumn)
}

// PIIDatasourceSummary aggregates PII review state for one datasource.
type PIIDatasourceSummary struct {
	DatasourceID   string         `json:"datasource_id"`
	DatasourceName string         `json:"datasource_name"`
	TotalColumns   int            `json:"total_columns"`
	PIIDetected    int            `json:"pii_detected"`
	Reviewed       int            `json:"reviewed"`
	Unreviewed     int            `json:"unreviewed"`
	ByType         map[string]int `json:"by_type"`
}

// PIIComplianceSummary returns per-datasource PII detection/review counts and
// a per-type breakdown for compliance reporting.
func (r *Repository) PIIComplianceSummary(ctx context.Context) ([]PIIDatasourceSummary, error) {
	summaries, err := platformdb.QuerySliceErr(ctx, r.db, "pii compliance summary", `
		SELECT d.id, d.name,
			COUNT(c.id) AS total_columns,
			COUNT(c.id) FILTER (WHERE c.pii_type IS NOT NULL) AS pii_detected,
			COUNT(c.id) FILTER (WHERE c.pii_type IS NOT NULL AND c.pii_reviewed_by IS NOT NULL) AS reviewed
		FROM datasources d
		LEFT JOIN columns c ON c.datasource_id = d.id
		GROUP BY d.id, d.name
		ORDER BY d.name
	`, nil, func(s platformdb.Scanner) (PIIDatasourceSummary, error) {
		var sum PIIDatasourceSummary
		if err := s.Scan(&sum.DatasourceID, &sum.DatasourceName, &sum.TotalColumns, &sum.PIIDetected, &sum.Reviewed); err != nil {
			return sum, fmt.Errorf("scan pii summary: %w", err)
		}
		sum.Unreviewed = sum.PIIDetected - sum.Reviewed
		sum.ByType = map[string]int{}
		return sum, nil
	})
	if err != nil {
		return nil, err
	}

	type typeCount struct {
		datasourceID string
		piiType      string
		count        int
	}
	counts, err := platformdb.QuerySliceErr(ctx, r.db, "pii type breakdown", `
		SELECT datasource_id, pii_type, COUNT(*)
		FROM columns
		WHERE pii_type IS NOT NULL
		GROUP BY datasource_id, pii_type
	`, nil, func(s platformdb.Scanner) (typeCount, error) {
		var tc typeCount
		if err := s.Scan(&tc.datasourceID, &tc.piiType, &tc.count); err != nil {
			return tc, fmt.Errorf("scan pii type count: %w", err)
		}
		return tc, nil
	})
	if err != nil {
		return nil, err
	}

	byDS := make(map[string]*PIIDatasourceSummary, len(summaries))
	for i := range summaries {
		byDS[summaries[i].DatasourceID] = &summaries[i]
	}
	for _, tc := range counts {
		if sum, ok := byDS[tc.datasourceID]; ok {
			sum.ByType[tc.piiType] = tc.count
		}
	}
	return summaries, nil
}
