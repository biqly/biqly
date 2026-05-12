package query

import (
	"testing"

	"github.com/biqly/biqly/internal/semantic"
)

func newModel() *semantic.SemanticModel {
	return &semantic.SemanticModel{
		Name: "orders",
		Dimensions: []semantic.Dimension{
			{Name: "country", ColumnRef: "customers.country", Type: "text"},
			{Name: "order_date", ColumnRef: "orders.created_at", Type: "date"},
		},
		Metrics: []semantic.Metric{
			{Name: "order_count", Expression: "orders.id", Aggregation: "count"},
			{Name: "total_revenue", Expression: "orders.total_amount", Aggregation: "sum"},
		},
	}
}

func TestEnrichResultTagsDimensionsAndMetrics(t *testing.T) {
	model := newModel()
	lq := LogicalQuery{
		Select: []SelectItem{
			{Type: SelectTypeDimension, Name: "country"},
			{Type: SelectTypeMetric, Name: "total_revenue"},
		},
	}
	result := &Result{
		Columns: []ResultColumn{
			{Name: "country", Type: "TEXT"},
			{Name: "total_revenue", Type: "NUMERIC"},
		},
		Rows: [][]any{{"TR", 100.0}, {"US", 50.0}},
	}

	EnrichResult(result, lq, model)

	if result.Columns[0].SemanticType != SemanticTypeDimension {
		t.Errorf("country.SemanticType = %q, want dimension", result.Columns[0].SemanticType)
	}
	if result.Columns[0].Format != FormatText {
		t.Errorf("country.Format = %q, want text", result.Columns[0].Format)
	}
	if result.Columns[1].SemanticType != SemanticTypeMetric {
		t.Errorf("total_revenue.SemanticType = %q, want metric", result.Columns[1].SemanticType)
	}
	if result.Columns[1].Format != FormatNumber {
		t.Errorf("total_revenue.Format = %q, want number", result.Columns[1].Format)
	}
}

func TestEnrichResultDateDimensionGetsDateFormat(t *testing.T) {
	model := newModel()
	lq := LogicalQuery{
		Select: []SelectItem{
			{Type: SelectTypeDimension, Name: "order_date"},
			{Type: SelectTypeMetric, Name: "order_count"},
		},
		GroupBy: []GroupBy{{Field: "order_date", TimeGrain: TimeGrainMonth}},
	}
	result := &Result{
		Columns: []ResultColumn{
			{Name: "order_date", Type: "INT"},
			{Name: "order_count", Type: "BIGINT"},
		},
		Rows: [][]any{{1, 10}, {2, 20}},
	}

	EnrichResult(result, lq, model)

	if result.Columns[0].Format != FormatDate {
		t.Errorf("order_date.Format = %q, want date", result.Columns[0].Format)
	}
	wantChart := []string{ChartLine, ChartBar, ChartTable}
	if !equalStringSlices(result.ChartSuggestions, wantChart) {
		t.Errorf("chart_suggestions = %v, want %v", result.ChartSuggestions, wantChart)
	}
}

func TestEnrichResultChartsForSingleMetricSingleRow(t *testing.T) {
	model := newModel()
	lq := LogicalQuery{
		Select: []SelectItem{{Type: SelectTypeMetric, Name: "order_count"}},
	}
	result := &Result{
		Columns: []ResultColumn{{Name: "order_count", Type: "BIGINT"}},
		Rows:    [][]any{{42}},
	}

	EnrichResult(result, lq, model)

	want := []string{ChartNumber, ChartTable}
	if !equalStringSlices(result.ChartSuggestions, want) {
		t.Errorf("chart_suggestions = %v, want %v", result.ChartSuggestions, want)
	}
}

func TestEnrichResultChartsForCategoricalBreakdown(t *testing.T) {
	model := newModel()
	lq := LogicalQuery{
		Select: []SelectItem{
			{Type: SelectTypeDimension, Name: "country"},
			{Type: SelectTypeMetric, Name: "total_revenue"},
		},
	}
	result := &Result{
		Columns: []ResultColumn{
			{Name: "country"},
			{Name: "total_revenue"},
		},
		Rows: [][]any{{"TR", 1.0}, {"US", 2.0}, {"DE", 3.0}},
	}

	EnrichResult(result, lq, model)

	// 3 rows ≤ 8 → bar+pie+table.
	want := []string{ChartBar, ChartPie, ChartTable}
	if !equalStringSlices(result.ChartSuggestions, want) {
		t.Errorf("chart_suggestions = %v, want %v", result.ChartSuggestions, want)
	}
}

func TestEnrichResultLeavesUnknownColumnsAlone(t *testing.T) {
	model := newModel()
	lq := LogicalQuery{Select: []SelectItem{{Type: SelectTypeDimension, Name: "country"}}}
	result := &Result{
		Columns: []ResultColumn{
			{Name: "country"},
			{Name: "raw_sql_col"},
		},
	}
	EnrichResult(result, lq, model)
	if result.Columns[1].SemanticType != "" {
		t.Errorf("unknown column should be left blank, got %q", result.Columns[1].SemanticType)
	}
}

func TestEnrichResultNilSafe(t *testing.T) {
	EnrichResult(nil, LogicalQuery{}, nil)
	EnrichResult(&Result{}, LogicalQuery{}, nil)
}

func equalStringSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
