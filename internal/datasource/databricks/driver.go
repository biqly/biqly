// Package databricks implements the Databricks SQL warehouse datasource driver.
package databricks

import (
	"context"
	"database/sql"

	"github.com/biqly/biqly/internal/datasource"
	"github.com/biqly/biqly/internal/dialect"

	_ "github.com/databricks/databricks-sql-go" // register databricks driver
)

// Driver implements the datasource.Driver interface for Databricks.
type Driver struct {
	*datasource.BaseDriver
}

// NewDriver creates a new Databricks driver.
func NewDriver() *Driver {
	return &Driver{
		BaseDriver: datasource.NewBaseDriver("databricks", "databricks", dialect.Databricks, datasource.DefaultPoolLimits()),
	}
}

// Introspect discovers schemas, tables, columns and (when Unity Catalog is
// enabled) foreign-key relations.
func (d *Driver) Introspect(ctx context.Context, db *sql.DB) (*datasource.IntrospectionResult, error) {
	return datasource.ComposeIntrospection(ctx, db, d.Type(), datasource.IntrospectSteps{
		Schemas:   introspectSchemas,
		Tables:    introspectTables,
		Columns:   introspectColumns,
		Relations: introspectRelations,
	})
}

var _ datasource.Driver = (*Driver)(nil)
