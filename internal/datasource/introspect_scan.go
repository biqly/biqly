package datasource

import "database/sql"

func ScanSchemaName(rows *sql.Rows) (SchemaInfo, error) {
	var s SchemaInfo
	err := rows.Scan(&s.Name)
	return s, err
}

func ScanTableInfo(rows *sql.Rows) (TableInfo, error) {
	var t TableInfo
	err := rows.Scan(&t.SchemaName, &t.TableName, &t.TableType, &t.RowEstimate)
	return t, err
}

// ScanStandardColumnInfo reads information_schema-style column metadata:
// schema, table, name, data_type, nullable (0/1), ordinal, char max, numeric precision, scale, default.
func ScanStandardColumnInfo(rows *sql.Rows) (ColumnInfo, error) {
	var c ColumnInfo
	var nullable int
	var columnDefault sql.NullString
	err := rows.Scan(
		&c.SchemaName, &c.TableName, &c.ColumnName, &c.DataType,
		&nullable, &c.OrdinalPosition, &c.CharMaxLength,
		&c.NumericPrecision, &c.NumericScale, &columnDefault,
	)
	if err != nil {
		return c, err
	}
	c.Nullable = nullable == 1
	if columnDefault.Valid {
		c.ColumnDefault = columnDefault.String
	}
	return c, nil
}

func ScanForeignKeyRelation(rows *sql.Rows) (RelationInfo, error) {
	var rel RelationInfo
	rel.RelationshipType = DefaultRelationshipType
	err := rows.Scan(
		&rel.ConstraintName, &rel.FromSchema, &rel.FromTable, &rel.FromColumn,
		&rel.ToSchema, &rel.ToTable, &rel.ToColumn,
	)
	return rel, err
}
