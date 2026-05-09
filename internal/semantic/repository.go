package semantic

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
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
	query := `
		INSERT INTO semantic_models (id, datasource_id, name, label, description, base_schema, base_table, synonyms, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	return r.db.QueryRowContext(ctx, query, m.ID, m.DatasourceID, m.Name, m.Label, m.Description, m.BaseSchema, m.BaseTable, m.Synonyms, m.IsActive).Err()
}

// GetModel retrieves a model by ID.
func (r *Repository) GetModel(ctx context.Context, id string) (*SemanticModel, error) {
	query := `SELECT id, datasource_id, name, label, description, base_schema, base_table, synonyms, is_active, created_by, created_at, updated_at FROM semantic_models WHERE id = $1`
	row := r.db.QueryRowContext(ctx, query, id)
	return r.scanModel(row)
}

// GetModelByName retrieves a model by datasource ID and name.
func (r *Repository) GetModelByName(ctx context.Context, datasourceID, name string) (*SemanticModel, error) {
	query := `SELECT id, datasource_id, name, label, description, base_schema, base_table, synonyms, is_active, created_by, created_at, updated_at FROM semantic_models WHERE datasource_id = $1 AND name = $2`
	row := r.db.QueryRowContext(ctx, query, datasourceID, name)
	return r.scanModel(row)
}

// ListModels returns all models, optionally filtered by datasource.
func (r *Repository) ListModels(ctx context.Context, datasourceID string) ([]SemanticModel, error) {
	query := `SELECT id, datasource_id, name, label, description, base_schema, base_table, synonyms, is_active, created_by, created_at, updated_at FROM semantic_models`
	var args []any
	if datasourceID != "" {
		query += " WHERE datasource_id = $1"
		args = append(args, datasourceID)
	}
	query += " ORDER BY created_at DESC"

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var models []SemanticModel
	for rows.Next() {
		m, err := r.scanModel(rows)
		if err != nil {
			return nil, err
		}
		models = append(models, *m)
	}
	return models, rows.Err()
}

// UpdateModel updates an existing semantic model.
func (r *Repository) UpdateModel(ctx context.Context, m *SemanticModel) error {
	query := `
		UPDATE semantic_models
		SET name = $2, label = $3, description = $4, base_schema = $5, base_table = $6, synonyms = $7, is_active = $8, updated_at = now()
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query, m.ID, m.Name, m.Label, m.Description, m.BaseSchema, m.BaseTable, m.Synonyms, m.IsActive)
	return err
}

// DeleteModel removes a semantic model.
func (r *Repository) DeleteModel(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM semantic_models WHERE id = $1`, id)
	return err
}

// Dimension operations

// CreateDimension inserts a new dimension.
func (r *Repository) CreateDimension(ctx context.Context, d *Dimension) error {
	query := `
		INSERT INTO semantic_dimensions (id, model_id, name, label, column_ref, type, synonyms, description, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`
	return r.db.QueryRowContext(ctx, query, d.ID, d.ModelID, d.Name, d.Label, d.ColumnRef, d.Type, d.Synonyms, d.Description, d.IsActive).Err()
}

// GetDimensions returns all active dimensions for a model.
func (r *Repository) GetDimensions(ctx context.Context, modelID string) ([]Dimension, error) {
	query := `SELECT id, model_id, name, label, column_ref, type, synonyms, description, is_active, created_at FROM semantic_dimensions WHERE model_id = $1 AND is_active = true ORDER BY name`
	rows, err := r.db.QueryContext(ctx, query, modelID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var dims []Dimension
	for rows.Next() {
		var d Dimension
		if err := rows.Scan(&d.ID, &d.ModelID, &d.Name, &d.Label, &d.ColumnRef, &d.Type, &d.Synonyms, &d.Description, &d.IsActive, &d.CreatedAt); err != nil {
			return nil, err
		}
		dims = append(dims, d)
	}
	return dims, rows.Err()
}

// Metric operations

// CreateMetric inserts a new metric.
func (r *Repository) CreateMetric(ctx context.Context, m *Metric) error {
	query := `
		INSERT INTO semantic_metrics (id, model_id, name, label, expression, aggregation, format, synonyms, description, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	return r.db.QueryRowContext(ctx, query, m.ID, m.ModelID, m.Name, m.Label, m.Expression, m.Aggregation, m.Format, m.Synonyms, m.Description, m.IsActive).Err()
}

// GetMetrics returns all active metrics for a model.
func (r *Repository) GetMetrics(ctx context.Context, modelID string) ([]Metric, error) {
	query := `SELECT id, model_id, name, label, expression, aggregation, format, synonyms, description, is_active, created_at FROM semantic_metrics WHERE model_id = $1 AND is_active = true ORDER BY name`
	rows, err := r.db.QueryContext(ctx, query, modelID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var metrics []Metric
	for rows.Next() {
		var m Metric
		if err := rows.Scan(&m.ID, &m.ModelID, &m.Name, &m.Label, &m.Expression, &m.Aggregation, &m.Format, &m.Synonyms, &m.Description, &m.IsActive, &m.CreatedAt); err != nil {
			return nil, err
		}
		metrics = append(metrics, m)
	}
	return metrics, rows.Err()
}

// Join operations

// CreateJoin inserts a new join.
func (r *Repository) CreateJoin(ctx context.Context, j *Join) error {
	query := `
		INSERT INTO semantic_joins (id, model_id, name, from_table, from_column, to_table, to_column, join_type, relationship, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	return r.db.QueryRowContext(ctx, query, j.ID, j.ModelID, j.Name, j.FromTable, j.FromColumn, j.ToTable, j.ToColumn, j.JoinType, j.Relationship, j.IsActive).Err()
}

// GetJoins returns all active joins for a model.
func (r *Repository) GetJoins(ctx context.Context, modelID string) ([]Join, error) {
	query := `SELECT id, model_id, name, from_table, from_column, to_table, to_column, join_type, relationship, is_active, created_at FROM semantic_joins WHERE model_id = $1 AND is_active = true ORDER BY name`
	rows, err := r.db.QueryContext(ctx, query, modelID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var joins []Join
	for rows.Next() {
		var j Join
		if err := rows.Scan(&j.ID, &j.ModelID, &j.Name, &j.FromTable, &j.FromColumn, &j.ToTable, &j.ToColumn, &j.JoinType, &j.Relationship, &j.IsActive, &j.CreatedAt); err != nil {
			return nil, err
		}
		joins = append(joins, j)
	}
	return joins, rows.Err()
}

// GetFullModel retrieves a model with its dimensions, metrics, and joins.
func (r *Repository) GetFullModel(ctx context.Context, id string) (*SemanticModel, error) {
	model, err := r.GetModel(ctx, id)
	if err != nil {
		return nil, err
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

func (r *Repository) scanModel(s scanner) (*SemanticModel, error) {
	m := &SemanticModel{}
	err := s.Scan(&m.ID, &m.DatasourceID, &m.Name, &m.Label, &m.Description, &m.BaseSchema, &m.BaseTable, &nullStringArray{s: &m.Synonyms}, &m.IsActive, &m.CreatedBy, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("scan model: %w", err)
	}
	return m, nil
}

type scanner interface {
	Scan(dest ...any) error
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
