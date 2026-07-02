// Package sqlite implements the SQLite datasource driver.
package sqlite

import (
	"context"
	"database/sql"

	"github.com/biqly/biqly/internal/datasource"
	"github.com/biqly/biqly/internal/dialect"

	_ "modernc.org/sqlite" // register sqlite driver
)

// Driver implements the datasource.Driver interface for SQLite.
type Driver struct {
	*datasource.BaseDriver
}

// NewDriver creates a new SQLite driver.
func NewDriver() *Driver {
	return &Driver{
		BaseDriver: datasource.NewBaseDriver("sqlite", "sqlite", dialect.SQLite, datasource.DefaultPoolLimits()),
	}
}

// Introspect discovers schemas, tables, columns and relations.
func (d *Driver) Introspect(ctx context.Context, db *sql.DB) (*datasource.IntrospectionResult, error) {
	return datasource.ComposeIntrospection(ctx, db, d.Type(), datasource.IntrospectSteps{
		Schemas:   introspectSchemas,
		Tables:    introspectTables,
		Columns:   introspectColumns,
		Relations: introspectRelations,
	})
}

var _ datasource.Driver = (*Driver)(nil)
