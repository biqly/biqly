package abtest

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"testing"
	"time"
)

type metricsFakeRows struct {
	values [][]any
	index  int
}

func (r *metricsFakeRows) Next() bool {
	r.index++
	return r.index < len(r.values)
}

func (r *metricsFakeRows) Scan(dest ...any) error {
	if r.index < 0 || r.index >= len(r.values) {
		return errors.New("fake rows scan without current row")
	}
	row := r.values[r.index]
	if len(dest) != len(row) {
		return errors.New("scan destination count does not match values")
	}
	for i := range dest {
		switch d := dest[i].(type) {
		case *string:
			if v, ok := row[i].(string); ok {
				*d = v
			} else {
				return errors.New("scan value is not string")
			}
		case *int:
			if v, ok := row[i].(int); ok {
				*d = v
			} else {
				return errors.New("scan value is not int")
			}
		case *float64:
			if v, ok := row[i].(float64); ok {
				*d = v
			} else {
				return errors.New("scan value is not float64")
			}
		case *sql.NullInt64:
			if v, ok := row[i].(sql.NullInt64); ok {
				*d = v
			} else if v, ok := row[i].(int64); ok {
				*d = sql.NullInt64{Int64: v, Valid: true}
			} else if row[i] == nil {
				*d = sql.NullInt64{Valid: false}
			} else {
				return errors.New("scan value is not NullInt64")
			}
		default:
			return errors.New("unsupported scan destination type in metrics test")
		}
	}
	return nil
}

func (*metricsFakeRows) Err() error {
	return nil
}

func (*metricsFakeRows) Close() error {
	return nil
}

func TestComputeMetrics(t *testing.T) {
	ctx := context.Background()
	expID := "exp-1"
	pStart := time.Now().Add(-24 * time.Hour)
	pEnd := time.Now()

	runner := &fakeDBRunner{
		queryContext: func(_ context.Context, query string, args ...any) (rowsScanner, error) {
			if !strings.Contains(query, "FROM ai_query_history") {
				t.Errorf("expected query to be against ai_query_history, got: %s", query)
			}
			if len(args) != 3 {
				t.Errorf("expected 3 query arguments, got %d", len(args))
			}
			if args[0] != expID {
				t.Errorf("expected arg[0] to be %s, got %v", expID, args[0])
			}
			
			// Columns returned:
			// 0: ab_variant_id (string)
			// 1: total_queries (int)
			// 2: success_rate (float64)
			// 3: validator_pass_rate (float64)
			// 4: avg_confidence (float64)
			// 5: user_correction_rate (float64)
			// 6: positive_feedback_rate (float64)
			// 7: execution_success_rate (float64)
			// 8: avg_cost_usd (float64)
			// 9: avg_latency_ms (float64)
			// 10: stddev_cost_usd (float64)
			// 11: stddev_latency_ms (float64)
			// 12: total_tokens (sql.NullInt64)
			rows := [][]any{
				{
					"control-var", 100, 0.85, 0.90, 0.82, 0.05, 0.40, 0.95, 0.015, 120.0, 0.002, 10.0,
					sql.NullInt64{Int64: 45000, Valid: true},
				},
				{
					"treatment-var", 120, 0.92, 0.95, 0.88, 0.02, 0.55, 0.98, 0.012, 105.0, 0.001, 8.5,
					sql.NullInt64{Int64: 51200, Valid: true},
				},
			}
			return &metricsFakeRows{values: rows, index: -1}, nil
		},
	}

	repo := newRepositoryWithRunner(runner)
	collector := NewMetricsCollector(repo)

	metrics, err := collector.ComputeMetrics(ctx, expID, pStart, pEnd)
	if err != nil {
		t.Fatalf("ComputeMetrics failed: %v", err)
	}

	if len(metrics) != 2 {
		t.Fatalf("expected 2 metric records, got %d", len(metrics))
	}

	// Verify Control
	c := metrics[0]
	if c.VariantID != "control-var" {
		t.Errorf("expected control variant, got: %s", c.VariantID)
	}
	if c.TotalQueries != 100 {
		t.Errorf("expected 100 total queries, got %d", c.TotalQueries)
	}
	if c.SuccessRate != 0.85 {
		t.Errorf("expected 0.85 SuccessRate, got %f", c.SuccessRate)
	}
	if c.AvgCostUSD != 0.015 {
		t.Errorf("expected 0.015 AvgCostUSD, got %f", c.AvgCostUSD)
	}
	if c.StdDevCostUSD != 0.002 {
		t.Errorf("expected 0.002 StdDevCostUSD, got %f", c.StdDevCostUSD)
	}
	if c.TotalTokens != 45000 {
		t.Errorf("expected 45000 TotalTokens, got %d", c.TotalTokens)
	}

	// Verify Treatment
	tr := metrics[1]
	if tr.VariantID != "treatment-var" {
		t.Errorf("expected treatment variant, got: %s", tr.VariantID)
	}
	if tr.TotalQueries != 120 {
		t.Errorf("expected 120 total queries, got %d", tr.TotalQueries)
	}
	if tr.SuccessRate != 0.92 {
		t.Errorf("expected 0.92 SuccessRate, got %f", tr.SuccessRate)
	}
	if tr.AvgCostUSD != 0.012 {
		t.Errorf("expected 0.012 AvgCostUSD, got %f", tr.AvgCostUSD)
	}
	if tr.StdDevCostUSD != 0.001 {
		t.Errorf("expected 0.001 StdDevCostUSD, got %f", tr.StdDevCostUSD)
	}
	if tr.TotalTokens != 51200 {
		t.Errorf("expected 51200 TotalTokens, got %d", tr.TotalTokens)
	}
}
