package query

import (
	"fmt"
	"strings"
)

// VisualizationHintFromResult builds a frontend chart hint from enriched result metadata.
func VisualizationHintFromResult(result *Result) (chartType, reason string) {
	if result == nil {
		return ChartTable, ""
	}
	ct := PrimaryChartSuggestion(result.ChartSuggestions)
	reason = "Suggested from result shape (dimensions, metrics, row count)."
	if len(result.ChartSuggestions) > 1 {
		reason = "Primary suggestion: " + ct + "; alternatives: " + strings.Join(result.ChartSuggestions[1:], ", ")
	}
	if ct == ChartNumber {
		ct = ChartTable
		reason = "Single aggregate value — use table or KPI display."
	}
	return ct, reason
}

const anomalyWarningDetailLimit = 5

// AnomalyWarningMessages builds user-facing warnings for detected outliers.
func AnomalyWarningMessages(result *Result) []string {
	if result == nil || len(result.Anomalies) == 0 {
		return nil
	}
	n := len(result.Anomalies)
	out := make([]string, 0, 1+min(n, anomalyWarningDetailLimit))
	out = append(out, fmt.Sprintf("%d outlier value(s) detected in the result (IQR method). Review highlighted cells.", n))
	for i, a := range result.Anomalies {
		if i >= anomalyWarningDetailLimit {
			out = append(out, fmt.Sprintf("… and %d more outlier(s).", n-anomalyWarningDetailLimit))
			break
		}
		out = append(out, fmt.Sprintf("Row %d, column %q deviates strongly (score %.1f).", a.RowIndex+1, a.Column, a.Score))
	}
	return out
}
