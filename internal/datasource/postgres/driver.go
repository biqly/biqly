package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/biqly/biqly/internal/datasource"
	"github.com/biqly/biqly/internal/dialect"
	_ "github.com/jackc/pgx/v5/stdlib"
)

// Driver implements datasource.Driver for PostgreSQL.
type Driver struct {
	dialect dialect.Dialect
}

// NewDriver creates a new PostgreSQL driver.
func NewDriver() *Driver {
	return &Driver{
		dialect: dialect.PostgresDialect{},
	}
}

func (d *Driver) Type() string {
	return "postgres"
}

func (d *Driver) Ping(ctx context.Context, dsn string) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("failed to open postgres connection: %w", err)
	}
	defer db.Close()
	return db.PingContext(ctx)
}

func (d *Driver) Open(ctx context.Context, dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open postgres connection: %w", err)
	}

	// Set reasonable defaults
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	return db, nil
}

func (d *Driver) Dialect() dialect.Dialect {
	return d.dialect
}

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
