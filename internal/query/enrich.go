package query

import (
	"math"
	"sort"
	"strings"

	"github.com/biqly/biqly/internal/semantic"
)

// EnrichResult stamps each result column with its semantic role (dimension or
// metric) and a rendering format hint, and proposes a small set of chart types
// that fit the result shape. The enrichment is best-effort: columns produced by
// raw expressions or window functions that do not match a semantic entry are
// left untouched.
//
// EnrichResult resolves columns by SelectItem alias (or the dimension/metric
// name when no alias is set) so the rules stay aligned with the compiler.
func EnrichResult(result *Result, lq *LogicalQuery, model *semantic.SemanticModel) {
	if result == nil || model == nil {
		return
	}

	dimByName := make(map[string]*semantic.Dimension, len(model.Dimensions))
	for i := range model.Dimensions {
		dimByName[model.Dimensions[i].Name] = &model.Dimensions[i]
	}
	metricByName := make(map[string]*semantic.Metric, len(model.Metrics))
	for i := range model.Metrics {
		metricByName[model.Metrics[i].Name] = &model.Metrics[i]
	}

	timeGrainBySelect := make(map[string]string, len(lq.GroupBy))
	for _, gb := range lq.GroupBy {
		if gb.TimeGrain != "" {
			timeGrainBySelect[gb.Field] = gb.TimeGrain
		}
	}

	type role struct {
		semanticType string
		format       string
	}
	byAlias := make(map[string]role, len(lq.Select))
	for _, item := range lq.Select {
		alias := item.Alias
		if alias == "" {
			alias = item.Name
		}
		switch item.Type {
		case SelectTypeDimension:
			if dim, ok := dimByName[item.Name]; ok {
				format := formatForDimension(dim.Type)
				if grain := timeGrainBySelect[item.Name]; grain != "" {
					format = formatForTimeGrain(grain, format)
				}
				byAlias[alias] = role{semanticType: SemanticTypeDimension, format: format}
			}
		case SelectTypeMetric:
			if mt, ok := metricByName[item.Name]; ok {
				byAlias[alias] = role{semanticType: SemanticTypeMetric, format: formatForMetric(mt)}
			}
		case SelectTypeWindow:
			byAlias[alias] = role{semanticType: SemanticTypeMetric, format: FormatNumber}
		case SelectTypeFormula:
			byAlias[alias] = role{semanticType: SemanticTypeMetric, format: formatForFormula(item.Formula)}
		}
	}

	for i := range result.Columns {
		if r, ok := byAlias[result.Columns[i].Name]; ok {
			result.Columns[i].SemanticType = r.semanticType
			result.Columns[i].Format = r.format
		}
	}

	result.ChartSuggestions = suggestCharts(result.Columns, len(result.Rows))
	result.PivotHint = suggestPivot(result.Columns)
	result.Anomalies = detectAnomalies(result)
}

// PrimaryChartSuggestion returns the preferred chart type for UI defaults.
func PrimaryChartSuggestion(suggestions []string) string {
	if len(suggestions) == 0 {
		return ChartTable
	}
	return suggestions[0]
}

// formatForMetric honours an explicit semantic-model metric format, falling
// back to FormatNumber when unset or unrecognised.
func formatForMetric(m *semantic.Metric) string {
	if m == nil || m.Format == nil {
		return FormatNumber
	}
	switch strings.ToLower(strings.TrimSpace(*m.Format)) {
	case FormatPercent:
		return FormatPercent
	case FormatCurrency:
		return FormatCurrency
	case FormatNumber:
		return FormatNumber
	default:
		return FormatNumber
	}
}

// formatForFormula maps a formula operator to a rendering hint. percent_of and
// percent_change already compile to a 0-100 scaled value, so they render as
// percentages; other operators (difference, ratio) stay plain numbers.
func formatForFormula(f *FormulaSpec) string {
	if f == nil {
		return FormatNumber
	}
	switch f.Op {
	case FormulaOpPercentOf, FormulaOpPercentChange:
		return FormatPercent
	default:
		return FormatNumber
	}
}

func formatForDimension(dimType string) string {
	switch strings.ToLower(strings.TrimSpace(dimType)) {
	case "date":
		return FormatDate
	case "timestamp", "datetime":
		return FormatDateTime
	case "number", "numeric", "float", "double", "integer", "int":
		return FormatNumber
	case "boolean", "bool":
		return FormatText
	default:
		return FormatText
	}
}

// formatForTimeGrain maps a time grain to a rendering hint. day/week are
// truncated to a real date (FormatDate). month/quarter compile to EXTRACT
// ordinals (1-12, 1-4), so they get their own formats and the frontend renders
// localized names while keeping the integer for sorting. year (a 4-digit
// integer) and hour (0-23) are plain numbers.
func formatForTimeGrain(grain, fallback string) string {
	switch grain {
	case TimeGrainMonth:
		return FormatMonthOfYear
	case TimeGrainQuarter:
		return FormatQuarter
	case TimeGrainDay, TimeGrainWeek:
		return FormatDate
	case TimeGrainYear, TimeGrainHour:
		return FormatNumber
	}
	return fallback
}

// isTimeDimFormat reports whether a dimension format represents a calendar
// bucket (date or a month/quarter ordinal). These share time-series chart and
// pivot treatment even though month/quarter values are integers.
func isTimeDimFormat(format string) bool {
	switch format {
	case FormatDate, FormatDateTime, FormatMonthOfYear, FormatQuarter:
		return true
	default:
		return false
	}
}

// suggestCharts returns a short, ordered list of chart types appropriate for
// the result shape. Rules favour the smallest sensible default first; the
// frontend can show alternatives below it.
func suggestCharts(columns []ResultColumn, rowCount int) []string {
	dims, metrics, hasTimeDim := 0, 0, false
	for _, c := range columns {
		switch c.SemanticType {
		case SemanticTypeDimension:
			dims++
			if isTimeDimFormat(c.Format) {
				hasTimeDim = true
			}
		case SemanticTypeMetric:
			metrics++
		}
	}

	switch {
	case dims == 0 && metrics == 1 && rowCount <= 1:
		return []string{ChartNumber, ChartTable}
	case hasTimeDim && metrics >= 1:
		return []string{ChartLine, ChartBar, ChartTable}
	case dims == 1 && metrics >= 1:
		if rowCount > 0 && rowCount <= 8 {
			return []string{ChartBar, ChartPie, ChartTable}
		}
		return []string{ChartBar, ChartTable}
	case dims >= 2:
		return []string{ChartTable}
	default:
		return []string{ChartTable}
	}
}

// suggestPivot recommends a pivot layout when there are two categorical dimensions
// and at least one metric (typical cross-tab shape).
func suggestPivot(columns []ResultColumn) *PivotHint {
	catDims := make([]string, 0, len(columns))
	metrics := make([]string, 0, len(columns))
	for _, c := range columns {
		switch c.SemanticType {
		case SemanticTypeDimension:
			if isTimeDimFormat(c.Format) {
				continue
			}
			catDims = append(catDims, c.Name)
		case SemanticTypeMetric:
			metrics = append(metrics, c.Name)
		}
	}
	if len(catDims) < 2 || len(metrics) == 0 {
		return nil
	}
	return &PivotHint{
		RowField:    catDims[0],
		ColumnField: catDims[1],
		ValueFields: append([]string(nil), metrics...),
		Reason:      "Two categorical dimensions with metrics — pivot for a matrix view.",
	}
}

const (
	anomalyMinRows   = 8
	anomalyMaxFlags  = 20
	anomalyIQRFactor = 1.5
)

// detectAnomalies flags metric cells outside the IQR fence (Q1 - 1.5*IQR, Q3 + 1.5*IQR).
func detectAnomalies(result *Result) []Anomaly {
	if result == nil || len(result.Rows) < anomalyMinRows {
		return nil
	}
	var out []Anomaly
	for colIdx, col := range result.Columns {
		if col.SemanticType != SemanticTypeMetric {
			continue
		}
		vals := make([]float64, 0, len(result.Rows))
		rowIdxs := make([]int, 0, len(result.Rows))
		for ri, row := range result.Rows {
			if colIdx >= len(row) {
				continue
			}
			v, ok := toFloat64(row[colIdx])
			if !ok {
				continue
			}
			vals = append(vals, v)
			rowIdxs = append(rowIdxs, ri)
		}
		if len(vals) < anomalyMinRows {
			continue
		}
		q1, q3 := quartiles(vals)
		iqr := q3 - q1
		if iqr == 0 {
			continue
		}
		low := q1 - anomalyIQRFactor*iqr
		high := q3 + anomalyIQRFactor*iqr
		for i, v := range vals {
			if v < low || v > high {
				dev := math.Max(low-v, v-high) / iqr
				out = append(out, Anomaly{
					RowIndex: rowIdxs[i],
					Column:   col.Name,
					Value:    result.Rows[rowIdxs[i]][colIdx],
					Score:    dev,
				})
			}
		}
	}
	if len(out) == 0 {
		return nil
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	if len(out) > anomalyMaxFlags {
		out = out[:anomalyMaxFlags]
	}
	return out
}

func quartiles(sortedVals []float64) (q1, q3 float64) {
	cp := append([]float64(nil), sortedVals...)
	sort.Float64s(cp)
	n := len(cp)
	if n == 0 {
		return 0, 0
	}
	q1 = cp[n/4]
	q3 = cp[(3*n)/4]
	return q1, q3
}

func toFloat64(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int64:
		return float64(x), true
	case int32:
		return float64(x), true
	default:
		return 0, false
	}
}
