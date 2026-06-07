// Package pgarray centralizes PostgreSQL array (text[]) encoding and decoding.
//
// It is the single place in the codebase that depends on the underlying driver
// helper (currently github.com/lib/pq). Migrating to pgx native types or
// pgtype later means changing only this package instead of every repository.
package pgarray

import "github.com/lib/pq"

// StringArray maps a Go []string to a PostgreSQL text[] column. It implements
// both driver.Valuer (for query parameters) and sql.Scanner (for scan targets),
// so it can be used directly as a scan destination:
//
//	var tags pgarray.StringArray
//	err := row.Scan(&tags)
type StringArray = pq.StringArray

// Strings wraps a []string for use as a query parameter targeting a text[]
// column:
//
//	_, err := db.ExecContext(ctx, q, pgarray.Strings(tags))
func Strings(v []string) any {
	return pq.Array(v)
}

// Scan wraps a pointer to a slice as a scan destination for a text[] column.
// Prefer the StringArray type for plain []string targets; use Scan when the
// destination is a field that must remain a concrete slice type:
//
//	err := row.Scan(pgarray.Scan(&dst))
func Scan(dst any) any {
	return pq.Array(dst)
}
