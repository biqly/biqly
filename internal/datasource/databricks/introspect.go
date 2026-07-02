package databricks

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/biqly/biqly/internal/datasource"
)

const schemasQuery = `
SELECT schema_name
FROM information_schema.schemata
WHERE schema_name NOT IN ('information_schema')
ORDER BY schema_name`

func introspectSchemas(ctx context.Context, db *sql.DB) ([]datasource.SchemaInfo, error) {
	return datasource.QueryAll(ctx, db, schemasQuery, nil, datasource.ScanSchemaName)
}

const tablesQuery = `
SELECT table_schema, table_name,
       CASE table_type WHEN 'VIEW' THEN 'VIEW' ELSE 'BASE TABLE' END,
       CAST(NULL AS BIGINT), COALESCE(comment, '')
FROM information_schema.tables
WHERE table_schema NOT IN ('information_schema')
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
WHERE table_schema NOT IN ('information_schema')
ORDER BY table_schema, table_name, ordinal_position`

func introspectColumns(ctx context.Context, db *sql.DB) ([]datasource.ColumnInfo, error) {
	return datasource.QueryAll(ctx, db, columnsQuery, nil, datasource.ScanStandardColumnInfo)
}

const relationsQuery = `
SELECT rc.constraint_name,
       kcu.table_schema, kcu.table_name, kcu.column_name,
       rk.table_schema, rk.table_name, rk.column_name
FROM information_schema.referential_constraints rc
JOIN information_schema.key_column_usage kcu
  ON kcu.constraint_catalog = rc.constraint_catalog
 AND kcu.constraint_schema = rc.constraint_schema
 AND kcu.constraint_name = rc.constraint_name
JOIN information_schema.key_column_usage rk
  ON rk.constraint_catalog = rc.unique_constraint_catalog
 AND rk.constraint_schema = rc.unique_constraint_schema
 AND rk.constraint_name = rc.unique_constraint_name
 AND rk.ordinal_position = kcu.position_in_unique_constraint
ORDER BY rc.constraint_name, kcu.ordinal_position`

func introspectRelations(ctx context.Context, db *sql.DB) ([]datasource.RelationInfo, error) {
	rels, err := datasource.QueryAll(ctx, db, relationsQuery, nil, datasource.ScanForeignKeyRelation)
	if err != nil {
		// hive_metastore catalogs have no information_schema constraint views;
		// degrade to no relations instead of failing the whole sync.
		slog.WarnContext(ctx, "databricks relation introspection unavailable", "error", err)
		return nil, nil
	}
	return rels, nil
}
