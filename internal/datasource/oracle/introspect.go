package oracle

import (
	"context"
	"database/sql"

	"github.com/biqly/biqly/internal/datasource"
)

const userFilter = `SELECT username FROM all_users WHERE oracle_maintained = 'N'`

const schemasQuery = `
SELECT username FROM all_users
WHERE oracle_maintained = 'N'
ORDER BY username`

func introspectSchemas(ctx context.Context, db *sql.DB) ([]datasource.SchemaInfo, error) {
	return datasource.QueryAll(ctx, db, schemasQuery, nil, datasource.ScanSchemaName)
}

const tablesQuery = `
SELECT t.owner, t.table_name, 'BASE TABLE', t.num_rows, COALESCE(c.comments, '')
FROM all_tables t
LEFT JOIN all_tab_comments c ON c.owner = t.owner AND c.table_name = t.table_name
WHERE t.owner IN (` + userFilter + `)
UNION ALL
SELECT v.owner, v.view_name, 'VIEW', NULL, ''
FROM all_views v
WHERE v.owner IN (` + userFilter + `)
ORDER BY 1, 2`

func introspectTables(ctx context.Context, db *sql.DB) ([]datasource.TableInfo, error) {
	return datasource.QueryAll(ctx, db, tablesQuery, nil, func(rows *sql.Rows) (datasource.TableInfo, error) {
		var t datasource.TableInfo
		err := rows.Scan(&t.SchemaName, &t.TableName, &t.TableType, &t.RowEstimate, &t.Comment)
		return t, err
	})
}

const columnsQuery = `
SELECT c.owner, c.table_name, c.column_name, c.data_type,
       CASE c.nullable WHEN 'Y' THEN 1 ELSE 0 END,
       c.column_id, c.char_length, c.data_precision, c.data_scale, ''
FROM all_tab_columns c
WHERE c.owner IN (` + userFilter + `)
ORDER BY c.owner, c.table_name, c.column_id`

func introspectColumns(ctx context.Context, db *sql.DB) ([]datasource.ColumnInfo, error) {
	cols, err := datasource.QueryAll(ctx, db, columnsQuery, nil, datasource.ScanStandardColumnInfo)
	if err != nil {
		return nil, err
	}
	pks, err := primaryKeyColumns(ctx, db)
	if err != nil {
		return nil, err
	}
	for i := range cols {
		if pks[pkKey{cols[i].SchemaName, cols[i].TableName, cols[i].ColumnName}] {
			cols[i].IsPrimaryKey = true
		}
	}
	return cols, nil
}

type pkKey struct{ schema, table, column string }

const pkQuery = `
SELECT cc.owner, cc.table_name, cc.column_name
FROM all_constraints ac
JOIN all_cons_columns cc
  ON cc.owner = ac.owner AND cc.constraint_name = ac.constraint_name
WHERE ac.constraint_type = 'P'
  AND ac.owner IN (` + userFilter + `)`

func primaryKeyColumns(ctx context.Context, db *sql.DB) (map[pkKey]bool, error) {
	rows, err := datasource.QueryAll(ctx, db, pkQuery, nil, func(r *sql.Rows) (pkKey, error) {
		var k pkKey
		err := r.Scan(&k.schema, &k.table, &k.column)
		return k, err
	})
	if err != nil {
		return nil, err
	}
	set := make(map[pkKey]bool, len(rows))
	for _, k := range rows {
		set[k] = true
	}
	return set, nil
}

const relationsQuery = `
SELECT ac.constraint_name,
       cc.owner, cc.table_name, cc.column_name,
       rc.owner, rc.table_name, rc.column_name
FROM all_constraints ac
JOIN all_cons_columns cc
  ON cc.owner = ac.owner AND cc.constraint_name = ac.constraint_name
JOIN all_cons_columns rc
  ON rc.owner = ac.r_owner AND rc.constraint_name = ac.r_constraint_name
 AND rc.position = cc.position
WHERE ac.constraint_type = 'R'
  AND ac.owner IN (` + userFilter + `)
ORDER BY ac.constraint_name, cc.position`

func introspectRelations(ctx context.Context, db *sql.DB) ([]datasource.RelationInfo, error) {
	return datasource.QueryAll(ctx, db, relationsQuery, nil, datasource.ScanForeignKeyRelation)
}
