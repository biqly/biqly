package db

import (
	"context"
	"database/sql"
	"fmt"
)

// querySliceInitialCap is the starting capacity for QuerySlice's result slice.
// Most repository list calls return on the order of tens to a few hundred
// rows; 64 amortizes the typical case to ~2 growth cycles instead of ~7.
const querySliceInitialCap = 64

// QuerySlice runs query with args and collects rows using scan.
func QuerySlice[T any](ctx context.Context, db *sql.DB, query string, args []any, scan func(Scanner) (T, error)) (out []T, err error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()

	out = make([]T, 0, querySliceInitialCap)
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

// QuerySliceErr wraps QuerySlice and prefixes errors with op.
func QuerySliceErr[T any](ctx context.Context, db *sql.DB, op, query string, args []any, scan func(Scanner) (T, error)) ([]T, error) {
	out, err := QuerySlice(ctx, db, query, args, scan)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", op, err)
	}
	return out, nil
}
