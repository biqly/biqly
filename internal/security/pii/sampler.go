package pii

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/biqly/biqly/internal/dialect"
	pkgmetadata "github.com/biqly/biqly/pkg/metadata"
)

// NewDBSampleFetcher builds a SampleFetcher that reads sample values directly
// from a live datasource connection using its SQL dialect. Identifiers are
// quoted through the dialect; no user input reaches the query.
func NewDBSampleFetcher(db *sql.DB, d dialect.Dialect) SampleFetcher {
	return func(ctx context.Context, col pkgmetadata.Column, limit int) ([]string, error) {
		colRef := d.QuoteIdentSegment(col.ColumnName)
		tableRef := d.QuoteIdentSegment(col.SchemaName) + "." + d.QuoteIdentSegment(col.TableName)
		// SelectWithLimit owns LIMIT vs TOP placement per dialect; the WHERE
		// clause rides along with the table reference so it lands before the
		// trailing LIMIT and after the projection in both syntaxes.
		query := d.SelectWithLimit(
			[]string{colRef},
			tableRef+" WHERE "+colRef+" IS NOT NULL",
			limit,
		)

		rows, err := db.QueryContext(ctx, query) // nosemgrep: go.lang.security.audit.database.string-formatted-query.string-formatted-query
		if err != nil {
			return nil, fmt.Errorf("sample %s: %w", tableRef, err)
		}
		defer func() { _ = rows.Close() }()

		samples := make([]string, 0, limit)
		for rows.Next() {
			var v sql.NullString
			if err := rows.Scan(&v); err != nil {
				return nil, fmt.Errorf("scan sample value: %w", err)
			}
			if v.Valid {
				samples = append(samples, v.String)
			}
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("iterate samples: %w", err)
		}
		return samples, nil
	}
}
