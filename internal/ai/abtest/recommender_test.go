package abtest

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"
)

type recommenderFakeRows struct {
	values [][]any
	index  int
}

func (r *recommenderFakeRows) Next() bool {
	r.index++
	return r.index < len(r.values)
}

func (r *recommenderFakeRows) Scan(dest ...any) error {
	row := r.values[r.index]
	for i := range dest {
		switch d := dest[i].(type) {
		case *string:
			v, ok := row[i].(string)
			if !ok {
				return fmt.Errorf("scan string: %T", row[i])
			}
			*d = v
		case *int:
			v, ok := row[i].(int)
			if !ok {
				return fmt.Errorf("scan int: %T", row[i])
			}
			*d = v
		case *bool:
			v, ok := row[i].(bool)
			if !ok {
				return fmt.Errorf("scan bool: %T", row[i])
			}
			*d = v
		case *float64:
			v, ok := row[i].(float64)
			if !ok {
				return fmt.Errorf("scan float64: %T", row[i])
			}
			*d = v
		case *sql.NullInt64:
			if v, ok := row[i].(sql.NullInt64); ok {
				*d = v
			} else if v, ok := row[i].(int64); ok {
				*d = sql.NullInt64{Int64: v, Valid: true}
			} else {
				*d = sql.NullInt64{Valid: false}
			}
		}
	}
	return nil
}

func (*recommenderFakeRows) Err() error { return nil }
func (*recommenderFakeRows) Close() error { return nil }

func TestRecommender_Recommend(t *testing.T) {
	ctx := context.Background()
	now := time.Now()

	tests := []struct {
		name                 string
		envMinSample         string
		expStatus            ExperimentStatus
		mockVariants         [][]any
		mockMetrics          [][]any
		wantWinnerID         string
		wantMinSampleReached bool
		reasonContains       string
	}{
		{
			name:      "Sample size not met",
			expStatus: ExperimentStatusRunning,
			mockVariants: [][]any{
				{"control-id", "exp-1", "control", 1, 50, true},
				{"treatment-id", "exp-1", "treatment", 2, 50, false},
			},
			mockMetrics: [][]any{
				{"control-id", 40, 0.80, 0.80, 0.80, 0.0, 0.0, 0.80, 0.0, 0.0, 0.0, 0.0, sql.NullInt64{Int64: 100, Valid: true}},
				{"treatment-id", 45, 0.95, 0.95, 0.95, 0.0, 0.0, 0.95, 0.0, 0.0, 0.0, 0.0, sql.NullInt64{Int64: 100, Valid: true}},
			},
			wantWinnerID:         "",
			wantMinSampleReached: false,
			reasonContains:       "Sample size threshold",
		},
		{
			name:         "Sample size met via custom env",
			envMinSample: "30",
			expStatus:    ExperimentStatusRunning,
			mockVariants: [][]any{
				{"control-id", "exp-1", "control", 1, 50, true},
				{"treatment-id", "exp-1", "treatment", 2, 50, false},
			},
			mockMetrics: [][]any{
				{"control-id", 40, 0.50, 0.50, 0.50, 0.0, 0.0, 0.50, 0.0, 0.0, 0.0, 0.0, sql.NullInt64{Int64: 100, Valid: true}},
				{"treatment-id", 45, 0.90, 0.90, 0.90, 0.0, 0.0, 0.90, 0.0, 0.0, 0.0, 0.0, sql.NullInt64{Int64: 100, Valid: true}},
			},
			wantWinnerID:         "treatment-id",
			wantMinSampleReached: true,
			reasonContains:       "statistically significantly better",
		},
		{
			name:      "Losing variant",
			expStatus: ExperimentStatusRunning,
			mockVariants: [][]any{
				{"control-id", "exp-1", "control", 1, 50, true},
				{"treatment-id", "exp-1", "treatment", 2, 50, false},
			},
			mockMetrics: [][]any{
				{"control-id", 150, 0.90, 0.90, 0.90, 0.0, 0.0, 0.90, 0.0, 0.0, 0.0, 0.0, sql.NullInt64{Int64: 100, Valid: true}},
				{"treatment-id", 150, 0.50, 0.50, 0.50, 0.0, 0.0, 0.50, 0.0, 0.0, 0.0, 0.0, sql.NullInt64{Int64: 100, Valid: true}},
			},
			wantWinnerID:         "",
			wantMinSampleReached: true,
			reasonContains:       "performing significantly worse",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envMinSample != "" {
				t.Setenv("BI_AB_MIN_SAMPLE_SIZE", tt.envMinSample)
			} else {
				t.Setenv("BI_AB_MIN_SAMPLE_SIZE", "")
			}

			runner := &fakeDBRunner{
				queryRowContext: func(_ context.Context, query string, _ ...any) rowScanner {
					if strings.Contains(query, "FROM ab_experiments") {
						return fakeRow{values: []any{
							"exp-1", "Experiment 1", "desc", "clarification", "en", string(tt.expStatus),
							sql.NullTime{Time: now, Valid: true}, sql.NullTime{}, sql.NullString{}, now, now,
						}}
					}
					return fakeRow{err: sql.ErrNoRows}
				},
				queryContext: func(_ context.Context, query string, _ ...any) (rowsScanner, error) {
					if strings.Contains(query, "FROM ab_variants") {
						return &recommenderFakeRows{values: tt.mockVariants, index: -1}, nil
					}
					if strings.Contains(query, "FROM ai_query_history") {
						return &recommenderFakeRows{values: tt.mockMetrics, index: -1}, nil
					}
					return nil, sql.ErrNoRows
				},
			}

			repo := newRepositoryWithRunner(runner)
			collector := NewMetricsCollector(repo)
			rec := NewRecommender(repo, collector)

			res, err := rec.Recommend(ctx, "exp-1")
			if err != nil {
				t.Fatalf("Recommend failed: %v", err)
			}

			if res.WinnerVariantID != tt.wantWinnerID {
				t.Errorf("WinnerVariantID = %q, want %q", res.WinnerVariantID, tt.wantWinnerID)
			}
			if res.MinSampleReached != tt.wantMinSampleReached {
				t.Errorf("MinSampleReached = %v, want %v", res.MinSampleReached, tt.wantMinSampleReached)
			}
			if !strings.Contains(res.Reason, tt.reasonContains) {
				t.Errorf("Reason = %q, expected to contain %q", res.Reason, tt.reasonContains)
			}
		})
	}
}
