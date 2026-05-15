package db

import (
	"context"
	"database/sql"
	"fmt"
)

// QuerySlice runs query with args and collects rows using scan.
func QuerySlice[T any](ctx context.Context, db *sql.DB, query string, args []any, scan func(Scanner) (T, error)) ([]T, error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []T
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
