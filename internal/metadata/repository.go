package metadata

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	platformdb "github.com/biqly/biqly/internal/platform/db"
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
		INSERT INTO datasources (
			id, name, type, dsn_encrypted, config, is_active,
			host, port, username, password_encrypted, database_name, ssl_mode, connection_params, dsn_mode
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13::jsonb, $14)
	`
	cp := defaultConnectionParams(ds.ConnectionParams)
	mode := ds.DSNMode
	if mode == "" {
		mode = DSNModeRaw
	}
	_, err := r.db.ExecContext(ctx, query,
		ds.ID, ds.Name, ds.Type, ds.DSNEncrypted, ds.Config, ds.IsActive,
		platformdb.NullIfEmptyPtr(ds.Host), nullableInt(ds.Port), platformdb.NullIfEmptyPtr(ds.Username),
		nullableEncrypted(ds.PasswordEncrypted),
		platformdb.NullIfEmptyPtr(ds.DatabaseName), platformdb.NullIfEmptyPtr(ds.SSLMode),
		cp, mode,
	)
	if err != nil {
		return fmt.Errorf("create datasource: %w", err)
	}
	return nil
}

// defaultConnectionParams normalizes a raw JSON connection_params payload so
// the database always sees a valid jsonb object literal, even when the caller
// passed nil or an empty byte slice.
func defaultConnectionParams(cp []byte) []byte {
	if len(cp) == 0 {
		return []byte("{}")
	}
	return cp
}

// UpdateDatasource updates an existing datasource connection record.
func (r *Repository) UpdateDatasource(ctx context.Context, ds *Datasource) error {
	query := `
		UPDATE datasources SET
			name = $2,
			type = $3,
			dsn_encrypted = $4,
			config = $5,
			is_active = $6,
			host = $7,
			port = $8,
			username = $9,
			password_encrypted = $10,
			database_name = $11,
			ssl_mode = $12,
			connection_params = $13::jsonb,
			dsn_mode = $14,
			updated_at = now()
		WHERE id = $1
	`
	cp := defaultConnectionParams(ds.ConnectionParams)
	mode := strings.TrimSpace(ds.DSNMode)
	if mode == "" {
		mode = DSNModeRaw
	}
	res, err := r.db.ExecContext(ctx, query,
		ds.ID, ds.Name, ds.Type, ds.DSNEncrypted, ds.Config, ds.IsActive,
		platformdb.NullIfEmptyPtr(ds.Host), nullableInt(ds.Port), platformdb.NullIfEmptyPtr(ds.Username),
		nullableEncrypted(ds.PasswordEncrypted), platformdb.NullIfEmptyPtr(ds.DatabaseName), platformdb.NullIfEmptyPtr(ds.SSLMode),
		cp, mode,
	)
	if err != nil {
		return fmt.Errorf("update datasource: %w", err)
	}
	if rows, err := res.RowsAffected(); err == nil && rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// GetDatasource retrieves a datasource by ID.
func (r *Repository) GetDatasource(ctx context.Context, id string) (*Datasource, error) {
	query := `SELECT ` + datasourceSelectColumns + ` FROM datasources WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, id)
	return r.scanDatasource(row)
}

// GetDatasourceByName retrieves a datasource by name.
func (r *Repository) GetDatasourceByName(ctx context.Context, name string) (*Datasource, error) {
	query := `SELECT ` + datasourceSelectColumns + ` FROM datasources WHERE name = $1`
	row := r.db.QueryRowContext(ctx, query, name)
	return r.scanDatasource(row)
}

// ListDatasources returns all configured datasources.
func (r *Repository) ListDatasources(ctx context.Context) ([]Datasource, error) {
	query := `SELECT ` + datasourceSelectColumns + ` FROM datasources ORDER BY created_at DESC`
	rows, err := platformdb.QuerySliceErr(ctx, r.db, "list datasources", query, nil, func(s platformdb.Scanner) (*Datasource, error) {
		return r.scanDatasource(s)
	})
	if err != nil {
		return nil, err
	}
	out := make([]Datasource, len(rows))
	for i, ds := range rows {
		out[i] = *ds
	}
	return out, nil
}

// DeleteDatasource removes a datasource by ID.
func (r *Repository) DeleteDatasource(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM datasources WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete datasource: %w", err)
	}
	return nil
}

// UpdateDatasourceSync updates the last_sync_at timestamp.
func (r *Repository) UpdateDatasourceSync(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE datasources SET last_sync_at = now(), updated_at = now() WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("update datasource sync: %w", err)
	}
	return nil
}

func (r *Repository) scanDatasource(s platformdb.Scanner) (*Datasource, error) {
	ds := &Datasource{}
	var host, username, dbName, sslMode sql.NullString
	var port sql.NullInt64
	var passEnc sql.NullString
	var cp []byte
	var dsnMode sql.NullString
	err := s.Scan(
		&ds.ID, &ds.Name, &ds.Type, &ds.DSNEncrypted, &ds.Config, &ds.IsActive, &ds.LastSyncAt, &ds.CreatedAt, &ds.UpdatedAt,
		&host, &port, &username, &passEnc, &dbName, &sslMode, &cp, &dsnMode,
	)
	if err != nil {
		return nil, fmt.Errorf("scan datasource: %w", err)
	}
	if host.Valid {
		v := host.String
		ds.Host = &v
	}
	if port.Valid {
		p := int(port.Int64)
		ds.Port = &p
	}
	if username.Valid {
		v := username.String
		ds.Username = &v
	}
	if passEnc.Valid {
		ds.PasswordEncrypted = passEnc.String
	}
	if dbName.Valid {
		v := dbName.String
		ds.DatabaseName = &v
	}
	if sslMode.Valid {
		v := sslMode.String
		ds.SSLMode = &v
	}
	if len(cp) > 0 {
		ds.ConnectionParams = append(json.RawMessage(nil), cp...)
	} else {
		ds.ConnectionParams = json.RawMessage("{}")
	}
	if dsnMode.Valid {
		ds.DSNMode = dsnMode.String
	} else {
		ds.DSNMode = DSNModeRaw
	}
	return ds, nil
}

// Schema operations

// UpsertSchemas inserts or updates schema metadata.
func (r *Repository) UpsertSchemas(ctx context.Context, datasourceID string, schemas []Schema) error {
	if len(schemas) == 0 {
		return nil
	}
	return execBatchInTx(ctx, r.db, "upsert schemas", func(tx *sql.Tx) error {
		for _, s := range schemas {
			if _, err := upsertSchemaRow(ctx, tx, datasourceID, s); err != nil {
				return err
			}
		}
		return nil
	})
}

// UpsertSchema inserts or updates schema metadata and returns the persisted ID.
func (r *Repository) UpsertSchema(ctx context.Context, datasourceID string, s Schema) (string, error) {
	return upsertSchemaRow(ctx, r.db, datasourceID, s)
}

// Table operations

// UpsertTables inserts or updates table metadata. The description column is preserved
// when the incoming description is nil/empty so that user-provided descriptions are
// not overwritten by a re-sync against a source DB that has no native comment.
func (r *Repository) UpsertTables(ctx context.Context, datasourceID string, tables []Table) error {
	if len(tables) == 0 {
		return nil
	}
	return execBatchInTx(ctx, r.db, "upsert tables", func(tx *sql.Tx) error {
		for _, t := range tables {
			if _, err := upsertTableRow(ctx, tx, datasourceID, t); err != nil {
				return err
			}
		}
		return nil
	})
}

// UpsertTable inserts or updates table metadata and returns the persisted ID.
func (r *Repository) UpsertTable(ctx context.Context, datasourceID string, t Table) (string, error) {
	return upsertTableRow(ctx, r.db, datasourceID, t)
}

// ListTables returns all stored tables for a datasource, optionally filtered by schema.
func (r *Repository) ListTables(ctx context.Context, datasourceID, schemaName string) ([]Table, error) {
	query := `
		SELECT ` + tableSelectColumns + `
		FROM tables
		WHERE datasource_id = $1
	`
	args := []any{datasourceID}
	if schemaName != "" {
		query += " AND schema_name = $2"
		args = append(args, schemaName)
	}
	query += " ORDER BY schema_name, table_name"

	return platformdb.QuerySliceErr(ctx, r.db, "list tables", query, args, scanTable)
}

// GetTable returns a single table by ID.
func (r *Repository) GetTable(ctx context.Context, id string) (*Table, error) {
	query := `SELECT ` + tableSelectColumns + ` FROM tables WHERE id = $1`
	t, err := scanTable(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		return nil, err
	}
	return &t, nil
}

// UpdateTableDescription replaces the description text on a table row.
func (r *Repository) UpdateTableDescription(ctx context.Context, id string, description *string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE tables SET description = $2, updated_at = now() WHERE id = $1`, id, description)
	if err != nil {
		return fmt.Errorf("update table description: %w", err)
	}
	return nil
}

// UpdateTableLabel replaces the human-friendly display label on a table row.
func (r *Repository) UpdateTableLabel(ctx context.Context, id string, label *string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE tables SET label = $2, updated_at = now() WHERE id = $1`, id, label)
	if err != nil {
		return fmt.Errorf("update table label: %w", err)
	}
	return nil
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
	return r.upsertEntityEmbedding(ctx, "tables", tableID, modelName, embedding)
}

// ListTableEmbeddings returns every stored embedding for a datasource. Used by
// the AI router on each NL→query request; we keep this single round-trip
// rather than fanning out per-table lookups.
func (r *Repository) ListTableEmbeddings(ctx context.Context, datasourceID string) ([]TableEmbedding, error) {
	return listEmbeddingsExpanded(ctx, r.db, "list table embeddings", `
		SELECT schema_name, table_name, embedding_model, embedding
		FROM tables
		WHERE datasource_id = $1 AND embedding IS NOT NULL
	`, []any{datasourceID}, scanTableEmbeddingRow)
}

func encodeEmbedding(vec []float32) (string, error) {
	if vec == nil {
		return "null", nil
	}
	b, err := json.Marshal(vec)
	if err != nil {
		return "", fmt.Errorf("encode embedding: %w", err)
	}
	return string(b), nil
}

func decodeEmbedding(raw []byte) ([]float32, error) {
	if len(raw) == 0 {
		return nil, nil
	}
	var vec []float32
	if err := json.Unmarshal(raw, &vec); err != nil {
		return nil, fmt.Errorf("decode embedding: %w", err)
	}
	return vec, nil
}

// Column operations

// UpsertColumns inserts or updates column metadata. Description is preserved when the
// incoming description is nil/empty so manually edited / AI-generated descriptions
// survive a metadata re-sync.
func (r *Repository) UpsertColumns(ctx context.Context, datasourceID string, columns []Column) error {
	if len(columns) == 0 {
		return nil
	}
	return execBatchInTx(ctx, r.db, "upsert columns", func(tx *sql.Tx) error {
		return upsertColumnsBatch(ctx, tx, datasourceID, columns)
	})
}

// ListColumns returns columns for a datasource, optionally scoped to a single table.
func (r *Repository) ListColumns(ctx context.Context, datasourceID, schemaName, tableName string) ([]Column, error) {
	query := `
		SELECT ` + columnSelectColumns + `
		FROM columns
		WHERE datasource_id = $1
	`
	args := []any{datasourceID}
	if schemaName != "" {
		query += fmt.Sprintf(" AND schema_name = $%d", len(args)+1)
		args = append(args, schemaName)
	}
	if tableName != "" {
		query += fmt.Sprintf(" AND table_name = $%d", len(args)+1)
		args = append(args, tableName)
	}
	query += " ORDER BY schema_name, table_name, ordinal_position"

	return platformdb.QuerySliceErr(ctx, r.db, "list columns", query, args, scanColumn)
}

// GetColumn returns a single column by ID.
func (r *Repository) GetColumn(ctx context.Context, id string) (*Column, error) {
	query := `SELECT ` + columnSelectColumns + ` FROM columns WHERE id = $1`
	c, err := scanColumn(r.db.QueryRowContext(ctx, query, id))
	if err != nil {
		return nil, err
	}
	return &c, nil
}

func (r *Repository) ListPermissionPolicies(ctx context.Context, datasourceID string) ([]PermissionPolicyRecord, error) {
	return platformdb.QuerySliceErr(ctx, r.db, "list permission policies", `
		SELECT denied_fields, row_filters
		FROM permissions
		WHERE datasource_id::text = $1
	`, []any{datasourceID}, scanPermissionPolicy)
}

// UpdateColumnDescription replaces the description text on a column row.
func (r *Repository) UpdateColumnDescription(ctx context.Context, id string, description *string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE columns SET description = $2 WHERE id = $1`, id, description)
	if err != nil {
		return fmt.Errorf("update column description: %w", err)
	}
	return nil
}

// UpsertColumnEmbedding stores (or replaces) the embedding vector for a single
// column. It mirrors table embedding storage and keeps deployments independent
// of pgvector.
func (r *Repository) UpsertColumnEmbedding(ctx context.Context, columnID, modelName string, embedding []float32) error {
	return r.upsertEntityEmbedding(ctx, "columns", columnID, modelName, embedding)
}

// ListColumnEmbeddings returns every stored column embedding for a datasource in
// one round-trip. Router code decides whether coverage is complete enough to use.
func (r *Repository) ListColumnEmbeddings(ctx context.Context, datasourceID string) ([]ColumnEmbedding, error) {
	return listEmbeddingsExpanded(ctx, r.db, "list column embeddings", `
		SELECT schema_name, table_name, column_name, embedding_model, embedding
		FROM columns
		WHERE datasource_id = $1 AND embedding IS NOT NULL
	`, []any{datasourceID}, scanColumnEmbeddingRow)
}

// Relation operations

// UpsertRelations inserts or updates relation metadata.
func (r *Repository) UpsertRelations(ctx context.Context, datasourceID string, relations []Relation) error {
	if len(relations) == 0 {
		return nil
	}
	return execBatchInTx(ctx, r.db, "upsert relations", func(tx *sql.Tx) error {
		return upsertRelationsBatch(ctx, tx, datasourceID, relations)
	})
}

// ListRelations returns foreign-key relationships for a datasource.
func (r *Repository) ListRelations(ctx context.Context, datasourceID string) ([]Relation, error) {
	query := `
		SELECT ` + relationSelectColumns + `
		FROM relations
		WHERE datasource_id = $1
		ORDER BY from_schema, from_table, from_column
	`
	return platformdb.QuerySliceErr(ctx, r.db, "list relations", query, []any{datasourceID}, scanRelation)
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
			sql_args, status, row_count, duration_ms, error_message,
			query_fingerprint
		)
		VALUES ($1, $2, $3, $4::jsonb, $5, $6::jsonb, $7, $8, $9, $10, $11)
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
		sql.NullString{String: entry.Fingerprint, Valid: entry.Fingerprint != ""},
	).Scan(&entry.ID, &entry.CreatedAt); err != nil {
		return fmt.Errorf("insert query history: %w", err)
	}
	return nil
}

// ListQueryHistory returns recent query history entries (newest first), capped at limit.
func (r *Repository) ListQueryHistory(ctx context.Context, limit int) ([]query.HistoryEntry, error) {
	if limit <= 0 {
		limit = 100
	}
	return platformdb.QuerySliceErr(ctx, r.db, "list query history", `
		SELECT id, datasource_id, model_id, user_id, logical_query, compiled_sql,
			sql_args, status, row_count, duration_ms, error_message,
			query_fingerprint, created_at
		FROM query_history
		ORDER BY created_at DESC
		LIMIT $1
	`, []any{limit}, scanQueryHistoryEntry)
}

// GetQueryHistory returns one query history entry by ID.
func (r *Repository) GetQueryHistory(ctx context.Context, id string) (*query.HistoryEntry, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, datasource_id, model_id, user_id, logical_query, compiled_sql,
			sql_args, status, row_count, duration_ms, error_message,
			query_fingerprint, created_at
		FROM query_history
		WHERE id = $1
	`, id)
	entry, err := scanQueryHistoryEntry(row)
	if err != nil {
		return nil, fmt.Errorf("get query history: %w", err)
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
func (r *Repository) ListSuccessfulAIQueries(ctx context.Context, datasourceID string, modelID *string, limit int) ([]SuccessfulAIQuery, error) {
	if limit <= 0 {
		return nil, nil
	}
	const minConfidence = 0.7
	q := `
		SELECT question, logical_query
		FROM ai_query_history
		WHERE datasource_id = $1::uuid
		  AND ($2::uuid IS NULL OR model_id = $2::uuid)
		  AND confidence_score >= $3
		  AND (warnings IS NULL OR cardinality(warnings) = 0)
		  AND logical_query IS NOT NULL
		ORDER BY created_at DESC
		LIMIT $4
	`
	var modelArg sql.NullString
	if modelID != nil && *modelID != "" {
		modelArg = sql.NullString{String: *modelID, Valid: true}
	}
	return platformdb.QuerySliceErr(ctx, r.db, "list successful AI queries", q,
		[]any{datasourceID, modelArg, minConfidence, limit},
		func(s platformdb.Scanner) (SuccessfulAIQuery, error) {
			var question string
			var lq []byte
			if err := s.Scan(&question, &lq); err != nil {
				return SuccessfulAIQuery{}, fmt.Errorf("scan AI history row: %w", err)
			}
			return SuccessfulAIQuery{Question: question, LogicalQuery: lq}, nil
		})
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

	outcome := entry.OutcomeStatus
	if outcome == "" {
		outcome = AIOutcomeUnknown
	}

	insert := `
		INSERT INTO ai_query_history (
			datasource_id, model_id, user_id, question, prompt_context,
			ai_response, logical_query, confidence_score, warnings,
			outcome_status, retry_count, needs_clarification,
			model_used, token_count, cost_usd, latency_ms
		)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6::jsonb, $7::jsonb, $8, $9,
			$10, $11, $12, $13, $14, $15, $16)
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
		outcome,
		entry.RetryCount,
		entry.NeedsClarification,
		entry.ModelUsed,
		entry.TokenCount,
		entry.CostUSD,
		entry.LatencyMs,
	).Scan(&entry.ID, &entry.CreatedAt); err != nil {
		return fmt.Errorf("insert AI query history: %w", err)
	}
	return nil
}

func scanTable(s platformdb.Scanner) (Table, error) {
	var t Table
	if err := s.Scan(&t.ID, &t.DatasourceID, &t.SchemaID, &t.SchemaName, &t.TableName, &t.TableType, &t.RowEstimate, &t.Description, &t.Label, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return t, fmt.Errorf("scan table: %w", err)
	}
	return t, nil
}

func scanColumn(s platformdb.Scanner) (Column, error) {
	var c Column
	if err := s.Scan(&c.ID, &c.DatasourceID, &c.TableID, &c.SchemaName, &c.TableName, &c.ColumnName, &c.DataType, &c.Nullable, &c.OrdinalPosition, &c.CharMaxLength, &c.NumericPrecision, &c.NumericScale, &c.ColumnDefault, &c.Description, &c.IsPrimaryKey, &c.IsForeignKey, &c.ReferencedSchema, &c.ReferencedTable, &c.ReferencedColumn, &c.CreatedAt); err != nil {
		return c, fmt.Errorf("scan column: %w", err)
	}
	return c, nil
}

func scanRelation(s platformdb.Scanner) (Relation, error) {
	var rel Relation
	if err := s.Scan(
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
		return rel, fmt.Errorf("scan relation: %w", err)
	}
	return rel, nil
}

func scanPermissionPolicy(s platformdb.Scanner) (PermissionPolicyRecord, error) {
	var (
		policy     PermissionPolicyRecord
		rowFilters []byte
		denied     pq.StringArray
	)
	if err := s.Scan(&denied, &rowFilters); err != nil {
		return policy, fmt.Errorf("scan permission policy: %w", err)
	}
	policy.DeniedFields = []string(denied)
	if len(rowFilters) > 0 {
		if err := json.Unmarshal(rowFilters, &policy.RowFilters); err != nil {
			return policy, fmt.Errorf("row filters: %w", err)
		}
	}
	return policy, nil
}

func scanQueryHistoryEntry(s platformdb.Scanner) (query.HistoryEntry, error) {
	var entry query.HistoryEntry
	var modelID, userID, compiledSQL, sqlArgs, errorMessage, fingerprint sql.NullString
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
		&fingerprint,
		&entry.CreatedAt,
	); err != nil {
		return entry, fmt.Errorf("scan query history: %w", err)
	}
	if err := json.Unmarshal(logicalQueryRaw, &entry.LogicalQuery); err != nil {
		return entry, fmt.Errorf("unmarshal logical query: %w", err)
	}
	entry.ModelID = platformdb.StringPtrFromNull(modelID)
	entry.UserID = platformdb.StringPtrFromNull(userID)
	entry.CompiledSQL = platformdb.StringPtrFromNull(compiledSQL)
	entry.SQLArgs = platformdb.StringPtrFromNull(sqlArgs)
	entry.ErrorMessage = platformdb.StringPtrFromNull(errorMessage)
	if fingerprint.Valid {
		entry.Fingerprint = fingerprint.String
	}
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
		return nil, fmt.Errorf("marshal json: %w", err)
	}
	s := string(encoded)
	return &s, nil
}

// Search operations

// SearchColumns searches columns by name or description.
func (r *Repository) SearchColumns(ctx context.Context, datasourceID, searchTerm string) ([]Column, error) {
	query := `
		SELECT ` + columnSelectColumns + `
		FROM columns
		WHERE datasource_id = $1 AND (column_name ILIKE $2 OR description ILIKE $2)
		ORDER BY table_name, column_name
	`
	return platformdb.QuerySliceErr(ctx, r.db, "search columns", query, []any{datasourceID, "%" + searchTerm + "%"}, scanColumn)
}

// SearchTables searches tables by name or description.
func (r *Repository) SearchTables(ctx context.Context, datasourceID, searchTerm string) ([]Table, error) {
	query := `
		SELECT ` + tableSelectColumns + `
		FROM tables
		WHERE datasource_id = $1 AND (table_name ILIKE $2 OR description ILIKE $2)
		ORDER BY schema_name, table_name
	`
	return platformdb.QuerySliceErr(ctx, r.db, "search tables", query, []any{datasourceID, "%" + searchTerm + "%"}, scanTable)
}

func nullableInt(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullableEncrypted(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}
