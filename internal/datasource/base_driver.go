package datasource

import (
	"context"
	"database/sql"
	"fmt"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

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
func (b *BaseDriver) Ping(ctx context.Context, dsn string) (err error) {
	ctx, span := otel.Tracer("biqly/datasource").Start(ctx, "datasource.Ping")
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()
	span.SetAttributes(attribute.String("db.system", b.typeName))
	if err = Ping(ctx, b.sqlDriver, dsn); err != nil {
		return fmt.Errorf("failed to open %s connection: %w", b.typeName, err)
	}
	return nil
}

// Open creates a connection pool.
func (b *BaseDriver) Open(ctx context.Context, dsn string) (db *sql.DB, err error) {
	ctx, span := otel.Tracer("biqly/datasource").Start(ctx, "datasource.Open")
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()
	span.SetAttributes(attribute.String("db.system", b.typeName))
	db, err = OpenPool(ctx, b.sqlDriver, dsn, b.poolLimits)
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
func ComposeIntrospection(ctx context.Context, db *sql.DB, dbSystem string, steps IntrospectSteps) (result *IntrospectionResult, err error) {
	ctx, span := otel.Tracer("biqly/datasource").Start(ctx, "datasource.Introspect")
	defer func() {
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, err.Error())
		}
		span.End()
	}()
	if dbSystem != "" {
		span.SetAttributes(attribute.String("db.system", dbSystem), attribute.String("datasource.driver", dbSystem))
	}

	schemas, err := introspectStep(ctx, "datasource.IntrospectSchemas", steps.Schemas, db)
	if err != nil {
		return nil, fmt.Errorf("introspect schemas: %w", err)
	}
	tables, err := introspectStep(ctx, "datasource.IntrospectTables", steps.Tables, db)
	if err != nil {
		return nil, fmt.Errorf("introspect tables: %w", err)
	}
	columns, err := introspectStep(ctx, "datasource.IntrospectColumns", steps.Columns, db)
	if err != nil {
		return nil, fmt.Errorf("introspect columns: %w", err)
	}
	relations, err := introspectStep(ctx, "datasource.IntrospectRelations", steps.Relations, db)
	if err != nil {
		return nil, fmt.Errorf("introspect relations: %w", err)
	}
	span.SetAttributes(
		attribute.Int("db.schema_count", len(schemas)),
		attribute.Int("db.table_count", len(tables)),
		attribute.Int("db.column_count", len(columns)),
		attribute.Int("db.relation_count", len(relations)),
	)
	return &IntrospectionResult{
		Schemas:   schemas,
		Tables:    tables,
		Columns:   columns,
		Relations: relations,
	}, nil
}

func introspectStep[T any](
	ctx context.Context,
	name string,
	fn func(context.Context, *sql.DB) ([]T, error),
	db *sql.DB,
) ([]T, error) {
	ctx, span := otel.Tracer("biqly/datasource").Start(ctx, name)
	defer span.End()
	out, err := fn(ctx, db)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		return nil, err
	}
	span.SetAttributes(attribute.Int("db.rows", len(out)))
	return out, nil
}
