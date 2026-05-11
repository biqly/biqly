package metadata

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"

	"github.com/biqly/biqly/internal/query"
	"github.com/lib/pq"
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

// TableEmbedding pairs a tableKey ("schema.table") with its stored embedding.
// Tables that have never been embedded are excluded from ListTableEmbeddings.
type TableEmbedding struct {
	SchemaName string
	TableName  string
	Model      string
	Embedding  []float32
}

// UpsertTableEmbedding stores (or replaces) the embedding vector for a single
// table. The vector is JSON-encoded so deployments without pgvector keep
// working; cosine similarity is computed in-process by the AI router.
func (r *Repository) UpsertTableEmbedding(ctx context.Context, tableID, modelName string, embedding []float32) error {
	encoded, err := encodeEmbedding(embedding)
	if err != nil {
		return fmt.Errorf("encode embedding: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
		UPDATE tables
		SET embedding = $2::jsonb,
		    embedding_model = $3,
		    embedding_updated_at = now()
		WHERE id = $1
	`, tableID, encoded, modelName)
	return err
}

// ListTableEmbeddings returns every stored embedding for a datasource. Used by
// the AI router on each NL→query request; we keep this single round-trip
// rather than fanning out per-table lookups.
func (r *Repository) ListTableEmbeddings(ctx context.Context, datasourceID string) ([]TableEmbedding, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT schema_name, table_name, embedding_model, embedding
		FROM tables
		WHERE datasource_id = $1 AND embedding IS NOT NULL
	`, datasourceID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []TableEmbedding
	for rows.Next() {
		var (
			te      TableEmbedding
			modelN  *string
			rawJSON []byte
		)
		if err := rows.Scan(&te.SchemaName, &te.TableName, &modelN, &rawJSON); err != nil {
			return nil, err
		}
		if modelN != nil {
			te.Model = *modelN
		}
		emb, err := decodeEmbedding(rawJSON)
		if err != nil {
			// Skip rows with a bad payload rather than failing the whole list;
			// the router still gets every well-formed entry.
			continue
		}
		te.Embedding = emb
		out = append(out, te)
	}
	return out, rows.Err()
}

func encodeEmbedding(vec []float32) (string, error) {
	if vec == nil {
		return "null", nil
	}
	b, err := json.Marshal(vec)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func decodeEmbedding(raw []byte) ([]float32, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var vec []float32
	if err := json.Unmarshal(raw, &vec); err != nil {
		return nil, err
	}
	return vec, nil
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
			referenced_schema = EXCLUDED.referenced_schema,
			referenced_table = EXCLUDED.referenced_table,
			referenced_column = EXCLUDED.referenced_column,
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

func (r *Repository) ListPermissionPolicies(ctx context.Context, datasourceID string) ([]PermissionPolicyRecord, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT denied_fields, row_filters
		FROM permissions
		WHERE datasource_id::text = $1
	`, datasourceID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var policies []PermissionPolicyRecord
	for rows.Next() {
		var (
			policy     PermissionPolicyRecord
			rowFilters []byte
			denied     pq.StringArray
		)
		if err := rows.Scan(&denied, &rowFilters); err != nil {
			return nil, err
		}
		policy.DeniedFields = []string(denied)
		if len(rowFilters) > 0 {
			_ = json.Unmarshal(rowFilters, &policy.RowFilters)
		}
		policies = append(policies, policy)
	}
	return policies, rows.Err()
}

// UpdateColumnDescription replaces the description text on a column row.
func (r *Repository) UpdateColumnDescription(ctx context.Context, id string, description *string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE columns SET description = $2 WHERE id = $1`, id, description)
	return err
}

// UpsertColumnEmbedding stores (or replaces) the embedding vector for a single
// column. It mirrors table embedding storage and keeps deployments independent
// of pgvector.
func (r *Repository) UpsertColumnEmbedding(ctx context.Context, columnID, modelName string, embedding []float32) error {
	encoded, err := encodeEmbedding(embedding)
	if err != nil {
		return fmt.Errorf("encode embedding: %w", err)
	}
	_, err = r.db.ExecContext(ctx, `
		UPDATE columns
		SET embedding = $2::jsonb,
		    embedding_model = $3,
		    embedding_updated_at = now()
		WHERE id = $1
	`, columnID, encoded, modelName)
	return err
}

// ListColumnEmbeddings returns every stored column embedding for a datasource in
// one round-trip. Router code decides whether coverage is complete enough to use.
func (r *Repository) ListColumnEmbeddings(ctx context.Context, datasourceID string) ([]ColumnEmbedding, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT schema_name, table_name, column_name, embedding_model, embedding
		FROM columns
		WHERE datasource_id = $1 AND embedding IS NOT NULL
	`, datasourceID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var out []ColumnEmbedding
	for rows.Next() {
		var (
			ce      ColumnEmbedding
			modelN  *string
			rawJSON []byte
		)
		if err := rows.Scan(&ce.SchemaName, &ce.TableName, &ce.ColumnName, &modelN, &rawJSON); err != nil {
			return nil, err
		}
		if modelN != nil {
			ce.Model = *modelN
		}
		emb, err := decodeEmbedding(rawJSON)
		if err != nil {
			continue
		}
		ce.Embedding = emb
		out = append(out, ce)
	}
	return out, rows.Err()
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

// ListRelations returns foreign-key relationships for a datasource.
func (r *Repository) ListRelations(ctx context.Context, datasourceID string) ([]Relation, error) {
	query := `
		SELECT id, datasource_id, constraint_name, from_schema, from_table, from_column, to_schema, to_table, to_column, relationship_type, created_at
		FROM relations
		WHERE datasource_id = $1
		ORDER BY from_schema, from_table, from_column
	`
	rows, err := r.db.QueryContext(ctx, query, datasourceID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var relations []Relation
	for rows.Next() {
		var rel Relation
		if err := rows.Scan(
			&rel.ID,
			&rel.DatasourceID,
			&rel.ConstraintName,
			&rel.FromSchema,
			&rel.FromTable,
			&rel.FromColumn,
			&rel.ToSchema,
			&rel.ToTable,
			&rel.ToColumn,
			&rel.RelationshipType,
			&rel.CreatedAt,
		); err != nil {
			return nil, err
		}
		relations = append(relations, rel)
	}
	return relations, rows.Err()
}

// History operations

// CreateQueryHistory stores a structured query execution history entry.
func (r *Repository) CreateQueryHistory(ctx context.Context, entry *query.HistoryEntry) error {
	logicalQueryJSON, err := json.Marshal(entry.LogicalQuery)
	if err != nil {
		return fmt.Errorf("marshal logical query: %w", err)
	}

	insert := `
		INSERT INTO query_history (
			datasource_id, model_id, user_id, logical_query, compiled_sql,
			sql_args, status, row_count, duration_ms, error_message
		)
		VALUES ($1, $2, $3, $4::jsonb, $5, $6::jsonb, $7, $8, $9, $10)
		RETURNING id, created_at
	`
	if err := r.db.QueryRowContext(
		ctx,
		insert,
		entry.DatasourceID,
		entry.ModelID,
		entry.UserID,
		string(logicalQueryJSON),
		entry.CompiledSQL,
		entry.SQLArgs,
		entry.Status,
		entry.RowCount,
		entry.DurationMs,
		entry.ErrorMessage,
	).Scan(&entry.ID, &entry.CreatedAt); err != nil {
		return fmt.Errorf("insert query history: %w", err)
	}
	return nil
}

// ListQueryHistory returns recent query history entries.
func (r *Repository) ListQueryHistory(ctx context.Context) ([]query.HistoryEntry, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, datasource_id, model_id, user_id, logical_query, compiled_sql,
			sql_args, status, row_count, duration_ms, error_message, created_at
		FROM query_history
		ORDER BY created_at DESC
		LIMIT 100
	`)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var entries []query.HistoryEntry
	for rows.Next() {
		entry, err := scanQueryHistoryEntry(rows)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

// GetQueryHistory returns one query history entry by ID.
func (r *Repository) GetQueryHistory(ctx context.Context, id string) (*query.HistoryEntry, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, datasource_id, model_id, user_id, logical_query, compiled_sql,
			sql_args, status, row_count, duration_ms, error_message, created_at
		FROM query_history
		WHERE id = $1
	`, id)
	entry, err := scanQueryHistoryEntry(row)
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

// SuccessfulAIQuery is a stripped-down history row used to build dynamic few-shot
// prompt examples. Only fields needed for the LLM are exposed.
type SuccessfulAIQuery struct {
	Question     string
	LogicalQuery []byte
}

// ListSuccessfulAIQueries returns the most recent N AI history entries that
// produced a high-confidence query with no warnings, scoped to the given
// datasource and (optionally) semantic model. Used to inject dynamic few-shot
// examples into the prompt builder.
func (r *Repository) ListSuccessfulAIQueries(ctx context.Context, datasourceID string, modelName *string, limit int) ([]SuccessfulAIQuery, error) {
	if limit <= 0 {
		return nil, nil
	}
	const minConfidence = 0.7
	q := `
		SELECT question, logical_query
		FROM ai_query_history
		WHERE datasource_id = $1
		  AND ($2::text IS NULL OR model_id = $2)
		  AND confidence_score >= $3
		  AND (warnings IS NULL OR cardinality(warnings) = 0)
		  AND logical_query IS NOT NULL
		ORDER BY created_at DESC
		LIMIT $4
	`
	var modelArg sql.NullString
	if modelName != nil && *modelName != "" {
		modelArg = sql.NullString{String: *modelName, Valid: true}
	}
	rows, err := r.db.QueryContext(ctx, q, datasourceID, modelArg, minConfidence, limit)
	if err != nil {
		return nil, fmt.Errorf("query successful AI history: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []SuccessfulAIQuery
	for rows.Next() {
		var question string
		var lq []byte
		if err := rows.Scan(&question, &lq); err != nil {
			return nil, fmt.Errorf("scan AI history row: %w", err)
		}
		out = append(out, SuccessfulAIQuery{Question: question, LogicalQuery: lq})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate AI history: %w", err)
	}
	return out, nil
}

// CreateAIQueryHistory stores an AI natural-language query history entry.
func (r *Repository) CreateAIQueryHistory(ctx context.Context, entry *AIQueryHistoryEntry) error {
	promptContextJSON, err := nullableJSON(entry.PromptContext)
	if err != nil {
		return fmt.Errorf("marshal prompt context: %w", err)
	}
	aiResponseJSON, err := nullableJSON(entry.AIResponse)
	if err != nil {
		return fmt.Errorf("marshal AI response: %w", err)
	}
	logicalQueryJSON, err := nullableJSON(entry.LogicalQuery)
	if err != nil {
		return fmt.Errorf("marshal logical query: %w", err)
	}

	insert := `
		INSERT INTO ai_query_history (
			datasource_id, model_id, user_id, question, prompt_context,
			ai_response, logical_query, confidence_score, warnings
		)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6::jsonb, $7::jsonb, $8, $9)
		RETURNING id, created_at
	`
	if err := r.db.QueryRowContext(
		ctx,
		insert,
		entry.DatasourceID,
		entry.ModelID,
		entry.UserID,
		entry.Question,
		promptContextJSON,
		aiResponseJSON,
		logicalQueryJSON,
		entry.ConfidenceScore,
		pq.Array(entry.Warnings),
	).Scan(&entry.ID, &entry.CreatedAt); err != nil {
		return fmt.Errorf("insert AI query history: %w", err)
	}
	return nil
}

func scanQueryHistoryEntry(s scanner) (query.HistoryEntry, error) {
	var entry query.HistoryEntry
	var modelID, userID, compiledSQL, sqlArgs, errorMessage sql.NullString
	var rowCount, durationMs sql.NullInt64
	var logicalQueryRaw []byte

	if err := s.Scan(
		&entry.ID,
		&entry.DatasourceID,
		&modelID,
		&userID,
		&logicalQueryRaw,
		&compiledSQL,
		&sqlArgs,
		&entry.Status,
		&rowCount,
		&durationMs,
		&errorMessage,
		&entry.CreatedAt,
	); err != nil {
		return entry, fmt.Errorf("scan query history: %w", err)
	}
	if err := json.Unmarshal(logicalQueryRaw, &entry.LogicalQuery); err != nil {
		return entry, fmt.Errorf("unmarshal logical query: %w", err)
	}
	entry.ModelID = nullableStringPtr(modelID)
	entry.UserID = nullableStringPtr(userID)
	entry.CompiledSQL = nullableStringPtr(compiledSQL)
	entry.SQLArgs = nullableStringPtr(sqlArgs)
	entry.ErrorMessage = nullableStringPtr(errorMessage)
	if rowCount.Valid {
		v := int(rowCount.Int64)
		entry.RowCount = &v
	}
	if durationMs.Valid {
		v := int(durationMs.Int64)
		entry.DurationMs = &v
	}
	return entry, nil
}

func nullableJSON(value any) (*string, error) {
	if value == nil {
		return nil, nil
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	s := string(encoded)
	return &s, nil
}

func nullableStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
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
