package metadata

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	platformdb "github.com/biqly/biqly/internal/platform/db"
	"github.com/biqly/biqly/internal/platform/db/pgarray"
)

// ErrSavedQueryNotFound is returned when a saved-query id does not exist.
var ErrSavedQueryNotFound = errors.New("saved query not found")

// SavedQueryRow is a unified grounding record. It can act as an EXAMPLE
// (embedding-RAG few-shot grounding, source="example"), a SKILL (executable
// parameterized LogicalQuery, source="skill", runnable=true), or carry the
// columns of both. It merges the legacy ai_confirmed_queries and ai_skills
// tables into one store.
type SavedQueryRow struct {
	ID                string
	DatasourceID      string
	ModelID           string
	Name              string
	Description       string
	Question          string
	QuestionHash      string
	SQLQuery          string
	LogicalQuery      []byte
	Parameters        []byte
	QuestionEmbedding []float32
	SemanticModelHash string
	Tags              []string
	Source            string
	Runnable          bool
	IsActive          bool
	CreatedBy         string
	Version           int
	LastVerifiedAt    *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// SavedQueryInsert is the input for creating a saved query (example or skill).
type SavedQueryInsert struct {
	DatasourceID      string
	ModelID           string
	Name              string
	Description       string
	Question          string
	QuestionHash      string
	SQLQuery          string
	LogicalQuery      []byte
	Parameters        []byte
	QuestionEmbedding []float32
	SemanticModelHash string
	Tags              []string
	Source            string
	Runnable          bool
	CreatedBy         string
}

// SavedQueryUpdate replaces the editable fields of a saved query; the version
// is bumped. It mirrors the legacy skill update surface.
type SavedQueryUpdate struct {
	Name         string
	Description  string
	Question     string
	LogicalQuery []byte
	Parameters   []byte
	Tags         []string
	IsActive     bool
}

// SavedQueryFilter scopes ListSavedQueries. An empty DatasourceID lists across
// all datasources (admin/MCP catalog view), matching the legacy skill listing.
type SavedQueryFilter struct {
	DatasourceID string
	RunnableOnly bool
	Source       string
}

const savedQueryColumns = `id::text, datasource_id::text, COALESCE(model_id::text, ''), name, description, question,
	question_hash, sql_query, logical_query, parameters, question_embedding, semantic_model_hash,
	tags, source, runnable, is_active, created_by, version, last_verified_at, created_at, updated_at`

// InsertSavedQuery stores a new saved query and returns its generated id.
func (r *Repository) InsertSavedQuery(ctx context.Context, in SavedQueryInsert) (string, error) {
	var modelID any
	if in.ModelID != "" {
		modelID = in.ModelID
	}
	embeddingJSON, err := encodeEmbedding(in.QuestionEmbedding)
	if err != nil {
		return "", err
	}
	params := in.Parameters
	if len(params) == 0 {
		params = []byte("[]")
	}
	source := in.Source
	if source == "" {
		source = "example"
	}
	var logicalQuery any
	if len(in.LogicalQuery) > 0 {
		logicalQuery = in.LogicalQuery
	}
	tags := in.Tags
	if tags == nil {
		tags = []string{}
	}
	var id string
	err = r.db.QueryRowContext(ctx, `
		INSERT INTO ai_saved_queries (
			datasource_id, model_id, name, description, question, question_hash,
			sql_query, logical_query, parameters, question_embedding, semantic_model_hash,
			tags, source, runnable, created_by
		) VALUES (
			$1::uuid, $2::uuid, $3, $4, $5, $6,
			$7, $8::jsonb, $9::jsonb, $10::jsonb, $11,
			$12, $13, $14, $15
		)
		RETURNING id::text
	`, in.DatasourceID, modelID, in.Name, in.Description, in.Question, in.QuestionHash,
		in.SQLQuery, logicalQuery, params, embeddingJSON, in.SemanticModelHash,
		pgarray.Strings(tags), source, in.Runnable, in.CreatedBy).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert saved query: %w", err)
	}
	return id, nil
}

// UpsertSavedQueryExample stores or refreshes a grounding example (source
// 'example'), keyed like the legacy ai_confirmed_queries unique key so the
// positive-feedback dual-write updates in place instead of duplicating recall
// rows. Mirrors Repository.UpsertConfirmedQuery; keeps new confirmations
// visible to few-shot recall (which now reads ai_saved_queries).
func (r *Repository) UpsertSavedQueryExample(ctx context.Context, in ConfirmedQueryUpsert) error {
	embeddingJSON, err := encodeEmbedding(in.QuestionEmbedding)
	if err != nil {
		return err
	}
	var modelID any = in.ModelID
	if in.ModelID == "" {
		modelID = nil
	}
	_, err = r.db.ExecContext(ctx, `
		INSERT INTO ai_saved_queries (
			datasource_id, model_id, created_by, question_hash, question, sql_query,
			semantic_model_hash, question_embedding, source, runnable, is_active, updated_at
		) VALUES (
			$1::uuid, $2::uuid, $3, $4, $5, $6, $7, $8::jsonb, 'example', false, true, NOW()
		)
		ON CONFLICT (
			datasource_id, question_hash, semantic_model_hash,
			COALESCE(model_id, '00000000-0000-0000-0000-000000000000'::uuid)
		) WHERE source = 'example'
		DO UPDATE SET
			question           = EXCLUDED.question,
			sql_query          = EXCLUDED.sql_query,
			question_embedding = EXCLUDED.question_embedding,
			created_by         = EXCLUDED.created_by,
			is_active          = true,
			updated_at         = NOW()
	`, in.DatasourceID, modelID, in.UserID, in.QuestionHash, in.NLQuery, in.SQLQuery, in.SemanticModelHash, embeddingJSON)
	if err != nil {
		return fmt.Errorf("upsert saved query example: %w", err)
	}
	return nil
}

// ListSavedQueries returns saved queries for the filter, newest-updated first.
func (r *Repository) ListSavedQueries(ctx context.Context, f SavedQueryFilter) ([]SavedQueryRow, error) {
	q := `SELECT ` + savedQueryColumns + ` FROM ai_saved_queries
		WHERE ($1 = '' OR datasource_id::text = $1)
		  AND ($2 = false OR runnable = true)
		  AND ($3 = '' OR source = $3)
		ORDER BY updated_at DESC`
	return platformdb.QuerySliceErr(ctx, r.db, "list saved queries", q,
		[]any{f.DatasourceID, f.RunnableOnly, f.Source},
		scanSavedQueryRow)
}

// GetSavedQueriesByIDs returns the saved queries with the given ids, scoped to
// datasourceID so a request can never inject another datasource's queries as
// grounding. Ordering follows the ids slice so explicit selections keep their
// priority; unknown or out-of-scope ids are silently dropped.
func (r *Repository) GetSavedQueriesByIDs(ctx context.Context, datasourceID string, ids []string) ([]SavedQueryRow, error) {
	if datasourceID == "" || len(ids) == 0 {
		return nil, nil
	}
	q := `SELECT ` + savedQueryColumns + ` FROM ai_saved_queries
		WHERE datasource_id = $1::uuid AND id = ANY($2::uuid[])`
	rows, err := platformdb.QuerySliceErr(ctx, r.db, "get saved queries by ids", q,
		[]any{datasourceID, pgarray.Strings(ids)},
		scanSavedQueryRow)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]SavedQueryRow, len(rows))
	for _, row := range rows {
		byID[row.ID] = row
	}
	ordered := make([]SavedQueryRow, 0, len(rows))
	seen := make(map[string]bool, len(rows))
	for _, id := range ids {
		if seen[id] {
			continue
		}
		if row, ok := byID[id]; ok {
			ordered = append(ordered, row)
			seen[id] = true
		}
	}
	return ordered, nil
}

// GetSavedQuery returns a single saved query by id.
func (r *Repository) GetSavedQuery(ctx context.Context, id string) (SavedQueryRow, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+savedQueryColumns+` FROM ai_saved_queries WHERE id = $1::uuid`, id)
	sq, err := scanSavedQueryRow(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SavedQueryRow{}, fmt.Errorf("saved query %s: %w", id, ErrSavedQueryNotFound)
		}
		return SavedQueryRow{}, err
	}
	return sq, nil
}

// UpdateSavedQuery replaces the editable fields of a saved query and bumps its version.
func (r *Repository) UpdateSavedQuery(ctx context.Context, id string, in SavedQueryUpdate) error {
	params := in.Parameters
	if len(params) == 0 {
		params = []byte("[]")
	}
	var logicalQuery any
	if len(in.LogicalQuery) > 0 {
		logicalQuery = in.LogicalQuery
	}
	tags := in.Tags
	if tags == nil {
		tags = []string{}
	}
	res, err := r.db.ExecContext(ctx, `
		UPDATE ai_saved_queries
		SET name = $2, description = $3, question = $4, logical_query = $5::jsonb, parameters = $6::jsonb,
			tags = $7, is_active = $8, version = version + 1, updated_at = now()
		WHERE id = $1::uuid
	`, id, in.Name, in.Description, in.Question, logicalQuery, params, pgarray.Strings(tags), in.IsActive)
	if err != nil {
		return fmt.Errorf("update saved query: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update saved query affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("saved query %s: %w", id, ErrSavedQueryNotFound)
	}
	return nil
}

// DeleteSavedQuery removes a saved query.
func (r *Repository) DeleteSavedQuery(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM ai_saved_queries WHERE id = $1::uuid`, id)
	if err != nil {
		return fmt.Errorf("delete saved query: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete saved query affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("saved query %s: %w", id, ErrSavedQueryNotFound)
	}
	return nil
}

// TouchSavedQueryVerified records a successful governed run of a runnable saved query.
func (r *Repository) TouchSavedQueryVerified(ctx context.Context, id string) error {
	if _, err := r.db.ExecContext(ctx, `UPDATE ai_saved_queries SET last_verified_at = now() WHERE id = $1::uuid`, id); err != nil {
		return fmt.Errorf("touch saved query verified: %w", err)
	}
	return nil
}

// DatasourceForSavedQuery resolves a saved-query id to its owning datasource for
// access-control middleware.
func (r *Repository) DatasourceForSavedQuery(ctx context.Context, id string) (string, error) {
	var datasourceID string
	err := r.db.QueryRowContext(ctx, `SELECT datasource_id::text FROM ai_saved_queries WHERE id = $1::uuid`, id).Scan(&datasourceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("saved query %s: %w", id, ErrSavedQueryNotFound)
		}
		return "", fmt.Errorf("datasource for saved query: %w", err)
	}
	return datasourceID, nil
}

// ListActiveSavedQueryExamples returns recent active embedding-bearing example
// rows for few-shot recall ranking. It is the ai_saved_queries equivalent of the
// legacy ListActiveConfirmedQueries and returns the same ConfirmedQueryRow shape
// so the recall ranker is unchanged.
func (r *Repository) ListActiveSavedQueryExamples(ctx context.Context, datasourceID, modelID, semanticModelHash string, limit int) ([]ConfirmedQueryRow, error) {
	if limit <= 0 {
		limit = ConfirmedQueriesCandidatePool
	}
	q := `
		SELECT id::text, datasource_id::text, COALESCE(model_id::text, ''), created_by,
			question_hash, question, sql_query, semantic_model_hash, question_embedding, is_active
		FROM ai_saved_queries
		WHERE datasource_id = $1::uuid
		  AND is_active = true
		  AND question_embedding IS NOT NULL
		  AND semantic_model_hash = $2
		  AND ($3::uuid IS NULL OR model_id = $3::uuid OR model_id IS NULL)
		ORDER BY created_at DESC
		LIMIT $4
	`
	var modelArg any
	if modelID != "" {
		modelArg = modelID
	}
	return platformdb.QuerySliceErr(ctx, r.db, "list active saved query examples", q,
		[]any{datasourceID, semanticModelHash, modelArg, limit},
		scanConfirmedQueryRow)
}

func scanSavedQueryRow(s platformdb.Scanner) (SavedQueryRow, error) {
	var (
		row          SavedQueryRow
		tags         pgarray.StringArray
		embeddingRaw []byte
	)
	if err := s.Scan(
		&row.ID, &row.DatasourceID, &row.ModelID, &row.Name, &row.Description, &row.Question,
		&row.QuestionHash, &row.SQLQuery, &row.LogicalQuery, &row.Parameters, &embeddingRaw, &row.SemanticModelHash,
		&tags, &row.Source, &row.Runnable, &row.IsActive, &row.CreatedBy, &row.Version,
		&row.LastVerifiedAt, &row.CreatedAt, &row.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return row, err
		}
		return row, fmt.Errorf("scan saved query: %w", err)
	}
	vec, err := decodeEmbedding(embeddingRaw)
	if err != nil {
		return row, err
	}
	row.QuestionEmbedding = vec
	row.Tags = []string(tags)
	return row, nil
}
