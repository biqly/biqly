// Package oracle implements the Oracle Database datasource driver.
package oracle

import (
	"context"
	"database/sql"

	"github.com/biqly/biqly/internal/datasource"
	"github.com/biqly/biqly/internal/dialect"

	_ "github.com/sijms/go-ora/v2" // register oracle driver
)

// Driver implements the datasource.Driver interface for Oracle Database (12c+).
type Driver struct {
	*datasource.BaseDriver
}

// NewDriver creates a new Oracle driver.
func NewDriver() *Driver {
	return &Driver{
		BaseDriver: datasource.NewBaseDriver("oracle", "oracle", dialect.Oracle, datasource.DefaultPoolLimits()),
	}
}

// Introspect discovers schemas, tables, columns and relations from the ALL_*
// dictionary views, restricted to non-Oracle-maintained users.
func (d *Driver) Introspect(ctx context.Context, db *sql.DB) (*datasource.IntrospectionResult, error) {
	return datasource.ComposeIntrospection(ctx, db, d.Type(), datasource.IntrospectSteps{
		Schemas:   introspectSchemas,
		Tables:    introspectTables,
		Columns:   introspectColumns,
		Relations: introspectRelations,
	})
}

var _ datasource.Driver = (*Driver)(nil)
