package sqlite

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/biqly/biqly/internal/datasource"
)

func introspectSchemas(_ context.Context, _ *sql.DB) ([]datasource.SchemaInfo, error) {
	return []datasource.SchemaInfo{{Name: "main"}}, nil
}

const tablesQuery = `
SELECT name, type
FROM sqlite_master
WHERE type IN ('table', 'view') AND name NOT LIKE 'sqlite_%'
ORDER BY name`

func introspectTables(ctx context.Context, db *sql.DB) ([]datasource.TableInfo, error) {
	return datasource.QueryAll(ctx, db, tablesQuery, nil, func(rows *sql.Rows) (datasource.TableInfo, error) {
		var t datasource.TableInfo
		var typ string
		if err := rows.Scan(&t.TableName, &typ); err != nil {
			return t, err
		}
		t.SchemaName = "main"
		t.TableType = "BASE TABLE"
		if typ == "view" {
			t.TableType = "VIEW"
		}
		return t, nil
	})
}

const columnsQuery = `
SELECT ti.name, ti.type, ti."notnull", ti.dflt_value, ti.cid, ti.pk
FROM pragma_table_info(?) AS ti
ORDER BY ti.cid`

func introspectColumns(ctx context.Context, db *sql.DB) ([]datasource.ColumnInfo, error) {
	tables, err := introspectTables(ctx, db)
	if err != nil {
		return nil, err
	}
	var out []datasource.ColumnInfo
	for _, t := range tables {
		cols, err := datasource.QueryAll(ctx, db, columnsQuery, []any{t.TableName}, func(rows *sql.Rows) (datasource.ColumnInfo, error) {
			var c datasource.ColumnInfo
			var notNull, pk, cid int
			var dflt sql.NullString
			if err := rows.Scan(&c.ColumnName, &c.DataType, &notNull, &dflt, &cid, &pk); err != nil {
				return c, err
			}
			c.SchemaName = "main"
			c.TableName = t.TableName
			c.Nullable = notNull == 0
			c.OrdinalPosition = cid + 1
			if dflt.Valid {
				c.ColumnDefault = dflt.String
			}
			c.IsPrimaryKey = pk > 0
			return c, nil
		})
		if err != nil {
			return nil, err
		}
		out = append(out, cols...)
	}
	return out, nil
}

const fkQuery = `
SELECT fk.id, fk."table", fk."from", fk."to"
FROM pragma_foreign_key_list(?) AS fk
ORDER BY fk.id, fk.seq`

func introspectRelations(ctx context.Context, db *sql.DB) ([]datasource.RelationInfo, error) {
	tables, err := introspectTables(ctx, db)
	if err != nil {
		return nil, err
	}
	var out []datasource.RelationInfo
	for _, t := range tables {
		rels, err := datasource.QueryAll(ctx, db, fkQuery, []any{t.TableName}, func(rows *sql.Rows) (datasource.RelationInfo, error) {
			var r datasource.RelationInfo
			var id int
			if err := rows.Scan(&id, &r.ToTable, &r.FromColumn, &r.ToColumn); err != nil {
				return r, err
			}
			r.ConstraintName = fmt.Sprintf("fk_%s_%d", t.TableName, id)
			r.FromSchema = "main"
			r.FromTable = t.TableName
			r.ToSchema = "main"
			r.RelationshipType = datasource.DefaultRelationshipType
			return r, nil
		})
		if err != nil {
			return nil, err
		}
		out = append(out, rels...)
	}
	return out, nil
}
