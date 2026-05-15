package semantic

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	platformdb "github.com/biqly/biqly/internal/platform/db"
)

// Repository handles semantic layer database operations.
type Repository struct {
	db *sql.DB
}

// NewRepository creates a new semantic repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

// Model operations

// CreateModel inserts a new semantic model.
func (r *Repository) CreateModel(ctx context.Context, m *SemanticModel) error {
	if m.Status == "" {
		m.Status = ModelStatusDraft
	}
	query := `
		INSERT INTO semantic_models (id, datasource_id, name, label, description, base_schema, base_table, synonyms, is_active, status, version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 0)
	`
	if err := r.db.QueryRowContext(ctx, query, m.ID, m.DatasourceID, m.Name, m.Label, m.Description, m.BaseSchema, m.BaseTable, m.Synonyms, m.IsActive, m.Status).Err(); err != nil {
		return fmt.Errorf("create model: %w", err)
	}
	return nil
}

// GetModel retrieves a model by ID.
func (r *Repository) GetModel(ctx context.Context, id string) (*SemanticModel, error) {
	query := modelSelectSQL() + ` WHERE id::text = $1`
	row := r.db.QueryRowContext(ctx, query, id)
	return r.scanModel(row)
}

// GetModelByName retrieves a model by datasource ID and name.
func (r *Repository) GetModelByName(ctx context.Context, datasourceID, name string) (*SemanticModel, error) {
	query := modelSelectSQL() + ` WHERE datasource_id::text = $1 AND name = $2`
	row := r.db.QueryRowContext(ctx, query, datasourceID, name)
	return r.scanModel(row)
}

// ListModels returns all models, optionally filtered by datasource.
func (r *Repository) ListModels(ctx context.Context, datasourceID string) ([]SemanticModel, error) {
	query := modelSelectSQL()
	var args []any
	if datasourceID != "" {
		query += " WHERE datasource_id::text = $1"
		args = append(args, datasourceID)
	}
	query += " ORDER BY created_at DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}
	defer func() { _ = rows.Close() }()

	models := make([]SemanticModel, 0)
	for rows.Next() {
		m, err := r.scanModel(rows)
		if err != nil {
			return nil, fmt.Errorf("list models: %w", err)
		}
		models = append(models, *m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list models: %w", err)
	}
	return models, nil
}

// UpdateModel updates an existing semantic model.
func (r *Repository) UpdateModel(ctx context.Context, m *SemanticModel) error {
	query := `
		UPDATE semantic_models
		SET name = $2, label = $3, description = $4, base_schema = $5, base_table = $6, synonyms = $7, is_active = $8,
		    status = 'draft', draft_updated_at = now(), updated_at = now()
		WHERE id::text = $1
	`
	_, err := r.db.ExecContext(ctx, query, m.ID, m.Name, m.Label, m.Description, m.BaseSchema, m.BaseTable, m.Synonyms, m.IsActive)
	if err != nil {
		return fmt.Errorf("update model: %w", err)
	}
	return nil
}

// DeleteModel removes a semantic model.
func (r *Repository) DeleteModel(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM semantic_models WHERE id::text = $1`, id)
	if err != nil {
		return fmt.Errorf("delete model: %w", err)
	}
	return nil
}

// Dimension operations

// CreateDimension inserts a new dimension.
func (r *Repository) CreateDimension(ctx context.Context, d *Dimension) error {
	query := `
		INSERT INTO semantic_dimensions (id, model_id, name, label, column_ref, type, synonyms, description, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	if err := r.db.QueryRowContext(ctx, query, d.ID, d.ModelID, d.Name, d.Label, d.ColumnRef, d.Type, d.Synonyms, d.Description, d.IsActive).Err(); err != nil {
		return fmt.Errorf("create dimension: %w", err)
	}
	return r.MarkModelDraft(ctx, d.ModelID)
}

// GetDimensions returns all active dimensions for a model.
func (r *Repository) GetDimensions(ctx context.Context, modelID string) ([]Dimension, error) {
	query := `SELECT id::text, model_id::text, name, label, column_ref, type, synonyms, description, is_active, created_at FROM semantic_dimensions WHERE model_id::text = $1 AND is_active = true ORDER BY name`
	rows, err := r.db.QueryContext(ctx, query, modelID)
	if err != nil {
		return nil, fmt.Errorf("get dimensions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	dims := make([]Dimension, 0)
	for rows.Next() {
		var d Dimension
		if err := rows.Scan(&d.ID, &d.ModelID, &d.Name, &d.Label, &d.ColumnRef, &d.Type, &nullStringArray{s: &d.Synonyms}, &d.Description, &d.IsActive, &d.CreatedAt); err != nil {
			return nil, fmt.Errorf("get dimensions scan: %w", err)
		}
		dims = append(dims, d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get dimensions: %w", err)
	}
	return dims, nil
}

// Metric operations

// CreateMetric inserts a new metric.
func (r *Repository) CreateMetric(ctx context.Context, m *Metric) error {
	query := `
		INSERT INTO semantic_metrics (id, model_id, name, label, expression, aggregation, format, synonyms, description, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	if err := r.db.QueryRowContext(ctx, query, m.ID, m.ModelID, m.Name, m.Label, m.Expression, m.Aggregation, m.Format, m.Synonyms, m.Description, m.IsActive).Err(); err != nil {
		return fmt.Errorf("create metric: %w", err)
	}
	return r.MarkModelDraft(ctx, m.ModelID)
}

// GetMetrics returns all active metrics for a model.
func (r *Repository) GetMetrics(ctx context.Context, modelID string) ([]Metric, error) {
	query := `SELECT id::text, model_id::text, name, label, expression, aggregation, format, synonyms, description, is_active, created_at FROM semantic_metrics WHERE model_id::text = $1 AND is_active = true ORDER BY name`
	rows, err := r.db.QueryContext(ctx, query, modelID)
	if err != nil {
		return nil, fmt.Errorf("get metrics: %w", err)
	}
	defer func() { _ = rows.Close() }()

	metrics := make([]Metric, 0)
	for rows.Next() {
		var m Metric
		if err := rows.Scan(&m.ID, &m.ModelID, &m.Name, &m.Label, &m.Expression, &m.Aggregation, &m.Format, &nullStringArray{s: &m.Synonyms}, &m.Description, &m.IsActive, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("get metrics scan: %w", err)
		}
		metrics = append(metrics, m)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get metrics: %w", err)
	}
	return metrics, nil
}

// Join operations

// CreateJoin inserts a new join.
func (r *Repository) CreateJoin(ctx context.Context, j *Join) error {
	query := `
		INSERT INTO semantic_joins (id, model_id, name, from_schema, from_table, from_column, to_schema, to_table, to_column, join_type, relationship, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	if err := r.db.QueryRowContext(ctx, query, j.ID, j.ModelID, j.Name, j.FromSchema, j.FromTable, j.FromColumn, j.ToSchema, j.ToTable, j.ToColumn, j.JoinType, j.Relationship, j.IsActive).Err(); err != nil {
		return fmt.Errorf("create join: %w", err)
	}
	return r.MarkModelDraft(ctx, j.ModelID)
}

// GetJoins returns all active joins for a model.
func (r *Repository) GetJoins(ctx context.Context, modelID string) ([]Join, error) {
	query := `SELECT id::text, model_id::text, name, from_schema, from_table, from_column, to_schema, to_table, to_column, join_type, relationship, is_active, created_at FROM semantic_joins WHERE model_id::text = $1 AND is_active = true ORDER BY name`
	rows, err := r.db.QueryContext(ctx, query, modelID)
	if err != nil {
		return nil, fmt.Errorf("get joins: %w", err)
	}
	defer func() { _ = rows.Close() }()

	joins := make([]Join, 0)
	for rows.Next() {
		var j Join
		if err := rows.Scan(&j.ID, &j.ModelID, &j.Name, &j.FromSchema, &j.FromTable, &j.FromColumn, &j.ToSchema, &j.ToTable, &j.ToColumn, &j.JoinType, &j.Relationship, &j.IsActive, &j.CreatedAt); err != nil {
			return nil, fmt.Errorf("get joins scan: %w", err)
		}
		joins = append(joins, j)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get joins: %w", err)
	}
	return joins, nil
}

// GetFullModel retrieves a model with its dimensions, metrics, and joins.
func (r *Repository) GetFullModel(ctx context.Context, id string) (*SemanticModel, error) {
	model, err := r.GetModel(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get full model: %w", err)
	}

	model.Dimensions, err = r.GetDimensions(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get dimensions: %w", err)
	}

	model.Metrics, err = r.GetMetrics(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get metrics: %w", err)
	}

	model.Joins, err = r.GetJoins(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get joins: %w", err)
	}

	return model, nil
}

func (r *Repository) GetPublishedFullModel(ctx context.Context, id string) (*SemanticModel, error) {
	model, err := r.GetModel(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get published full model: %w", err)
	}
	published, err := r.latestPublishedSnapshot(ctx, model.ID)
	if err == nil {
		return published, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("latest published snapshot: %w", err)
	}
	if model.Status == ModelStatusDraft {
		return nil, fmt.Errorf("semantic model %s has no published context", model.Name)
	}
	return r.GetFullModel(ctx, id)
}

func (r *Repository) GetPublishedModelByName(ctx context.Context, datasourceID, name string) (*SemanticModel, error) {
	model, err := r.GetModelByName(ctx, datasourceID, name)
	if err != nil {
		return nil, fmt.Errorf("get published model by name: %w", err)
	}
	return r.GetPublishedFullModel(ctx, model.ID)
}

type PublishResult struct {
	Model      *SemanticModel          `json:"model,omitempty"`
	Validation PublishValidationResult `json:"validation"`
	Version    int                     `json:"version,omitempty"`
}

func (r *Repository) ValidateModel(ctx context.Context, id string, catalog CatalogReader) (PublishValidationResult, error) {
	model, err := r.GetFullModel(ctx, id)
	if err != nil {
		return PublishValidationResult{}, fmt.Errorf("validate model: %w", err)
	}
	return ValidateContext(ctx, *model, catalog), nil
}

func (r *Repository) PublishModel(ctx context.Context, id, publishedBy string, catalog CatalogReader) (*PublishResult, error) {
	model, err := r.GetFullModel(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("publish model: %w", err)
	}
	validation := ValidateContext(ctx, *model, catalog)
	if !validation.Valid {
		return &PublishResult{Model: model, Validation: validation, Version: model.Version}, nil
	}
	nextVersion := model.Version + 1
	if nextVersion <= 0 {
		nextVersion = 1
	}
	model.Status = ModelStatusPublished
	model.Version = nextVersion
	if publishedBy != "" {
		model.PublishedBy = &publishedBy
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("publish model begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	payload, err := json.Marshal(model)
	if err != nil {
		return nil, fmt.Errorf("publish model marshal context: %w", err)
	}
	validationPayload, err := json.Marshal(validation)
	if err != nil {
		return nil, fmt.Errorf("publish model marshal validation: %w", err)
	}
	if _, errExec := tx.ExecContext(ctx, `
		INSERT INTO semantic_context_snapshots (model_id, version, context, validation_result, created_by)
		VALUES ($1, $2, $3::jsonb, $4::jsonb, NULLIF($5, ''))
	`, model.ID, nextVersion, payload, validationPayload, publishedBy); errExec != nil {
		return nil, fmt.Errorf("publish model insert snapshot: %w", errExec)
	}
	if _, errExec := tx.ExecContext(ctx, `
		UPDATE semantic_models
		SET status = 'published',
		    version = $2,
		    published_at = now(),
		    published_by = NULLIF($3, ''),
		    updated_at = now()
		WHERE id::text = $1
	`, model.ID, nextVersion, publishedBy); errExec != nil {
		return nil, fmt.Errorf("publish model update: %w", errExec)
	}
	if errCommit := tx.Commit(); errCommit != nil {
		return nil, fmt.Errorf("publish model commit: %w", errCommit)
	}
	published, err := r.GetPublishedFullModel(ctx, model.ID)
	if err != nil {
		return nil, fmt.Errorf("publish model reload: %w", err)
	}
	return &PublishResult{Model: published, Validation: validation, Version: nextVersion}, nil
}

func (r *Repository) RollbackModel(ctx context.Context, id string, targetVersion int, publishedBy string) (*PublishResult, error) {
	current, err := r.GetModel(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("rollback model: %w", err)
	}
	if targetVersion <= 0 {
		targetVersion = current.Version - 1
	}
	if targetVersion <= 0 {
		return nil, fmt.Errorf("no previous published context to roll back to")
	}
	target, err := r.snapshotByVersion(ctx, current.ID, targetVersion)
	if err != nil {
		return nil, fmt.Errorf("rollback model snapshot: %w", err)
	}
	nextVersion := current.Version + 1
	target.Version = nextVersion
	target.Status = ModelStatusPublished
	if publishedBy != "" {
		target.PublishedBy = &publishedBy
	}
	payload, err := json.Marshal(target)
	if err != nil {
		return nil, fmt.Errorf("rollback model marshal context: %w", err)
	}
	validation := PublishValidationResult{Valid: true, Warnings: []string{fmt.Sprintf("rolled back from version %d to version %d", current.Version, targetVersion)}}
	validationPayload, err := json.Marshal(validation)
	if err != nil {
		return nil, fmt.Errorf("rollback model marshal validation: %w", err)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("rollback model begin tx: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	if _, errExec := tx.ExecContext(ctx, `
		INSERT INTO semantic_context_snapshots (model_id, version, context, validation_result, created_by)
		VALUES ($1, $2, $3::jsonb, $4::jsonb, NULLIF($5, ''))
	`, current.ID, nextVersion, payload, validationPayload, publishedBy); errExec != nil {
		return nil, fmt.Errorf("rollback model insert snapshot: %w", errExec)
	}
	if _, errExec := tx.ExecContext(ctx, `
		UPDATE semantic_models
		SET status = 'published',
		    version = $2,
		    published_at = now(),
		    published_by = NULLIF($3, ''),
		    updated_at = now()
		WHERE id::text = $1
	`, current.ID, nextVersion, publishedBy); errExec != nil {
		return nil, fmt.Errorf("rollback model update: %w", errExec)
	}
	if errCommit := tx.Commit(); errCommit != nil {
		return nil, fmt.Errorf("rollback model commit: %w", errCommit)
	}
	published, err := r.GetPublishedFullModel(ctx, current.ID)
	if err != nil {
		return nil, fmt.Errorf("rollback model reload: %w", err)
	}
	return &PublishResult{Model: published, Validation: validation, Version: nextVersion}, nil
}

func (r *Repository) MarkModelDraft(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE semantic_models
		SET status = 'draft',
		    draft_updated_at = now(),
		    updated_at = now()
		WHERE id::text = $1
	`, id)
	if err != nil {
		return fmt.Errorf("mark model draft: %w", err)
	}
	return nil
}

func (r *Repository) latestPublishedSnapshot(ctx context.Context, modelID string) (*SemanticModel, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT context
		FROM semantic_context_snapshots
		WHERE model_id::text = $1
		ORDER BY version DESC
		LIMIT 1
	`, modelID)
	var raw []byte
	if err := row.Scan(&raw); err != nil {
		return nil, fmt.Errorf("latest published snapshot scan: %w", err)
	}
	return decodeModelSnapshot(raw)
}

func (r *Repository) snapshotByVersion(ctx context.Context, modelID string, version int) (*SemanticModel, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT context
		FROM semantic_context_snapshots
		WHERE model_id::text = $1 AND version = $2
	`, modelID, version)
	var raw []byte
	if err := row.Scan(&raw); err != nil {
		return nil, fmt.Errorf("snapshot by version scan: %w", err)
	}
	return decodeModelSnapshot(raw)
}

func decodeModelSnapshot(raw []byte) (*SemanticModel, error) {
	var model SemanticModel
	if err := json.Unmarshal(raw, &model); err != nil {
		return nil, fmt.Errorf("decode model snapshot: %w", err)
	}
	return &model, nil
}

func modelSelectSQL() string {
	return `SELECT id::text, datasource_id::text, name, label, description, base_schema, base_table, synonyms, is_active,
		status, version, published_at, published_by, draft_updated_at, created_by, created_at, updated_at
		FROM semantic_models`
}

func (r *Repository) scanModel(s platformdb.Scanner) (*SemanticModel, error) {
	m := &SemanticModel{}
	err := s.Scan(
		&m.ID,
		&m.DatasourceID,
		&m.Name,
		&m.Label,
		&m.Description,
		&m.BaseSchema,
		&m.BaseTable,
		&nullStringArray{s: &m.Synonyms},
		&m.IsActive,
		&m.Status,
		&m.Version,
		&m.PublishedAt,
		&m.PublishedBy,
		&m.DraftUpdatedAt,
		&m.CreatedBy,
		&m.CreatedAt,
		&m.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("scan model: %w", err)
	}
	return m, nil
}

// nullStringArray helps scan PostgreSQL arrays into Go slices
type nullStringArray struct {
	s *[]string
}

func (n *nullStringArray) Scan(src any) error {
	if src == nil {
		*n.s = []string{}
		return nil
	}
	switch v := src.(type) {
	case string:
		*n.s = parseStringArray(v)
	case []byte:
		*n.s = parseStringArray(string(v))
	default:
		*n.s = []string{}
	}
	return nil
}

func parseStringArray(s string) []string {
	// Simple PostgreSQL array parser for strings: {"a","b","c"}
	s = strings.TrimPrefix(s, "{")
	s = strings.TrimSuffix(s, "}")
	if s == "" {
		return []string{}
	}
	parts := strings.Split(s, ",")
	for i, p := range parts {
		parts[i] = strings.Trim(p, "\"")
	}
	return parts
}
