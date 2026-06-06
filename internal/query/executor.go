package query

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

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
		if cap(*vp) >= n {
			return (*vp)[:n], vp
		}
		scanSlicePool.Put(vp)
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
func (e *Executor) Execute(ctx context.Context, db *sql.DB, cq *CompiledQuery) (result *Result, err error) {
	ctx, span := otel.Tracer("biqly/query").Start(ctx, "query.Execute")
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()

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

	rows, queryErr := db.QueryContext(ctx, cq.SQL, cq.Args...)
	if queryErr != nil {
		return nil, fmt.Errorf("query execution failed: %w", queryErr)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	res, scanErr := e.scanRows(rows, start)
	if scanErr != nil {
		return nil, scanErr
	}

	span.SetAttributes(
		attribute.Int("db.rows_returned", res.Stats.RowCount),
		attribute.Bool("db.truncated", res.Stats.Truncated),
	)

	return res, nil
}

func (e *Executor) scanRows(rows *sql.Rows, start time.Time) (*Result, error) {
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

	return &Result{
		Columns: columns,
		Rows:    resultRows,
		Stats: Stats{
			DurationMs: time.Since(start).Milliseconds(),
			RowCount:   len(resultRows),
			Truncated:  truncated,
		},
	}, nil
}
