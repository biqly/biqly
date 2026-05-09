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

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var schemas []datasource.SchemaInfo
	for rows.Next() {
		var s datasource.SchemaInfo
		if err := rows.Scan(&s.Name); err != nil {
			return nil, fmt.Errorf("scan schema: %w", err)
		}
		schemas = append(schemas, s)
	}
	return schemas, rows.Err()
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

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []datasource.TableInfo
	for rows.Next() {
		var t datasource.TableInfo
		if err := rows.Scan(&t.SchemaName, &t.TableName, &t.TableType, &t.RowEstimate, &t.Comment); err != nil {
			return nil, fmt.Errorf("scan table: %w", err)
		}
		tables = append(tables, t)
	}
	return tables, rows.Err()
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

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []datasource.ColumnInfo
	for rows.Next() {
		var c datasource.ColumnInfo
		if err := rows.Scan(
			&c.SchemaName, &c.TableName, &c.ColumnName, &c.DataType,
			&c.Nullable, &c.OrdinalPosition, &c.CharMaxLength,
			&c.NumericPrecision, &c.NumericScale, &c.ColumnDefault, &c.Comment,
		); err != nil {
			return nil, fmt.Errorf("scan column: %w", err)
		}
		columns = append(columns, c)
	}

	// Now get primary key info
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

	pkRows, err := db.QueryContext(ctx, pkQuery)
	if err != nil {
		return columns, nil // Non-fatal, skip PK info
	}
	defer pkRows.Close()

	pkSet := make(map[string]bool)
	for pkRows.Next() {
		var schema, table, column string
		if err := pkRows.Scan(&schema, &table, &column); err != nil {
			continue
		}
		key := fmt.Sprintf("%s.%s.%s", schema, table, column)
		pkSet[key] = true
	}

	// Mark primary keys
	for i := range columns {
		key := fmt.Sprintf("%s.%s.%s", columns[i].SchemaName, columns[i].TableName, columns[i].ColumnName)
		if pkSet[key] {
			columns[i].IsPrimaryKey = true
		}
	}

	return columns, rows.Err()
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

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var relations []datasource.RelationInfo
	for rows.Next() {
		var r datasource.RelationInfo
		r.RelationshipType = "many_to_one"
		if err := rows.Scan(
			&r.ConstraintName, &r.FromSchema, &r.FromTable, &r.FromColumn,
			&r.ToSchema, &r.ToTable, &r.ToColumn,
		); err != nil {
			return nil, fmt.Errorf("scan relation: %w", err)
		}
		relations = append(relations, r)
	}
	return relations, rows.Err()
}
