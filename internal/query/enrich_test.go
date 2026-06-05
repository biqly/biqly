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

	EnrichResult(result, &lq, model)

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

	EnrichResult(result, &lq, model)

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

	EnrichResult(result, &lq, model)

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

	EnrichResult(result, &lq, model)

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
	EnrichResult(result, &lq, model)
	if result.Columns[1].SemanticType != "" {
		t.Errorf("unknown column should be left blank, got %q", result.Columns[1].SemanticType)
	}
}

func TestEnrichResultNilSafe(t *testing.T) {
	t.Helper()
	EnrichResult(nil, &LogicalQuery{}, nil)
	EnrichResult(&Result{}, &LogicalQuery{}, nil)
}

func TestSuggestPivotTwoDimensions(t *testing.T) {
	cols := []ResultColumn{
		{Name: "region", SemanticType: SemanticTypeDimension, Format: FormatText},
		{Name: "product", SemanticType: SemanticTypeDimension, Format: FormatText},
		{Name: "revenue", SemanticType: SemanticTypeMetric, Format: FormatNumber},
	}
	hint := suggestPivot(cols)
	if hint == nil || hint.RowField != "region" || hint.ColumnField != "product" {
		t.Fatalf("pivot fields = %+v", hint)
	}
}

func TestDetectAnomaliesFlagsOutlier(t *testing.T) {
	rows := make([][]any, 12)
	for i := range rows {
		rows[i] = []any{float64(10 + float64(i%5))}
	}
	rows[11] = []any{float64(1000)}
	result := &Result{
		Columns: []ResultColumn{{Name: "amount", SemanticType: SemanticTypeMetric, Format: FormatNumber}},
		Rows:    rows,
	}
	anomalies := detectAnomalies(result)
	if len(anomalies) == 0 {
		t.Fatal("expected at least one anomaly")
	}
	if anomalies[0].RowIndex != 11 {
		t.Errorf("anomaly row = %d, want 11", anomalies[0].RowIndex)
	}
}

func TestVisualizationHintFromResult(t *testing.T) {
	ct, reason := VisualizationHintFromResult(&Result{ChartSuggestions: []string{ChartBar, ChartTable}})
	if ct != ChartBar || reason == "" {
		t.Errorf("hint = %q %q", ct, reason)
	}
	ct, _ = VisualizationHintFromResult(&Result{ChartSuggestions: []string{ChartNumber}})
	if ct != ChartTable {
		t.Errorf("number maps to table, got %q", ct)
	}
}

func TestAnomalyWarningMessages(t *testing.T) {
	if msgs := AnomalyWarningMessages(nil); msgs != nil {
		t.Fatalf("nil result: got %v, want nil", msgs)
	}
	if msgs := AnomalyWarningMessages(&Result{}); msgs != nil {
		t.Fatalf("empty anomalies: got %v, want nil", msgs)
	}

	result := &Result{
		Anomalies: []Anomaly{
			{RowIndex: 0, Column: "revenue", Score: 3.2},
			{RowIndex: 5, Column: "revenue", Score: 2.8},
		},
	}
	msgs := AnomalyWarningMessages(result)
	if len(msgs) != 3 {
		t.Fatalf("len(msgs) = %d, want 3 (summary + 2 details)", len(msgs))
	}
	if msgs[0] == "" || msgs[1] == "" {
		t.Fatal("expected non-empty summary and detail messages")
	}
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
