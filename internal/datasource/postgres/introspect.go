package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/biqly/biqly/internal/datasource"
)

func (d *Driver) introspectSchemas(ctx context.Context, db *sql.DB) ([]datasource.SchemaInfo, error) {
	query := `
		SELECT schema_name
		FROM information_schema.schemata
		WHERE schema_name NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
		ORDER BY schema_name
	`
	return datasource.QueryAll(ctx, db, query, nil, func(rows *sql.Rows) (datasource.SchemaInfo, error) {
		var s datasource.SchemaInfo
		err := rows.Scan(&s.Name)
		return s, err
	})
}

func (d *Driver) introspectTables(ctx context.Context, db *sql.DB) ([]datasource.TableInfo, error) {
	query := `
		SELECT
			n.nspname AS schema_name,
			c.relname AS table_name,
			CASE c.relkind
				WHEN 'r' THEN 'BASE TABLE'
				WHEN 'p' THEN 'BASE TABLE'
				WHEN 'v' THEN 'VIEW'
				WHEN 'm' THEN 'VIEW'
				ELSE 'BASE TABLE'
			END AS table_type,
			NULLIF(c.reltuples, -1)::bigint AS row_estimate,
			COALESCE(pg_catalog.obj_description(c.oid, 'pg_class'), '') AS comment
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname NOT IN ('pg_catalog', 'information_schema')
			AND c.relkind IN ('r', 'p', 'v', 'm')
		ORDER BY n.nspname, c.relname
	`
	return datasource.QueryAll(ctx, db, query, nil, func(rows *sql.Rows) (datasource.TableInfo, error) {
		var t datasource.TableInfo
		err := rows.Scan(&t.SchemaName, &t.TableName, &t.TableType, &t.RowEstimate, &t.Comment)
		return t, err
	})
}

func (d *Driver) introspectColumns(ctx context.Context, db *sql.DB) ([]datasource.ColumnInfo, error) {
	query := `
		SELECT
			c.table_schema,
			c.table_name,
			c.column_name,
			c.data_type,
			CASE WHEN c.is_nullable = 'YES' THEN true ELSE false END as nullable,
			c.ordinal_position,
			c.character_maximum_length,
			c.numeric_precision,
			c.numeric_scale,
			COALESCE(c.column_default, '') AS column_default,
			COALESCE(
				pg_catalog.col_description(
					format('%I.%I', c.table_schema, c.table_name)::regclass,
					c.ordinal_position
				), ''
			) AS comment
		FROM information_schema.columns c
		WHERE c.table_schema NOT IN ('pg_catalog', 'information_schema')
		ORDER BY c.table_schema, c.table_name, c.ordinal_position
	`

	columns, err := datasource.QueryAll(ctx, db, query, nil, func(rows *sql.Rows) (datasource.ColumnInfo, error) {
		var c datasource.ColumnInfo
		err := rows.Scan(
			&c.SchemaName, &c.TableName, &c.ColumnName, &c.DataType,
			&c.Nullable, &c.OrdinalPosition, &c.CharMaxLength,
			&c.NumericPrecision, &c.NumericScale, &c.ColumnDefault, &c.Comment,
		)
		return c, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan columns: %w", err)
	}

	pkQuery := `
		SELECT 
			tc.table_schema,
			tc.table_name,
			kcu.column_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		WHERE tc.constraint_type = 'PRIMARY KEY'
	`

	pkRows, pkErr := datasource.QueryAll(ctx, db, pkQuery, nil, func(rows *sql.Rows) (struct {
		schema, table, column string
	}, error) {
		var p struct {
			schema, table, column string
		}
		err := rows.Scan(&p.schema, &p.table, &p.column)
		return p, err
	})
	if pkErr != nil {
		return columns, nil
	}

	pkSet := make(map[string]struct{}, len(pkRows))
	for _, p := range pkRows {
		key := fmt.Sprintf("%s.%s.%s", p.schema, p.table, p.column)
		pkSet[key] = struct{}{}
	}

	for i := range columns {
		key := fmt.Sprintf("%s.%s.%s", columns[i].SchemaName, columns[i].TableName, columns[i].ColumnName)
		if _, ok := pkSet[key]; ok {
			columns[i].IsPrimaryKey = true
		}
	}

	return columns, nil
}

func (d *Driver) introspectRelations(ctx context.Context, db *sql.DB) ([]datasource.RelationInfo, error) {
	query := `
		SELECT
			tc.constraint_name,
			tc.table_schema AS from_schema,
			tc.table_name AS from_table,
			kcu.column_name AS from_column,
			ccu.table_schema AS to_schema,
			ccu.table_name AS to_table,
			ccu.column_name AS to_column
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
			ON tc.constraint_name = kcu.constraint_name
			AND tc.table_schema = kcu.table_schema
		JOIN information_schema.constraint_column_usage ccu
			ON tc.constraint_name = ccu.constraint_name
			AND tc.table_schema = ccu.table_schema
		WHERE tc.constraint_type = 'FOREIGN KEY'
	`
	return datasource.QueryAll(ctx, db, query, nil, func(rows *sql.Rows) (datasource.RelationInfo, error) {
		var r datasource.RelationInfo
		r.RelationshipType = datasource.DefaultRelationshipType
		err := rows.Scan(
			&r.ConstraintName, &r.FromSchema, &r.FromTable, &r.FromColumn,
			&r.ToSchema, &r.ToTable, &r.ToColumn,
		)
		return r, err
	})
}
