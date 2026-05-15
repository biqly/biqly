package metadata

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

const (
	columnUpsertRowCols    = 19
	columnUpsertChunkRows  = 250
	columnUpsertValueQuery = `
		INSERT INTO columns (id, datasource_id, table_id, schema_name, table_name, column_name, data_type, nullable, ordinal_position, character_maximum_length, numeric_precision, numeric_scale, column_default, description, is_primary_key, is_foreign_key, referenced_schema, referenced_table, referenced_column)
		VALUES %s
		ON CONFLICT (datasource_id, schema_name, table_name, column_name) DO UPDATE
		SET data_type = EXCLUDED.data_type,
			nullable = EXCLUDED.nullable,
			is_primary_key = EXCLUDED.is_primary_key,
			is_foreign_key = EXCLUDED.is_foreign_key,
			referenced_schema = EXCLUDED.referenced_schema,
			referenced_table = EXCLUDED.referenced_table,
			referenced_column = EXCLUDED.referenced_column,
			description = COALESCE(NULLIF(EXCLUDED.description, ''), columns.description)`
)

type execContexter interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func upsertColumnsBatch(ctx context.Context, q execContexter, datasourceID string, columns []Column) error {
	for start := 0; start < len(columns); start += columnUpsertChunkRows {
		end := start + columnUpsertChunkRows
		if end > len(columns) {
			end = len(columns)
		}
		if err := upsertColumnsChunk(ctx, q, datasourceID, columns[start:end]); err != nil {
			return err
		}
	}
	return nil
}

func upsertColumnsChunk(ctx context.Context, q execContexter, datasourceID string, columns []Column) error {
	if len(columns) == 0 {
		return nil
	}

	var values strings.Builder
	args := make([]any, 0, len(columns)*columnUpsertRowCols)
	placeholder := 1
	for i, c := range columns {
		if i > 0 {
			values.WriteByte(',')
		}
		values.WriteByte('(')
		for col := 0; col < columnUpsertRowCols; col++ {
			if col > 0 {
				values.WriteByte(',')
			}
			fmt.Fprintf(&values, "$%d", placeholder)
			placeholder++
		}
		values.WriteByte(')')
		args = append(args,
			c.ID, datasourceID, c.TableID, c.SchemaName, c.TableName, c.ColumnName,
			c.DataType, c.Nullable, c.OrdinalPosition, c.CharMaxLength, c.NumericPrecision,
			c.NumericScale, c.ColumnDefault, c.Description, c.IsPrimaryKey, c.IsForeignKey,
			c.ReferencedSchema, c.ReferencedTable, c.ReferencedColumn,
		)
	}

	query := fmt.Sprintf(columnUpsertValueQuery, values.String())
	if _, err := q.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("upsert columns batch: %w", err)
	}
	return nil
}
