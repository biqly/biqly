package query

import (
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
func EnrichResult(result *Result, lq LogicalQuery, model *semantic.SemanticModel) {
	if result == nil || model == nil {
		return
	}

	dimByName := make(map[string]semantic.Dimension, len(model.Dimensions))
	for _, d := range model.Dimensions {
		dimByName[d.Name] = d
	}
	metricByName := make(map[string]semantic.Metric, len(model.Metrics))
	for _, m := range model.Metrics {
		metricByName[m.Name] = m
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
			if metric, ok := metricByName[item.Name]; ok {
				byAlias[alias] = role{semanticType: SemanticTypeMetric, format: formatForMetric(metric)}
			}
		case SelectTypeWindow:
			byAlias[alias] = role{semanticType: SemanticTypeMetric, format: FormatNumber}
		}
	}

	for i := range result.Columns {
		if r, ok := byAlias[result.Columns[i].Name]; ok {
			result.Columns[i].SemanticType = r.semanticType
			result.Columns[i].Format = r.format
		}
	}

	result.ChartSuggestions = suggestCharts(result.Columns, len(result.Rows))
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

func formatForTimeGrain(grain, fallback string) string {
	switch grain {
	case TimeGrainDay, TimeGrainWeek, TimeGrainMonth, TimeGrainQuarter, TimeGrainYear:
		return FormatDate
	}
	return fallback
}

func formatForMetric(metric semantic.Metric) string {
	switch strings.ToLower(string(metric.Aggregation)) {
	case "count", "count_distinct":
		return FormatNumber
	}
	// Without dedicated unit metadata on the metric we cannot reliably infer
	// currency vs. plain number. Leave it as number; UI can override via the
	// metric's label/description.
	return FormatNumber
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
			if c.Format == FormatDate || c.Format == FormatDateTime {
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
