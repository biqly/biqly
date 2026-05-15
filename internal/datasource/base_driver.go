package datasource

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/biqly/biqly/internal/dialect"
)

// BaseDriver implements Driver.Type, Ping, Open, and Dialect for SQL backends.
// Concrete drivers embed it and supply Introspect.
type BaseDriver struct {
	typeName   string
	sqlDriver  string
	dialectVal dialect.Dialect
	poolLimits PoolLimits
}

// NewBaseDriver builds shared driver plumbing for a SQL backend.
func NewBaseDriver(typeName, sqlDriver string, d dialect.Dialect, limits PoolLimits) *BaseDriver {
	return &BaseDriver{
		typeName:   typeName,
		sqlDriver:  sqlDriver,
		dialectVal: d,
		poolLimits: limits,
	}
}

// Type returns the driver type identifier.
func (b *BaseDriver) Type() string {
	return b.typeName
}

// Dialect returns the SQL dialect for this driver.
func (b *BaseDriver) Dialect() dialect.Dialect {
	return b.dialectVal
}

// Ping verifies connectivity.
func (b *BaseDriver) Ping(ctx context.Context, dsn string) error {
	if err := Ping(ctx, b.sqlDriver, dsn); err != nil {
		return fmt.Errorf("failed to open %s connection: %w", b.typeName, err)
	}
	return nil
}

// Open creates a connection pool.
func (b *BaseDriver) Open(ctx context.Context, dsn string) (*sql.DB, error) {
	db, err := OpenPool(ctx, b.sqlDriver, dsn, b.poolLimits)
	if err != nil {
		return nil, fmt.Errorf("failed to open %s connection: %w", b.typeName, err)
	}
	return db, nil
}

// IntrospectSteps runs schema introspection helpers in order.
type IntrospectSteps struct {
	Schemas   func(context.Context, *sql.DB) ([]SchemaInfo, error)
	Tables    func(context.Context, *sql.DB) ([]TableInfo, error)
	Columns   func(context.Context, *sql.DB) ([]ColumnInfo, error)
	Relations func(context.Context, *sql.DB) ([]RelationInfo, error)
}

// ComposeIntrospection runs introspection steps and assembles the result.
func ComposeIntrospection(ctx context.Context, db *sql.DB, steps IntrospectSteps) (*IntrospectionResult, error) {
	schemas, err := steps.Schemas(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("introspect schemas: %w", err)
	}
	tables, err := steps.Tables(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("introspect tables: %w", err)
	}
	columns, err := steps.Columns(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("introspect columns: %w", err)
	}
	relations, err := steps.Relations(ctx, db)
	if err != nil {
		return nil, fmt.Errorf("introspect relations: %w", err)
	}
	return &IntrospectionResult{
		Schemas:   schemas,
		Tables:    tables,
		Columns:   columns,
		Relations: relations,
	}, nil
}
