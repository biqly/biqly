// Package postgres implements the datasource.Driver interface for PostgreSQL.
package postgres

import (
	"context"
	"database/sql"

	"github.com/biqly/biqly/internal/datasource"
	"github.com/biqly/biqly/internal/dialect"

	_ "github.com/jackc/pgx/v5/stdlib" // register pgx driver
)

// Driver implements the datasource.Driver interface for PostgreSQL.
type Driver struct {
	*datasource.BaseDriver
}

// NewDriver creates a new PostgreSQL driver.
func NewDriver() *Driver {
	return &Driver{
		BaseDriver: datasource.NewBaseDriver("postgres", "pgx", dialect.Postgres, datasource.DefaultPoolLimits()),
	}
}

// SupportsReadOnlyTx: pgx issues BEGIN READ ONLY for sql.TxOptions{ReadOnly:true},
// so a write inside the transaction is rejected by PostgreSQL itself.
func (*Driver) SupportsReadOnlyTx() bool {
	return true
}

// Introspect discovers the schema of a PostgreSQL database.
func (d *Driver) Introspect(ctx context.Context, db *sql.DB) (*datasource.IntrospectionResult, error) {
	return datasource.ComposeIntrospection(ctx, db, d.Type(), datasource.IntrospectSteps{
		Schemas:   d.introspectSchemas,
		Tables:    d.introspectTables,
		Columns:   d.introspectColumns,
		Relations: d.introspectRelations,
	})
}

var _ datasource.Driver = (*Driver)(nil)
