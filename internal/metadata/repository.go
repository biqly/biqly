package metadata

import (
	"context"
	"database/sql"
	"fmt"
)

// Repository handles metadata database operations.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new metadata repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Datasource operations

// CreateDatasource inserts a new datasource record.
func (r *Repository) CreateDatasource(ctx context.Context, ds *Datasource) error {
	query := `
		INSERT INTO datasources (id, name, type, dsn_encrypted, config, is_active)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := r.db.ExecContext(ctx, query, ds.ID, ds.Name, ds.Type, ds.DSNEncrypted, ds.Config, ds.IsActive)
	return err
}

// GetDatasource retrieves a datasource by ID.
func (r *Repository) GetDatasource(ctx context.Context, id string) (*Datasource, error) {
	query := `SELECT id, name, type, dsn_encrypted, config, is_active, last_sync_at, created_at, updated_at FROM datasources WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, id)
	return r.scanDatasource(row)
}

// GetDatasourceByName retrieves a datasource by name.
func (r *Repository) GetDatasourceByName(ctx context.Context, name string) (*Datasource, error) {
	query := `SELECT id, name, type, dsn_encrypted, config, is_active, last_sync_at, created_at, updated_at FROM datasources WHERE name = $1`
	row := r.db.QueryRowContext(ctx, query, name)
	return r.scanDatasource(row)
}

// ListDatasources returns all configured datasources.
func (r *Repository) ListDatasources(ctx context.Context) ([]Datasource, error) {
	query := `SELECT id, name, type, dsn_encrypted, config, is_active, last_sync_at, created_at, updated_at FROM datasources ORDER BY created_at DESC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	datasources := make([]Datasource, 0)
	for rows.Next() {
		ds, err := r.scanDatasource(rows)
		if err != nil {
			return nil, err
		}
		datasources = append(datasources, *ds)
	}
	return datasources, rows.Err()
}

// DeleteDatasource removes a datasource by ID.
func (r *Repository) DeleteDatasource(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM datasources WHERE id = $1`, id)
	return err
}

// UpdateDatasourceSync updates the last_sync_at timestamp.
func (r *Repository) UpdateDatasourceSync(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE datasources SET last_sync_at = now(), updated_at = now() WHERE id = $1`, id)
	return err
}

type scanner interface {
	Scan(dest ...any) error
}

func (r *Repository) scanDatasource(s scanner) (*Datasource, error) {
	ds := &Datasource{}
	err := s.Scan(&ds.ID, &ds.Name, &ds.Type, &ds.DSNEncrypted, &ds.Config, &ds.IsActive, &ds.LastSyncAt, &ds.CreatedAt, &ds.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("scan datasource: %w", err)
	}
	return ds, nil
}

// Schema operations

// UpsertSchemas inserts or updates schema metadata.
func (r *Repository) UpsertSchemas(ctx context.Context, datasourceID string, schemas []Schema) error {
	for _, s := range schemas {
		if _, err := r.UpsertSchema(ctx, datasourceID, s); err != nil {
			return err
		}
	}
	return nil
}

// UpsertSchema inserts or updates schema metadata and returns the persisted ID.
func (r *Repository) UpsertSchema(ctx context.Context, datasourceID string, s Schema) (string, error) {
	query := `
		INSERT INTO schemas (id, datasource_id, schema_name)
		VALUES ($1, $2, $3)
		ON CONFLICT (datasource_id, schema_name) DO UPDATE
		SET schema_name = EXCLUDED.schema_name
		RETURNING id
	`
	var id string
	if err := r.db.QueryRowContext(ctx, query, s.ID, datasourceID, s.SchemaName).Scan(&id); err != nil {
		return "", fmt.Errorf("upsert schema %s: %w", s.SchemaName, err)
	}
	return id, nil
}

// Table operations

// UpsertTables inserts or updates table metadata. The description column is preserved
// when the incoming description is nil/empty so that user-provided descriptions are
// not overwritten by a re-sync against a source DB that has no native comment.
func (r *Repository) UpsertTables(ctx context.Context, datasourceID string, tables []Table) error {
	for _, t := range tables {
		if _, err := r.UpsertTable(ctx, datasourceID, t); err != nil {
			return err
		}
	}
	return nil
}

// UpsertTable inserts or updates table metadata and returns the persisted ID.
func (r *Repository) UpsertTable(ctx context.Context, datasourceID string, t Table) (string, error) {
	query := `
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
	if err := r.db.QueryRowContext(ctx, query, t.ID, datasourceID, t.SchemaID, t.SchemaName, t.TableName, t.TableType, t.RowEstimate, t.Description).Scan(&id); err != nil {
		return "", fmt.Errorf("upsert table %s.%s: %w", t.SchemaName, t.TableName, err)
	}
	return id, nil
}

// ListTables returns all stored tables for a datasource, optionally filtered by schema.
func (r *Repository) ListTables(ctx context.Context, datasourceID, schemaName string) ([]Table, error) {
	query := `
		SELECT id, datasource_id, schema_id, schema_name, table_name, table_type, row_estimate, description, created_at, updated_at
		FROM tables
		WHERE datasource_id = $1
	`
	args := []any{datasourceID}
	if schemaName != "" {
		query += " AND schema_name = $2"
		args = append(args, schemaName)
	}
	query += " ORDER BY schema_name, table_name"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var tables []Table
	for rows.Next() {
		var t Table
		if err := rows.Scan(&t.ID, &t.DatasourceID, &t.SchemaID, &t.SchemaName, &t.TableName, &t.TableType, &t.RowEstimate, &t.Description, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tables = append(tables, t)
	}
	return tables, rows.Err()
}

// GetTable returns a single table by ID.
func (r *Repository) GetTable(ctx context.Context, id string) (*Table, error) {
	query := `
		SELECT id, datasource_id, schema_id, schema_name, table_name, table_type, row_estimate, description, created_at, updated_at
		FROM tables WHERE id = $1
	`
	var t Table
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&t.ID, &t.DatasourceID, &t.SchemaID, &t.SchemaName, &t.TableName, &t.TableType, &t.RowEstimate, &t.Description, &t.CreatedAt, &t.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// UpdateTableDescription replaces the description text on a table row.
func (r *Repository) UpdateTableDescription(ctx context.Context, id string, description *string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE tables SET description = $2, updated_at = now() WHERE id = $1`, id, description)
	return err
}

// Column operations

// UpsertColumns inserts or updates column metadata. Description is preserved when the
// incoming description is nil/empty so manually edited / AI-generated descriptions
// survive a metadata re-sync.
func (r *Repository) UpsertColumns(ctx context.Context, datasourceID string, columns []Column) error {
	query := `
		INSERT INTO columns (id, datasource_id, table_id, schema_name, table_name, column_name, data_type, nullable, ordinal_position, character_maximum_length, numeric_precision, numeric_scale, column_default, description, is_primary_key, is_foreign_key, referenced_schema, referenced_table, referenced_column)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19)
		ON CONFLICT (datasource_id, schema_name, table_name, column_name) DO UPDATE
		SET data_type = EXCLUDED.data_type,
			nullable = EXCLUDED.nullable,
			is_primary_key = EXCLUDED.is_primary_key,
			is_foreign_key = EXCLUDED.is_foreign_key,
			description = COALESCE(NULLIF(EXCLUDED.description, ''), columns.description)
	`
	for _, c := range columns {
		if _, err := r.db.ExecContext(ctx, query, c.ID, datasourceID, c.TableID, c.SchemaName, c.TableName, c.ColumnName, c.DataType, c.Nullable, c.OrdinalPosition, c.CharMaxLength, c.NumericPrecision, c.NumericScale, c.ColumnDefault, c.Description, c.IsPrimaryKey, c.IsForeignKey, c.ReferencedSchema, c.ReferencedTable, c.ReferencedColumn); err != nil {
			return fmt.Errorf("upsert column %s.%s.%s: %w", c.SchemaName, c.TableName, c.ColumnName, err)
		}
	}
	return nil
}

// ListColumns returns columns for a datasource, optionally scoped to a single table.
func (r *Repository) ListColumns(ctx context.Context, datasourceID, schemaName, tableName string) ([]Column, error) {
	query := `
		SELECT id, datasource_id, table_id, schema_name, table_name, column_name, data_type, nullable, ordinal_position, character_maximum_length, numeric_precision, numeric_scale, column_default, description, is_primary_key, is_foreign_key, referenced_schema, referenced_table, referenced_column, created_at
		FROM columns
		WHERE datasource_id = $1
	`
	args := []any{datasourceID}
	if schemaName != "" {
		query += fmt.Sprintf(" AND schema_name = $%d", len(args)+1)
		args = append(args, schemaName)
	}
	if tableName != "" {
		//nolint:gosec // parameterized query with $N placeholders
		query += fmt.Sprintf(" AND table_name = $%d", len(args)+1)
		args = append(args, tableName)
	}
	query += " ORDER BY schema_name, table_name, ordinal_position"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var columns []Column
	for rows.Next() {
		var c Column
		if err := rows.Scan(&c.ID, &c.DatasourceID, &c.TableID, &c.SchemaName, &c.TableName, &c.ColumnName, &c.DataType, &c.Nullable, &c.OrdinalPosition, &c.CharMaxLength, &c.NumericPrecision, &c.NumericScale, &c.ColumnDefault, &c.Description, &c.IsPrimaryKey, &c.IsForeignKey, &c.ReferencedSchema, &c.ReferencedTable, &c.ReferencedColumn, &c.CreatedAt); err != nil {
			return nil, err
		}
		columns = append(columns, c)
	}
	return columns, rows.Err()
}

// GetColumn returns a single column by ID.
func (r *Repository) GetColumn(ctx context.Context, id string) (*Column, error) {
	query := `
		SELECT id, datasource_id, table_id, schema_name, table_name, column_name, data_type, nullable, ordinal_position, character_maximum_length, numeric_precision, numeric_scale, column_default, description, is_primary_key, is_foreign_key, referenced_schema, referenced_table, referenced_column, created_at
		FROM columns WHERE id = $1
	`
	var c Column
	err := r.db.QueryRowContext(ctx, query, id).Scan(
		&c.ID, &c.DatasourceID, &c.TableID, &c.SchemaName, &c.TableName, &c.ColumnName, &c.DataType, &c.Nullable, &c.OrdinalPosition, &c.CharMaxLength, &c.NumericPrecision, &c.NumericScale, &c.ColumnDefault, &c.Description, &c.IsPrimaryKey, &c.IsForeignKey, &c.ReferencedSchema, &c.ReferencedTable, &c.ReferencedColumn, &c.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// UpdateColumnDescription replaces the description text on a column row.
func (r *Repository) UpdateColumnDescription(ctx context.Context, id string, description *string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE columns SET description = $2 WHERE id = $1`, id, description)
	return err
}

// Relation operations

// UpsertRelations inserts or updates relation metadata.
func (r *Repository) UpsertRelations(ctx context.Context, datasourceID string, relations []Relation) error {
	query := `
		INSERT INTO relations (id, datasource_id, constraint_name, from_schema, from_table, from_column, to_schema, to_table, to_column, relationship_type)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		ON CONFLICT (datasource_id, constraint_name) DO UPDATE
		SET relationship_type = EXCLUDED.relationship_type
	`
	for _, rel := range relations {
		if _, err := r.db.ExecContext(ctx, query, rel.ID, datasourceID, rel.ConstraintName, rel.FromSchema, rel.FromTable, rel.FromColumn, rel.ToSchema, rel.ToTable, rel.ToColumn, rel.RelationshipType); err != nil {
			return fmt.Errorf("upsert relation %s: %w", rel.ConstraintName, err)
		}
	}
	return nil
}

// Search operations

// SearchColumns searches columns by name or description.
func (r *Repository) SearchColumns(ctx context.Context, datasourceID, searchTerm string) ([]Column, error) {
	query := `
		SELECT id, datasource_id, table_id, schema_name, table_name, column_name, data_type, nullable, ordinal_position, character_maximum_length, numeric_precision, numeric_scale, column_default, description, is_primary_key, is_foreign_key, created_at
		FROM columns
		WHERE datasource_id = $1 AND (column_name ILIKE $2 OR description ILIKE $2)
		ORDER BY table_name, column_name
	`
	rows, err := r.db.QueryContext(ctx, query, datasourceID, "%"+searchTerm+"%")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var columns []Column
	for rows.Next() {
		var c Column
		if err := rows.Scan(&c.ID, &c.DatasourceID, &c.TableID, &c.SchemaName, &c.TableName, &c.ColumnName, &c.DataType, &c.Nullable, &c.OrdinalPosition, &c.CharMaxLength, &c.NumericPrecision, &c.NumericScale, &c.ColumnDefault, &c.Description, &c.IsPrimaryKey, &c.IsForeignKey, &c.CreatedAt); err != nil {
			return nil, err
		}
		columns = append(columns, c)
	}
	return columns, rows.Err()
}

// SearchTables searches tables by name or description.
func (r *Repository) SearchTables(ctx context.Context, datasourceID, searchTerm string) ([]Table, error) {
	query := `
		SELECT id, datasource_id, schema_id, schema_name, table_name, table_type, row_estimate, description, created_at, updated_at
		FROM tables
		WHERE datasource_id = $1 AND (table_name ILIKE $2 OR description ILIKE $2)
		ORDER BY schema_name, table_name
	`
	rows, err := r.db.QueryContext(ctx, query, datasourceID, "%"+searchTerm+"%")
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var tables []Table
	for rows.Next() {
		var t Table
		if err := rows.Scan(&t.ID, &t.DatasourceID, &t.SchemaID, &t.SchemaName, &t.TableName, &t.TableType, &t.RowEstimate, &t.Description, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		tables = append(tables, t)
	}
	return tables, rows.Err()
}
