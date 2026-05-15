package metadata

import (
	"context"
	"database/sql"
	"fmt"
)

type queryRower interface {
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func upsertSchemaRow(ctx context.Context, q queryRower, datasourceID string, s Schema) (string, error) {
	const query = `
		INSERT INTO schemas (id, datasource_id, schema_name)
		VALUES ($1, $2, $3)
		ON CONFLICT (datasource_id, schema_name) DO UPDATE
		SET schema_name = EXCLUDED.schema_name
		RETURNING id
	`
	var id string
	if err := q.QueryRowContext(ctx, query, s.ID, datasourceID, s.SchemaName).Scan(&id); err != nil {
		return "", fmt.Errorf("upsert schema %s: %w", s.SchemaName, err)
	}
	return id, nil
}

func upsertTableRow(ctx context.Context, q queryRower, datasourceID string, t Table) (string, error) {
	const query = `
		INSERT INTO tables (id, datasource_id, schema_id, schema_name, table_name, table_type, row_estimate, description)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		ON CONFLICT (datasource_id, schema_name, table_name) DO UPDATE
		SET table_type = EXCLUDED.table_type,
			row_estimate = EXCLUDED.row_estimate,
			description = COALESCE(NULLIF(EXCLUDED.description, ''), tables.description),
			updated_at = now()
		RETURNING id
	`
	var id string
	if err := q.QueryRowContext(ctx, query, t.ID, datasourceID, t.SchemaID, t.SchemaName, t.TableName, t.TableType, t.RowEstimate, t.Description).Scan(&id); err != nil {
		return "", fmt.Errorf("upsert table %s.%s: %w", t.SchemaName, t.TableName, err)
	}
	return id, nil
}
