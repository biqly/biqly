package snowflake

import (
	"context"
	"database/sql"

	"github.com/biqly/biqly/internal/datasource"
)

const schemasQuery = `
SELECT schema_name
FROM information_schema.schemata
WHERE schema_name NOT IN ('INFORMATION_SCHEMA')
ORDER BY schema_name`

func introspectSchemas(ctx context.Context, db *sql.DB) ([]datasource.SchemaInfo, error) {
	return datasource.QueryAll(ctx, db, schemasQuery, nil, datasource.ScanSchemaName)
}

const tablesQuery = `
SELECT table_schema, table_name,
       CASE table_type WHEN 'VIEW' THEN 'VIEW' ELSE 'BASE TABLE' END,
       row_count, COALESCE(comment, '')
FROM information_schema.tables
WHERE table_schema NOT IN ('INFORMATION_SCHEMA')
ORDER BY table_schema, table_name`

func introspectTables(ctx context.Context, db *sql.DB) ([]datasource.TableInfo, error) {
	return datasource.QueryAll(ctx, db, tablesQuery, nil, func(rows *sql.Rows) (datasource.TableInfo, error) {
		var t datasource.TableInfo
		err := rows.Scan(&t.SchemaName, &t.TableName, &t.TableType, &t.RowEstimate, &t.Comment)
		return t, err
	})
}

const columnsQuery = `
SELECT table_schema, table_name, column_name, data_type,
       CASE is_nullable WHEN 'YES' THEN 1 ELSE 0 END,
       ordinal_position, character_maximum_length,
       numeric_precision, numeric_scale, COALESCE(column_default, '')
FROM information_schema.columns
WHERE table_schema NOT IN ('INFORMATION_SCHEMA')
ORDER BY table_schema, table_name, ordinal_position`

func introspectColumns(ctx context.Context, db *sql.DB) ([]datasource.ColumnInfo, error) {
	return datasource.QueryAll(ctx, db, columnsQuery, nil, datasource.ScanStandardColumnInfo)
}

func introspectRelations(_ context.Context, _ *sql.DB) ([]datasource.RelationInfo, error) {
	// Snowflake INFORMATION_SCHEMA has no KEY_COLUMN_USAGE; FK metadata is only
	// available via SHOW IMPORTED KEYS, whose output is version-brittle.
	return nil, nil
}
