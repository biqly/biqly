package query

import (
	"fmt"
	"time"
)

// QueryResult holds the execution result.
type QueryResult struct {
	Columns []ResultColumn `json:"columns"`
	Rows    [][]any        `json:"rows"`
	Stats   QueryStats     `json:"stats"`
}

// ResultColumn describes a result column.
type ResultColumn struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// QueryStats holds execution statistics.
type QueryStats struct {
	DurationMs int64 `json:"duration_ms"`
	RowCount   int   `json:"row_count"`
}

// CompiledQuery holds the SQL generated from a LogicalQuery.
type CompiledQuery struct {
	SQL  string
	Args []any
}

// QueryHistoryEntry represents a stored query in history.
type QueryHistoryEntry struct {
	ID             string       `json:"id"`
	DatasourceID   string       `json:"datasource_id"`
	ModelID        *string      `json:"model_id"`
	UserID         *string      `json:"user_id"`
	LogicalQuery   LogicalQuery `json:"logical_query"`
	CompiledSQL    *string      `json:"compiled_sql"`
	SQLArgs        *string      `json:"sql_args"`
	Status         string       `json:"status"`
	RowCount       *int         `json:"row_count"`
	DurationMs     *int         `json:"duration_ms"`
	ErrorMessage   *string      `json:"error_message"`
	CreatedAt      time.Time    `json:"created_at"`
}

// ValidationError represents a LogicalQuery validation error.
type ValidationError struct {
	Field   string `json:"field"`
	Message string `json:"message"`
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on %s: %s", e.Field, e.Message)
}

// ValidationErrors is a collection of validation errors.
type ValidationErrors []*ValidationError

func (ve ValidationErrors) Error() string {
	if len(ve) == 0 {
		return ""
	}
	msg := "validation failed:"
	for _, e := range ve {
		msg += fmt.Sprintf(" %s: %s;", e.Field, e.Message)
	}
	return msg
}
