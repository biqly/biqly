// Package postgres implements the datasource.Driver interface for PostgreSQL.
package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/biqly/biqly/internal/datasource"
	"github.com/biqly/biqly/internal/dialect"

	_ "github.com/jackc/pgx/v5/stdlib" // register pgx driver
)

// Driver implements the datasource.Driver interface for PostgreSQL.
type Driver struct {
	dialect dialect.Dialect
}

// NewDriver creates a new PostgreSQL driver.
func NewDriver() *Driver {
	return &Driver{
		dialect: dialect.PostgresDialect{},
	}
}

// Type returns the driver type identifier.
func (d *Driver) Type() string {
	return "postgres"
}

// Ping tests connectivity to a PostgreSQL instance.
func (d *Driver) Ping(ctx context.Context, dsn string) error {
	if err := datasource.Ping(ctx, "pgx", dsn); err != nil {
		return fmt.Errorf("failed to open postgres connection: %w", err)
	}
	return nil
}

// Open establishes a connection pool to PostgreSQL.
func (d *Driver) Open(ctx context.Context, dsn string) (*sql.DB, error) {
	db, err := datasource.OpenPool(ctx, "pgx", dsn, datasource.DefaultPoolLimits())
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres connection: %w", err)
	}
	return db, nil
}

// Dialect returns the PostgreSQL SQL dialect.
func (d *Driver) Dialect() dialect.Dialect {
	return d.dialect
}

// Introspect discovers the schema of a PostgreSQL database.
func (d *Driver) Introspect(ctx context.Context, db *sql.DB) (*datasource.IntrospectionResult, error) {
	schemas, err := d.introspectSchemas(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("introspect schemas: %w", err)
	}

	tables, err := d.introspectTables(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("introspect tables: %w", err)
	}

	columns, err := d.introspectColumns(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("introspect columns: %w", err)
	}

	relations, err := d.introspectRelations(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("introspect relations: %w", err)
	}

	return &datasource.IntrospectionResult{
		Schemas:   schemas,
		Tables:    tables,
		Columns:   columns,
		Relations: relations,
	}, nil
}
