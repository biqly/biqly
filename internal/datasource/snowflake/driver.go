// Package snowflake implements the Snowflake datasource driver.
package snowflake

import (
	"context"
	"database/sql"

	"github.com/biqly/biqly/internal/datasource"
	"github.com/biqly/biqly/internal/dialect"

	_ "github.com/snowflakedb/gosnowflake" // register snowflake driver
)

// Driver implements the datasource.Driver interface for Snowflake.
type Driver struct {
	*datasource.BaseDriver
}

// NewDriver creates a new Snowflake driver.
func NewDriver() *Driver {
	return &Driver{
		BaseDriver: datasource.NewBaseDriver("snowflake", "snowflake", dialect.Snowflake, datasource.DefaultPoolLimits()),
	}
}

// Introspect discovers schemas, tables and columns. Snowflake's
// INFORMATION_SCHEMA exposes no KEY_COLUMN_USAGE, so relations and primary-key
// flags are not populated (Phase 1 limitation).
func (d *Driver) Introspect(ctx context.Context, db *sql.DB) (*datasource.IntrospectionResult, error) {
	return datasource.ComposeIntrospection(ctx, db, d.Type(), datasource.IntrospectSteps{
		Schemas:   introspectSchemas,
		Tables:    introspectTables,
		Columns:   introspectColumns,
		Relations: introspectRelations,
	})
}

var _ datasource.Driver = (*Driver)(nil)
