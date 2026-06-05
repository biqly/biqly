package query

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/biqly/biqly/internal/security"
)

var scanSlicePool = sync.Pool{
	New: func() any {
		s := make([]any, 0, 32)
		return &s
	},
}

func borrowScanSlice(n int) (slice []any, pooled *[]any) {
	if vp, ok := scanSlicePool.Get().(*[]any); ok {
		return (*vp)[:n], vp
	}
	return make([]any, n), nil
}

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
	capacity := e.maxRows
	if capacity <= 0 {
		capacity = 64
	}
	count := 0

	var vals []any
	var valPtrs []any
	var valsPtr *[]any
	var valPtrsPtr *[]any

	if len(colTypes) <= 32 {
		vals, valsPtr = borrowScanSlice(len(colTypes))
		valPtrs, valPtrsPtr = borrowScanSlice(len(colTypes))
	} else {
		vals = make([]any, len(colTypes))
		valPtrs = make([]any, len(colTypes))
	}

	defer func() {
		if valsPtr != nil {
			for i := range *valsPtr {
				(*valsPtr)[i] = nil
			}
			scanSlicePool.Put(valsPtr)
		}
		if valPtrsPtr != nil {
			for i := range *valPtrsPtr {
				(*valPtrsPtr)[i] = nil
			}
			scanSlicePool.Put(valPtrsPtr)
		}
	}()

	for i := range vals {
		valPtrs[i] = &vals[i]
	}

	numCols := len(vals)
	allCells := make([]any, 0, capacity*numCols)

	truncated := false
	for rows.Next() {
		if e.maxRows > 0 && count >= e.maxRows {
			truncated = true
			break
		}

		if err := rows.Scan(valPtrs...); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}

		// Convert to plain values
		allCells = append(allCells, vals...)
		count++
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration error: %w", err)
	}

	resultRows := make([][]any, count)
	for i := range count {
		resultRows[i] = allCells[i*numCols : (i+1)*numCols]
	}

	duration := time.Since(start).Milliseconds()

	return &Result{
		Columns: columns,
		Rows:    resultRows,
		Stats: Stats{
			DurationMs: duration,
			RowCount:   len(resultRows),
			Truncated:  truncated,
		},
	}, nil
}
