package metadata

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	platformdb "github.com/biqly/biqly/internal/platform/db"
	"github.com/biqly/biqly/internal/platform/db/pgarray"
	pkgquery "github.com/biqly/biqly/pkg/query"
	"github.com/bytedance/sonic"
)

// Repository handles metadata database operations.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new metadata repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// DB returns the underlying sql.DB instance.
func (r *Repository) DB() *sql.DB {
	return r.db
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
		platformdb.NullIfEmptyPtr(ds.Host), platformdb.NullIfNilIntPtr(ds.Port), platformdb.NullIfEmptyPtr(ds.Username),
		platformdb.NullIfEmpty(ds.PasswordEncrypted),
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
		platformdb.NullIfEmptyPtr(ds.Host), platformdb.NullIfNilIntPtr(ds.Port), platformdb.NullIfEmptyPtr(ds.Username),
		platformdb.NullIfEmpty(ds.PasswordEncrypted), platformdb.NullIfEmptyPtr(ds.DatabaseName), platformdb.NullIfEmptyPtr(ds.SSLMode),
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
	query := `SELECT ` + datasourceSelectColumns + ` FROM datasources WHERE id = $1` // nosemgrep: go.lang.security.audit.database.string-formatted-query.string-formatted-query
	row := r.db.QueryRowContext(ctx, query, id)
	return r.scanDatasource(row)
}

// GetDatasourceByName retrieves a datasource by name.
func (r *Repository) GetDatasourceByName(ctx context.Context, name string) (*Datasource, error) {
	query := `SELECT ` + datasourceSelectColumns + ` FROM datasources WHERE name = $1` // nosemgrep: go.lang.security.audit.database.string-formatted-query.string-formatted-query
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
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("delete datasource tx begin: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// 1. Delete from leaf tables referencing semantic_models
	if _, err := tx.ExecContext(ctx, `DELETE FROM semantic_dimensions WHERE model_id IN (SELECT id FROM semantic_models WHERE datasource_id = $1)`, id); err != nil {
		return fmt.Errorf("delete semantic_dimensions: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM semantic_metrics WHERE model_id IN (SELECT id FROM semantic_models WHERE datasource_id = $1)`, id); err != nil {
		return fmt.Errorf("delete semantic_metrics: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM semantic_joins WHERE model_id IN (SELECT id FROM semantic_models WHERE datasource_id = $1)`, id); err != nil {
		return fmt.Errorf("delete semantic_joins: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM semantic_context_snapshots WHERE model_id IN (SELECT id FROM semantic_models WHERE datasource_id = $1)`, id); err != nil {
		return fmt.Errorf("delete semantic_context_snapshots: %w", err)
	}

	// 2. Delete from other child tables of datasources/semantic_models
	tablesToDelete := []string{
		"drift_reports",
		"ai_feedback",
		"few_shot_examples",
		"permissions",
		"ai_query_history",
		"query_saved",
		"query_history",
		"business_glossary_terms",
		"relations",
		"columns",
		"tables",
		"schemas",
		"semantic_models",
	}
	for _, tbl := range tablesToDelete {
		if _, err := tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM %s WHERE datasource_id = $1", tbl), id); err != nil {
			return fmt.Errorf("delete from %s: %w", tbl, err)
		}
	}

	// 3. Delete from datasources table itself
	if _, err := tx.ExecContext(ctx, `DELETE FROM datasources WHERE id = $1`, id); err != nil {
		return fmt.Errorf("delete from datasources: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("delete datasource commit: %w", err)
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

func (*Repository) scanDatasource(s platformdb.Scanner) (*Datasource, error) {
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
	ds.Host = platformdb.StringPtrFromNull(host)
	ds.Port = platformdb.IntPtrFromNull(port)
	ds.Username = platformdb.StringPtrFromNull(username)
	if passEnc.Valid {
		ds.PasswordEncrypted = passEnc.String
	}
	ds.DatabaseName = platformdb.StringPtrFromNull(dbName)
	ds.SSLMode = platformdb.StringPtrFromNull(sslMode)
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
	query := `SELECT ` + tableSelectColumns + ` FROM tables WHERE id = $1` // nosemgrep: go.lang.security.audit.database.string-formatted-query.string-formatted-query
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

// UpdateTableDescriptionAndLabel writes description and/or label in a single
// SQL statement so partial updates do not leak between the two columns.
// nil fields are left untouched.
func (r *Repository) UpdateTableDescriptionAndLabel(ctx context.Context, id string, description, label *string) error {
	if description == nil && label == nil {
		return nil
	}
	const q = `
		UPDATE tables
		SET description = CASE WHEN $2::boolean THEN $3 ELSE description END,
		    label       = CASE WHEN $4::boolean THEN $5 ELSE label END,
		    updated_at  = now()
		WHERE id = $1`
	_, err := r.db.ExecContext(ctx, q,
		id,
		description != nil, derefStringOrEmpty(description),
		label != nil, derefStringOrEmpty(label),
	)
	if err != nil {
		return fmt.Errorf("update table description+label: %w", err)
	}
	return nil
}

// UpdateTableDisplayExpression sets (or clears, when nil) the display
// expression used to label rows of this table in UIs.
func (r *Repository) UpdateTableDisplayExpression(ctx context.Context, id string, expr *string) error {
	const q = `UPDATE tables SET display_expression = $2, updated_at = now() WHERE id = $1`
	if _, err := r.db.ExecContext(ctx, q, id, expr); err != nil {
		return fmt.Errorf("update table display expression: %w", err)
	}
	return nil
}

func derefStringOrEmpty(p *string) any {
	if p == nil {
		return ""
	}
	return *p
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
	b, err := sonic.Marshal(vec)
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
	if err := sonic.Unmarshal(raw, &vec); err != nil {
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
	query := `SELECT ` + columnSelectColumns + ` FROM columns WHERE id = $1` // nosemgrep: go.lang.security.audit.database.string-formatted-query.string-formatted-query
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
		WHERE datasource_id = $1::uuid
	`, []any{datasourceID}, scanPermissionPolicy)
}

// UpdateColumnDescription replaces the description text on a column row.
func (r *Repository) UpdateColumnDescription(ctx context.Context, id string, description *string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE columns SET description = $2, updated_at = now() WHERE id = $1`, id, description)
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

// ListRelationDetails returns foreign-key relationships for a datasource, each
// enriched with a description from a matching active semantic-model join
// (either direction) when one exists. Descriptions live on semantic_joins, so
// this is the bridge that lets the metadata UI show them.
func (r *Repository) ListRelationDetails(ctx context.Context, datasourceID string) ([]RelationDetail, error) {
	// LATERAL-match each relation to its best active semantic join (either
	// direction) so the row carries both the description and the join id. The
	// id is returned even when the description is still empty, so the UI can
	// offer per-relationship AI describe for joins that just haven't been
	// described yet.
	// The relation's own description wins; the matched semantic join's
	// description remains as a fallback for joins documented before relations
	// carried their own descriptions. The lateral output columns are aliased
	// (jid/jdesc) so the bare column names in relationSelectColumns (e.g.
	// "id") stay unambiguous — otherwise an exposed sj.id collides with
	// relations.id.
	query := `
		SELECT ` + relationSelectColumns + `,
			COALESCE(NULLIF(relations.description, ''), j.jdesc, '') AS description,
			COALESCE(j.jid::text, '') AS semantic_join_id
		FROM relations
		LEFT JOIN LATERAL (
			SELECT sj.id AS jid, sj.description AS jdesc
			FROM semantic_joins sj
			JOIN semantic_models sm ON sm.id = sj.model_id
			WHERE sm.datasource_id = relations.datasource_id
			  AND sj.is_active
			  AND ((sj.from_table = relations.from_table AND sj.from_column = relations.from_column
					AND sj.to_table = relations.to_table AND sj.to_column = relations.to_column)
				OR (sj.from_table = relations.to_table AND sj.from_column = relations.to_column
					AND sj.to_table = relations.from_table AND sj.to_column = relations.from_column))
			ORDER BY (sj.description <> '') DESC, sj.created_at DESC
			LIMIT 1
		) j ON true
		WHERE datasource_id = $1
		ORDER BY from_schema, from_table, from_column
	`
	return platformdb.QuerySliceErr(ctx, r.db, "list relation details", query, []any{datasourceID}, scanRelationDetail)
}

// UpdateRelationDescription sets the English (fallback-locale) description of
// a relation; localized values overlay via entity_translations.
func (r *Repository) UpdateRelationDescription(ctx context.Context, relationID, description string) error {
	_, err := r.db.ExecContext(ctx,
		`UPDATE relations SET description = $2 WHERE id = $1`, relationID, description)
	if err != nil {
		return fmt.Errorf("update relation description: %w", err)
	}
	return nil
}

// History operations

// CreateQueryHistory stores a structured query execution history entry.
func (r *Repository) CreateQueryHistory(ctx context.Context, entry *pkgquery.HistoryEntry) error {
	logicalQueryJSON, err := sonic.Marshal(entry.LogicalQuery)
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
func (r *Repository) ListQueryHistory(ctx context.Context, limit int) ([]pkgquery.HistoryEntry, error) {
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
func (r *Repository) GetQueryHistory(ctx context.Context, id string) (*pkgquery.HistoryEntry, error) {
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
			model_used, prompt_tokens, completion_tokens, token_count, cost_usd, latency_ms,
			ab_experiment_id, ab_variant_id, memory_recall_used, memory_recall_hit_count
		)
		VALUES ($1, $2, $3, $4, $5::jsonb, $6::jsonb, $7::jsonb, $8, $9,
			$10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22)
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
		pgarray.Strings(entry.Warnings),
		outcome,
		entry.RetryCount,
		entry.NeedsClarification,
		entry.ModelUsed,
		entry.PromptTokens,
		entry.CompletionTokens,
		entry.TokenCount,
		entry.CostUSD,
		entry.LatencyMs,
		entry.ABExperimentID,
		entry.ABVariantID,
		entry.MemoryRecallUsed,
		entry.MemoryRecallHitCount,
	).Scan(&entry.ID, &entry.CreatedAt); err != nil {
		return fmt.Errorf("insert AI query history: %w", err)
	}
	return nil
}

// ListAIQueryHistory returns recent AI query history entries (newest first).
// userID filters to a single user when non-empty; passing "" returns all rows
// (caller must enforce admin permission before calling).
func (r *Repository) ListAIQueryHistory(ctx context.Context, userID string, limit int) ([]AIQueryHistoryEntry, error) {
	if limit <= 0 {
		limit = 50
	}

	const baseQuery = `SELECT id, datasource_id, model_id, user_id, question, prompt_context,
		       ai_response, logical_query, confidence_score, warnings, outcome_status,
		       retry_count, needs_clarification, model_used, prompt_tokens, completion_tokens,
		       token_count, cost_usd, latency_ms, created_at, ab_experiment_id, ab_variant_id,
		       memory_recall_used, memory_recall_hit_count FROM ai_query_history`
	const filteredQuery = baseQuery + ` WHERE user_id = $1 ORDER BY created_at DESC LIMIT $2`
	const allQuery = baseQuery + ` ORDER BY created_at DESC LIMIT $1`

	var (
		rows *sql.Rows
		err  error
	)
	if userID != "" {
		rows, err = r.db.QueryContext(ctx, filteredQuery, userID, limit)
	} else {
		rows, err = r.db.QueryContext(ctx, allQuery, limit)
	}
	if err != nil {
		return nil, fmt.Errorf("query AI history: %w", err)
	}
	defer func() { _ = rows.Close() }()

	list := make([]AIQueryHistoryEntry, 0, 64)
	for rows.Next() {
		entry, err := scanAIHistoryEntry(rows)
		if err != nil {
			return nil, err
		}
		list = append(list, entry)
	}
	return list, rows.Err()
}

func scanAIHistoryEntry(s platformdb.Scanner) (AIQueryHistoryEntry, error) {
	var entry AIQueryHistoryEntry
	var modelID, userID, outcome, modelUsed sql.NullString
	var promptCtx, aiResp, logicalQ []byte
	var confidence, cost sql.NullFloat64
	var warnings pgarray.StringArray
	var retryCount sql.NullInt64
	var needsClarification sql.NullBool
	var promptTokens, completionTokens, tokenCount, latencyMs sql.NullInt64

	var abExpID, abVarID sql.NullString

	if err := s.Scan(
		&entry.ID, &entry.DatasourceID, &modelID, &userID, &entry.Question,
		&promptCtx, &aiResp, &logicalQ, &confidence, &warnings, &outcome,
		&retryCount, &needsClarification, &modelUsed, &promptTokens, &completionTokens,
		&tokenCount, &cost, &latencyMs, &entry.CreatedAt, &abExpID, &abVarID,
		&entry.MemoryRecallUsed, &entry.MemoryRecallHitCount,
	); err != nil {
		return entry, fmt.Errorf("scan AI history row: %w", err)
	}

	if modelID.Valid {
		entry.ModelID = new(modelID.String)
	}
	if userID.Valid {
		entry.UserID = new(userID.String)
	}
	if outcome.Valid {
		entry.OutcomeStatus = outcome.String
	}
	if modelUsed.Valid {
		entry.ModelUsed = new(modelUsed.String)
	}
	if confidence.Valid {
		entry.ConfidenceScore = new(confidence.Float64)
	}
	if cost.Valid {
		entry.CostUSD = new(cost.Float64)
	}
	if retryCount.Valid {
		entry.RetryCount = int(retryCount.Int64)
	}
	if needsClarification.Valid {
		entry.NeedsClarification = needsClarification.Bool
	}
	if promptTokens.Valid {
		entry.PromptTokens = new(int(promptTokens.Int64))
	}
	if completionTokens.Valid {
		entry.CompletionTokens = new(int(completionTokens.Int64))
	}
	if tokenCount.Valid {
		entry.TokenCount = new(int(tokenCount.Int64))
	}
	if latencyMs.Valid {
		entry.LatencyMs = new(int(latencyMs.Int64))
	}
	if abExpID.Valid {
		entry.ABExperimentID = new(abExpID.String)
	}
	if abVarID.Valid {
		entry.ABVariantID = new(abVarID.String)
	}
	if len(promptCtx) > 0 {
		if err := sonic.Unmarshal(promptCtx, &entry.PromptContext); err != nil {
			return entry, fmt.Errorf("decode prompt context: %w", err)
		}
	}
	if len(aiResp) > 0 {
		if err := sonic.Unmarshal(aiResp, &entry.AIResponse); err != nil {
			return entry, fmt.Errorf("decode ai response: %w", err)
		}
	}
	if len(logicalQ) > 0 {
		if err := sonic.Unmarshal(logicalQ, &entry.LogicalQuery); err != nil {
			return entry, fmt.Errorf("decode logical query: %w", err)
		}
	}
	entry.Warnings = []string(warnings)

	return entry, nil
}

func scanTable(s platformdb.Scanner) (Table, error) {
	var t Table
	if err := s.Scan(&t.ID, &t.DatasourceID, &t.SchemaID, &t.SchemaName, &t.TableName, &t.TableType, &t.RowEstimate, &t.Description, &t.Label, &t.DisplayExpression, &t.CreatedAt, &t.UpdatedAt); err != nil {
		return t, fmt.Errorf("scan table: %w", err)
	}
	return t, nil
}

func scanColumn(s platformdb.Scanner) (Column, error) {
	var c Column
	if err := s.Scan(&c.ID, &c.DatasourceID, &c.TableID, &c.SchemaName, &c.TableName, &c.ColumnName, &c.DataType, &c.Nullable, &c.OrdinalPosition, &c.CharMaxLength, &c.NumericPrecision, &c.NumericScale, &c.ColumnDefault, &c.Description, &c.IsPrimaryKey, &c.IsForeignKey, &c.ReferencedSchema, &c.ReferencedTable, &c.ReferencedColumn, &c.CreatedAt, &c.PIIType, &c.PIIConfidence, &c.PIIDetectedAt, &c.PIIReviewedBy, &c.PIIMaskingStrategy); err != nil {
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

func scanRelationDetail(s platformdb.Scanner) (RelationDetail, error) {
	var rel RelationDetail
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
		&rel.Description,
		&rel.SemanticJoinID,
	); err != nil {
		return rel, fmt.Errorf("scan relation detail: %w", err)
	}
	return rel, nil
}

func scanPermissionPolicy(s platformdb.Scanner) (PermissionPolicyRecord, error) {
	var (
		policy     PermissionPolicyRecord
		rowFilters []byte
		denied     pgarray.StringArray
	)
	if err := s.Scan(&denied, &rowFilters); err != nil {
		return policy, fmt.Errorf("scan permission policy: %w", err)
	}
	policy.DeniedFields = []string(denied)
	if len(rowFilters) > 0 {
		if err := sonic.Unmarshal(rowFilters, &policy.RowFilters); err != nil {
			return policy, fmt.Errorf("row filters: %w", err)
		}
	}
	return policy, nil
}

func scanQueryHistoryEntry(s platformdb.Scanner) (pkgquery.HistoryEntry, error) {
	var entry pkgquery.HistoryEntry
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
	if err := sonic.Unmarshal(logicalQueryRaw, &entry.LogicalQuery); err != nil {
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
		entry.RowCount = new(int(rowCount.Int64))
	}
	if durationMs.Valid {
		entry.DurationMs = new(int(durationMs.Int64))
	}
	return entry, nil
}

func nullableJSON(value any) (*string, error) {
	if value == nil {
		return nil, nil //nolint:nilnil // nil value serializes as SQL NULL
	}
	encoded, err := sonic.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("marshal json: %w", err)
	}
	return new(string(encoded)), nil
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
