package query

import (
	"fmt"
	"github.com/bytedance/sonic"
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
	// FormatMonthOfYear marks a month time-grain column whose value is the
	// month-of-year ordinal (1-12) produced by EXTRACT(MONTH …), not a date.
	// The frontend renders it as a localized month name while preserving the
	// integer for sorting.
	FormatMonthOfYear = "month_of_year"
	// FormatQuarter marks a quarter time-grain column whose value is the
	// quarter-of-year ordinal (1-4) produced by EXTRACT(QUARTER …). The
	// frontend renders it as a localized "Q{n}" label; the integer sorts.
	FormatQuarter = "quarter"
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
	// Truncated reports whether the underlying SQL produced more rows than
	// the configured max-rows cap. Clients can use this to surface "showing
	// first N rows" rather than silently presenting a partial result set.
	Truncated bool `json:"truncated,omitempty"`
	// TotalCount is the total number of rows the query would return without
	// LIMIT/OFFSET. Populated by QueryService when pagination is requested;
	// zero/omitted when unknown or when the count query was skipped.
	TotalCount int `json:"total_count,omitempty"`
}

// CompiledQuery holds the SQL generated from a LogicalQuery.
type CompiledQuery struct {
	SQL  string
	Args []any
	// Policy records the security policy decisions applied during
	// compilation (RLS predicates, PII masking). Nil when no policy applied.
	Policy *PolicyDecisions `json:"policy,omitempty"`
}

// AppliedRowFilter is one row-level security predicate merged into the query.
type AppliedRowFilter struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    any    `json:"value,omitempty"`
}

// PolicyDecisions records which security policies were applied at compile
// time, so audit can prove enforcement rather than infer it.
type PolicyDecisions struct {
	RowFilters    []AppliedRowFilter `json:"row_filters,omitempty"`
	MaskedColumns []string           `json:"masked_columns,omitempty"`
	HiddenColumns []string           `json:"hidden_columns,omitempty"`
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
	Field               string   `json:"field"`
	Code                string   `json:"code,omitempty"`
	Message             string   `json:"message"`
	Value               string   `json:"value,omitempty"`
	AllowedAlternatives []string `json:"allowed_alternatives,omitempty"`
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
	parts := make([]string, 0, len(ve)+1)
	parts = append(parts, "validation failed:")
	for _, e := range ve {
		parts = append(parts, fmt.Sprintf(" %s: %s;", e.Field, e.Message))
	}
	return strings.Join(parts, "")
}

// HasCode reports whether the collection contains any error with the given code.
func (ve ValidationErrors) HasCode(code string) bool {
	for _, err := range ve {
		if err.Code == code {
			return true
		}
	}
	return false
}

// FilterByCode filters the collection to errors matching the given code.
func (ve ValidationErrors) FilterByCode(code string) []*ValidationError {
	var res []*ValidationError
	for _, err := range ve {
		if err.Code == code {
			res = append(res, err)
		}
	}
	return res
}

// ToRepairJSON serializes the validation errors into a compact JSON array
// intended for LLM repair prompting.
func (ve ValidationErrors) ToRepairJSON() string {
	if len(ve) == 0 {
		return "[]"
	}
	data, err := sonic.ConfigStd.Marshal(ve)
	if err != nil {
		return "[]"
	}
	return string(data)
}
