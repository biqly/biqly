package metadata

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
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
		end := min(start+columnUpsertChunkRows, len(columns))
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

	values := make([]string, 0, len(columns))
	args := make([]any, 0, len(columns)*columnUpsertRowCols)
	placeholder := 1
	for _, c := range columns {
		row := make([]string, 0, columnUpsertRowCols)
		for range columnUpsertRowCols {
			row = append(row, "$"+strconv.Itoa(placeholder))
			placeholder++
		}
		values = append(values, "("+strings.Join(row, ",")+")")
		args = append(args,
			c.ID, datasourceID, c.TableID, c.SchemaName, c.TableName, c.ColumnName,
			c.DataType, c.Nullable, c.OrdinalPosition, c.CharMaxLength, c.NumericPrecision,
			c.NumericScale, c.ColumnDefault, c.Description, c.IsPrimaryKey, c.IsForeignKey,
			c.ReferencedSchema, c.ReferencedTable, c.ReferencedColumn,
		)
	}

	query := strings.Replace(columnUpsertValueQuery, "%s", strings.Join(values, ","), 1)
	if _, err := q.ExecContext(ctx, query, args...); err != nil {
		return fmt.Errorf("upsert columns batch: %w", err)
	}
	return nil
}
