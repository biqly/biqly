package datasource

import (
	"context"
	"database/sql"
)

// QueryAll runs query and collects one element per row using scan.
// It closes rows and returns rows.Err() after iteration.
func QueryAll[T any](ctx context.Context, db *sql.DB, query string, args []any, scan func(*sql.Rows) (T, error)) (out []T, err error) {
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()
	out = make([]T, 0, 64)
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, rowsErr
	}
	return out, nil
}
