package query

import (
	"slices"
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

func TestVisualizationHintFromResult_NilResult(t *testing.T) {
	ct, reason := VisualizationHintFromResult(nil)
	if ct != ChartTable {
		t.Errorf("nil result should give ChartTable, got %q", ct)
	}
	if reason != "" {
		t.Errorf("nil result should give empty reason, got %q", reason)
	}
}

func TestPrimaryChartSuggestion_EmptyReturnsTable(t *testing.T) {
	ct := PrimaryChartSuggestion(nil)
	if ct != ChartTable {
		t.Errorf("PrimaryChartSuggestion(nil) = %q, want %q", ct, ChartTable)
	}
	ct = PrimaryChartSuggestion([]string{})
	if ct != ChartTable {
		t.Errorf("PrimaryChartSuggestion([]) = %q, want %q", ct, ChartTable)
	}
}

func TestFormatForDimension(t *testing.T) {
	tests := []struct {
		dimType string
		want    string
	}{
		{"date", FormatDate},
		{"DATE", FormatDate},
		{"timestamp", FormatDateTime},
		{"datetime", FormatDateTime},
		{"number", FormatNumber},
		{"numeric", FormatNumber},
		{"float", FormatNumber},
		{"double", FormatNumber},
		{"integer", FormatNumber},
		{"int", FormatNumber},
		{"boolean", FormatText},
		{"bool", FormatText},
		{"text", FormatText},
		{"unknown", FormatText},
		{"", FormatText},
	}
	for _, tt := range tests {
		t.Run(tt.dimType, func(t *testing.T) {
			got := formatForDimension(tt.dimType)
			if got != tt.want {
				t.Errorf("formatForDimension(%q) = %q, want %q", tt.dimType, got, tt.want)
			}
		})
	}
}

func TestFormatForTimeGrain_CalendarGrains(t *testing.T) {
	tests := []struct {
		grain    string
		fallback string
		want     string
	}{
		{TimeGrainDay, FormatNumber, FormatDate},
		{TimeGrainWeek, FormatNumber, FormatDate},
		{TimeGrainMonth, FormatText, FormatDate},
		{TimeGrainQuarter, FormatNumber, FormatDate},
		{TimeGrainYear, FormatText, FormatDate},
		{"hour", FormatNumber, FormatNumber},
		{"", FormatText, FormatText},
	}
	for _, tt := range tests {
		t.Run(tt.grain, func(t *testing.T) {
			got := formatForTimeGrain(tt.grain, tt.fallback)
			if got != tt.want {
				t.Errorf("formatForTimeGrain(%q, %q) = %q, want %q", tt.grain, tt.fallback, got, tt.want)
			}
		})
	}
}

func TestSuggestCharts_MultipleDimensions(t *testing.T) {
	cols := []ResultColumn{
		{Name: "region", SemanticType: SemanticTypeDimension, Format: FormatText},
		{Name: "product", SemanticType: SemanticTypeDimension, Format: FormatText},
	}
	suggestions := suggestCharts(cols, 10)
	want := []string{ChartTable}
	if !equalStringSlices(suggestions, want) {
		t.Errorf("suggestCharts(2 dims) = %v, want %v", suggestions, want)
	}
}

func TestSuggestCharts_NoDimsOneMetricManyRows(t *testing.T) {
	// dims==0 && metrics==0 → default table
	cols := []ResultColumn{
		{Name: "val", SemanticType: "", Format: ""},
	}
	suggestions := suggestCharts(cols, 10)
	want := []string{ChartTable}
	if !equalStringSlices(suggestions, want) {
		t.Errorf("suggestCharts(no semantic types) = %v, want %v", suggestions, want)
	}
}

func TestSuggestCharts_OneDimOneMetricMoreThan8Rows(t *testing.T) {
	cols := []ResultColumn{
		{Name: "country", SemanticType: SemanticTypeDimension, Format: FormatText},
		{Name: "revenue", SemanticType: SemanticTypeMetric, Format: FormatNumber},
	}
	suggestions := suggestCharts(cols, 20)
	want := []string{ChartBar, ChartTable}
	if !equalStringSlices(suggestions, want) {
		t.Errorf("suggestCharts(1 dim, >8 rows) = %v, want %v", suggestions, want)
	}
}

func TestSuggestCharts_OneMetricZeroRows(t *testing.T) {
	cols := []ResultColumn{
		{Name: "revenue", SemanticType: SemanticTypeMetric, Format: FormatNumber},
	}
	suggestions := suggestCharts(cols, 0)
	want := []string{ChartNumber, ChartTable}
	if !equalStringSlices(suggestions, want) {
		t.Errorf("suggestCharts(1 metric, 0 rows) = %v, want %v", suggestions, want)
	}
}

func TestDetectAnomalies_NilResult(t *testing.T) {
	if got := detectAnomalies(nil); got != nil {
		t.Errorf("detectAnomalies(nil) = %v, want nil", got)
	}
}

func TestDetectAnomalies_FewRows(t *testing.T) {
	result := &Result{
		Columns: []ResultColumn{{Name: "amount", SemanticType: SemanticTypeMetric, Format: FormatNumber}},
		Rows:    make([][]any, 5), // < anomalyMinRows (8)
	}
	if got := detectAnomalies(result); got != nil {
		t.Errorf("detectAnomalies(5 rows) = %v, want nil", got)
	}
}

func TestDetectAnomalies_NoMetricColumns(t *testing.T) {
	result := &Result{
		Columns: []ResultColumn{{Name: "name", SemanticType: SemanticTypeDimension, Format: FormatText}},
		Rows:    make([][]any, 10),
	}
	if got := detectAnomalies(result); got != nil {
		t.Errorf("detectAnomalies(no metric cols) = %v, want nil", got)
	}
}

func TestDetectAnomalies_IQRZero(t *testing.T) {
	rows := make([][]any, 12)
	for i := range rows {
		rows[i] = []any{float64(10)}
	}
	result := &Result{
		Columns: []ResultColumn{{Name: "amount", SemanticType: SemanticTypeMetric, Format: FormatNumber}},
		Rows:    rows,
	}
	// All values are the same → IQR = 0 → skip
	if got := detectAnomalies(result); got != nil {
		t.Errorf("detectAnomalies(iqr=0) = %v, want nil", got)
	}
}

func TestDetectAnomalies_NonFloatValuesSkipped(t *testing.T) {
	rows := make([][]any, 12)
	for i := range rows {
		rows[i] = []any{"string_value"}
	}
	result := &Result{
		Columns: []ResultColumn{{Name: "amount", SemanticType: SemanticTypeMetric, Format: FormatNumber}},
		Rows:    rows,
	}
	// No float values → no anomalies
	if got := detectAnomalies(result); got != nil {
		t.Errorf("detectAnomalies(string vals) = %v, want nil", got)
	}
}

func TestDetectAnomalies_CapsAtMaxFlags(t *testing.T) {
	rows := make([][]any, 30)
	for i := range rows {
		rows[i] = []any{float64(10 + float64(i%5))} // values 10-14
	}
	// Replace last few rows with extreme outliers
	for i := 25; i < 30; i++ {
		rows[i] = []any{float64(10000 + i)}
	}
	result := &Result{
		Columns: []ResultColumn{{Name: "amount", SemanticType: SemanticTypeMetric, Format: FormatNumber}},
		Rows:    rows,
	}
	anomalies := detectAnomalies(result)
	if len(anomalies) > anomalyMaxFlags {
		t.Errorf("got %d anomalies, want at most %d", len(anomalies), anomalyMaxFlags)
	}
	if len(anomalies) == 0 {
		t.Error("expected at least one anomaly")
	}
}

func TestQuartiles_EmptySlice(t *testing.T) {
	q1, q3 := quartiles(nil)
	if q1 != 0 || q3 != 0 {
		t.Errorf("quartiles(nil) = (%f, %f), want (0, 0)", q1, q3)
	}
	q1, q3 = quartiles([]float64{})
	if q1 != 0 || q3 != 0 {
		t.Errorf("quartiles([]) = (%f, %f), want (0, 0)", q1, q3)
	}
}

func TestQuartiles_SingleElement(t *testing.T) {
	q1, q3 := quartiles([]float64{5})
	if q1 != 5 || q3 != 5 {
		t.Errorf("quartiles([5]) = (%f, %f), want (5, 5)", q1, q3)
	}
}

func TestToFloat64(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  float64
		ok    bool
	}{
		{"float64", float64(42.5), 42.5, true},
		{"float32", float32(3.14), float64(float32(3.14)), true},
		{"int", 42, 42.0, true},
		{"int64", int64(100), 100.0, true},
		{"int32", int32(50), 50.0, true},
		{"string", "hello", 0, false},
		{"nil", nil, 0, false},
		{"bool", true, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := toFloat64(tt.input)
			if ok != tt.ok {
				t.Fatalf("toFloat64(%v) ok = %v, want %v", tt.input, ok, tt.ok)
			}
			if ok && got != tt.want {
				t.Errorf("toFloat64(%v) = %f, want %f", tt.input, got, tt.want)
			}
		})
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
	return slices.Equal(a, b)
}
