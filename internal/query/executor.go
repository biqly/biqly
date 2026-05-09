package query

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/biqly/biqly/internal/security"
)

// Executor runs compiled SQL queries against a database.
type Executor struct {
	maxRows int
	timeout time.Duration
	checker *security.ReadOnlyChecker
}

// NewExecutor creates a new query executor.
func NewExecutor(maxRows int, timeout time.Duration) *Executor {
	return &Executor{
		maxRows: maxRows,
		timeout: timeout,
		checker: security.NewReadOnlyChecker(),
	}
}

// Execute runs a compiled query and returns results.
func (e *Executor) Execute(ctx context.Context, db *sql.DB, cq *CompiledQuery) (*QueryResult, error) {
	// Safety check
	if err := e.checker.Check(cq.SQL); err != nil {
		return nil, fmt.Errorf("security check failed: %w", err)
	}

	// Apply timeout
	if e.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, e.timeout)
		defer cancel()
	}

	start := time.Now()

	// Execute query
	rows, err := db.QueryContext(ctx, cq.SQL, cq.Args...)
	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	defer func() { _ = rows.Close() }()

	// Get column info
	colTypes, err := rows.ColumnTypes()
	if err != nil {
		return nil, fmt.Errorf("get column types: %w", err)
	}

	columns := make([]ResultColumn, len(colTypes))
	for i, ct := range colTypes {
		columns[i] = ResultColumn{
			Name: ct.Name(),
			Type: ct.DatabaseTypeName(),
		}
	}

	// Read rows
	var resultRows [][]any
	count := 0
	for rows.Next() {
		if e.maxRows > 0 && count >= e.maxRows {
			break
		}

		// Create scan targets
		vals := make([]any, len(colTypes))
		valPtrs := make([]any, len(colTypes))
		for i := range vals {
			valPtrs[i] = &vals[i]
		}

		if err := rows.Scan(valPtrs...); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}

		// Convert to plain values
		row := make([]any, len(vals))
		copy(row, vals)
		resultRows = append(resultRows, row)
		count++
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	duration := time.Since(start).Milliseconds()

	return &Result{
		Columns: columns,
		Rows:    resultRows,
		Stats: Stats{
			DurationMs: duration,
			RowCount:   len(resultRows),
		},
	}, nil
}
