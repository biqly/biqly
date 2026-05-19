package query

import (
	"fmt"
	"strings"
	"time"

	"github.com/biqly/biqly/pkg/logicalquery"
)

// Result holds the execution result.
type Result struct {
	Columns []ResultColumn `json:"columns"`
	Rows    [][]any        `json:"rows"`
	Stats   Stats          `json:"stats"`
	// ChartSuggestions is a frontend hint: a small ordered list of chart types
	// that would render this result well ("bar", "line", "table", "number").
	// Populated by EnrichResult based on the selected dimensions/metrics; empty
	// when no semantic-model context is available.
	ChartSuggestions []string `json:"chart_suggestions,omitempty"`
	// PivotHint suggests row/column fields when the result is a good pivot candidate.
	PivotHint *PivotHint `json:"pivot_hint,omitempty"`
	// Anomalies flags outlier metric values (IQR method) for post-query inspection.
	Anomalies []Anomaly `json:"anomalies,omitempty"`
}

// PivotHint tells the frontend how to lay out a wide result as a pivot table.
type PivotHint struct {
	RowField    string   `json:"row_field"`
	ColumnField string   `json:"column_field"`
	ValueFields []string `json:"value_fields"`
	Reason      string   `json:"reason,omitempty"`
}

// Anomaly marks one cell that deviates strongly from the column distribution.
type Anomaly struct {
	RowIndex int     `json:"row_index"`
	Column   string  `json:"column"`
	Value    any     `json:"value"`
	Score    float64 `json:"score"`
}

// ResultColumn describes a result column.
type ResultColumn struct {
	Name string `json:"name"`
	Type string `json:"type"`
	// SemanticType identifies the role of this column in the semantic layer:
	// "dimension", "metric", or "" when the column did not map to any semantic
	// entity (raw SQL escape hatches, expressions, etc.).
	SemanticType string `json:"semantic_type,omitempty"`
	// Format is a frontend rendering hint. Common values: "number", "currency",
	// "percent", "date", "datetime", "text".
	Format string `json:"format,omitempty"`
}

// Semantic-type values for ResultColumn.SemanticType.
const (
	SemanticTypeDimension = "dimension"
	SemanticTypeMetric    = "metric"
)

// Format values for ResultColumn.Format.
const (
	FormatNumber   = "number"
	FormatCurrency = "currency"
	FormatPercent  = "percent"
	FormatDate     = "date"
	FormatDateTime = "datetime"
	FormatText     = "text"
)

// Chart suggestion identifiers.
const (
	ChartBar    = "bar"
	ChartLine   = "line"
	ChartTable  = "table"
	ChartNumber = "number"
	ChartPie    = "pie"
)

// Stats holds execution statistics.
type Stats struct {
	DurationMs int64 `json:"duration_ms"`
	RowCount   int   `json:"row_count"`
}

// CompiledQuery holds the SQL generated from a LogicalQuery.
type CompiledQuery struct {
	SQL  string
	Args []any
}

// HistoryEntry represents a stored query in history.
type HistoryEntry struct {
	ID           string                    `json:"id"`
	DatasourceID string                    `json:"datasource_id"`
	ModelID      *string                   `json:"model_id"`
	UserID       *string                   `json:"user_id"`
	LogicalQuery logicalquery.LogicalQuery `json:"logical_query"`
	CompiledSQL  *string                   `json:"compiled_sql"`
	SQLArgs      *string                   `json:"sql_args"`
	Status       string                    `json:"status"`
	RowCount     *int                      `json:"row_count"`
	DurationMs   *int                      `json:"duration_ms"`
	ErrorMessage *string                   `json:"error_message"`
	// Fingerprint groups runs of the same canonical LogicalQuery under the same
	// semantic-model version and permission scope. See ComputeFingerprint.
	Fingerprint string    `json:"fingerprint,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

// ValidationError represents a LogicalQuery validation error.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

// Error formats the validation error for log messages and API envelopes.
func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on %s: %s", e.Field, e.Message)
}

// ValidationErrors is a collection of validation errors.
type ValidationErrors []*ValidationError

// Error joins every validation message into a single human-readable string.
func (ve ValidationErrors) Error() string {
	if len(ve) == 0 {
		return ""
	}
	var msg strings.Builder
	msg.WriteString("validation failed:")
	for _, e := range ve {
		fmt.Fprintf(&msg, " %s: %s;", e.Field, e.Message)
	}
	return msg.String()
}
