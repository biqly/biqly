package semantic

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sync"

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
	ensureModelDefaults(m)
	query := `
		INSERT INTO semantic_models (id, datasource_id, name, label, description, base_schema, base_table, synonyms, excluded_schemas, is_active, status, version)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, 0)
	`
	if err := r.db.QueryRowContext(ctx, query, m.ID, m.DatasourceID, m.Name, m.Label, m.Description, m.BaseSchema, m.BaseTable, m.Synonyms, m.ExcludedSchemas, m.IsActive, m.Status).Err(); err != nil {
		return fmt.Errorf("create model: %w", err)
	}
	return nil
}

// GetModel retrieves a model by ID.
func (r *Repository) GetModel(ctx context.Context, id string) (*SemanticModel, error) {
	query := modelSelectSQL() + ` WHERE id = $1::uuid` //nolint:gosec // nosemgrep: go.lang.security.audit.database.string-formatted-query.string-formatted-query
	row := r.db.QueryRowContext(ctx, query, id)
	return r.scanModel(row)
}

// GetModelByName retrieves a model by datasource ID and name.
func (r *Repository) GetModelByName(ctx context.Context, datasourceID, name string) (*SemanticModel, error) {
	query := modelSelectSQL() + ` WHERE datasource_id = $1::uuid AND name = $2` //nolint:gosec // nosemgrep: go.lang.security.audit.database.string-formatted-query.string-formatted-query
	row := r.db.QueryRowContext(ctx, query, datasourceID, name)
	return r.scanModel(row)
}

// ListModels returns all models, optionally filtered by datasource.
func (r *Repository) ListModels(ctx context.Context, datasourceID string) ([]SemanticModel, error) {
	query := modelSelectSQL()
	var args []any
	if datasourceID != "" {
		query += " WHERE datasource_id = $1::uuid"
		args = append(args, datasourceID)
	}
	query += " ORDER BY created_at DESC"

	ptrs, err := platformdb.QuerySliceErr(ctx, r.db, "list models", query, args, r.scanModel)
	if err != nil {
		return nil, err
	}
	models := make([]SemanticModel, len(ptrs))
	for i, m := range ptrs {
		models[i] = *m
	}
	return models, nil
}

// ensureModelDefaults zeroes-out optional fields that the database layer
// requires to be non-nil (notably array columns that map to NOT NULL JSONB).
func ensureModelDefaults(m *SemanticModel) {
	if m.ExcludedSchemas == nil {
		m.ExcludedSchemas = []string{}
	}
}

// UpdateModel updates an existing semantic model.
func (r *Repository) UpdateModel(ctx context.Context, m *SemanticModel) error {
	ensureModelDefaults(m)
	query := `
		UPDATE semantic_models
		SET name = $2, label = $3, description = $4, base_schema = $5, base_table = $6, synonyms = $7, excluded_schemas = $8, is_active = $9,
		    status = 'draft', draft_updated_at = now(), updated_at = now()
		WHERE id = $1::uuid
	`
	_, err := r.db.ExecContext(ctx, query, m.ID, m.Name, m.Label, m.Description, m.BaseSchema, m.BaseTable, m.Synonyms, m.ExcludedSchemas, m.IsActive)
	if err != nil {
		return fmt.Errorf("update model: %w", err)
	}
	return nil
}

// DeleteModel removes a semantic model.
func (r *Repository) DeleteModel(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM semantic_models WHERE id = $1::uuid`, id)
	if err != nil {
		return fmt.Errorf("delete model: %w", err)
	}
	return nil
}

// BulkInsertModelChildren inserts many dimensions, metrics, and joins for a
// model inside a single transaction with prepared statements. Used when
// generating a model from metadata to avoid one round-trip per row.
func (r *Repository) BulkInsertModelChildren(ctx context.Context, modelID string, dims []Dimension, mets []Metric, joins []Join) error {
	if len(dims) == 0 && len(mets) == 0 && len(joins) == 0 {
		return nil
	}
	return platformdb.RunInTx(ctx, r.db, func(tx *sql.Tx) error {
		if len(dims) > 0 {
			stmt, err := tx.PrepareContext(ctx, `INSERT INTO semantic_dimensions (id, model_id, name, label, column_ref, type, time_grain, synonyms, description, is_active) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`)
			if err != nil {
				return fmt.Errorf("prepare dimensions: %w", err)
			}
			for i := range dims {
				d := &dims[i]
				if _, err := stmt.ExecContext(ctx, d.ID, d.ModelID, d.Name, d.Label, d.ColumnRef, d.Type, platformdb.NullIfEmpty(d.TimeGrain), d.Synonyms, d.Description, d.IsActive); err != nil {
					_ = stmt.Close()
					return fmt.Errorf("insert dimension %q: %w", d.Name, err)
				}
			}
			_ = stmt.Close()
		}
		if len(mets) > 0 {
			stmt, err := tx.PrepareContext(ctx, `INSERT INTO semantic_metrics (id, model_id, name, label, expression, aggregation, format, synonyms, description, is_active) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`)
			if err != nil {
				return fmt.Errorf("prepare metrics: %w", err)
			}
			for i := range mets {
				m := &mets[i]
				if _, err := stmt.ExecContext(ctx, m.ID, m.ModelID, m.Name, m.Label, m.Expression, m.Aggregation, m.Format, m.Synonyms, m.Description, m.IsActive); err != nil {
					_ = stmt.Close()
					return fmt.Errorf("insert metric %q: %w", m.Name, err)
				}
			}
			_ = stmt.Close()
		}
		if len(joins) > 0 {
			stmt, err := tx.PrepareContext(ctx, `INSERT INTO semantic_joins (id, model_id, name, from_schema, from_table, from_column, to_schema, to_table, to_column, join_type, relationship, is_active) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`)
			if err != nil {
				return fmt.Errorf("prepare joins: %w", err)
			}
			for i := range joins {
				j := &joins[i]
				if _, err := stmt.ExecContext(ctx, j.ID, j.ModelID, j.Name, j.FromSchema, j.FromTable, j.FromColumn, j.ToSchema, j.ToTable, j.ToColumn, j.JoinType, j.Relationship, j.IsActive); err != nil {
					_ = stmt.Close()
					return fmt.Errorf("insert join %q: %w", j.Name, err)
				}
			}
			_ = stmt.Close()
		}
		if _, err := tx.ExecContext(ctx, `UPDATE semantic_models SET status = 'draft', draft_updated_at = now(), updated_at = now() WHERE id = $1::uuid`, modelID); err != nil {
			return fmt.Errorf("mark model draft: %w", err)
		}
		return nil
	})
}

// Dimension operations

// CreateDimension inserts a new dimension.
func (r *Repository) CreateDimension(ctx context.Context, d *Dimension) error {
	query := `
		INSERT INTO semantic_dimensions (id, model_id, name, label, column_ref, type, time_grain, synonyms, description, is_active)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	if err := r.db.QueryRowContext(ctx, query, d.ID, d.ModelID, d.Name, d.Label, d.ColumnRef, d.Type, platformdb.NullIfEmpty(d.TimeGrain), d.Synonyms, d.Description, d.IsActive).Err(); err != nil {
		return fmt.Errorf("create dimension: %w", err)
	}
	return r.MarkModelDraft(ctx, d.ModelID)
}

// GetDimensions returns all active dimensions for a model.
func (r *Repository) GetDimensions(ctx context.Context, modelID string) ([]Dimension, error) {
	query := `SELECT id::text, model_id::text, name, label, column_ref, type, time_grain, synonyms, description, is_active, created_at FROM semantic_dimensions WHERE model_id = $1::uuid AND is_active = true ORDER BY name`
	dims, err := platformdb.QuerySliceErr(ctx, r.db, "get dimensions", query, []any{modelID}, scanDimension)
	if err != nil {
		return nil, err
	}
	if err := r.attachEnumMappings(ctx, modelID, dims); err != nil {
		return nil, err
	}
	return dims, nil
}

// ListAllDimensions returns every dimension (active and inactive) for a model
// so the modeling UI can show soft-deleted items in a "Pasif" section.
func (r *Repository) ListAllDimensions(ctx context.Context, modelID string) ([]Dimension, error) {
	query := `SELECT id::text, model_id::text, name, label, column_ref, type, time_grain, synonyms, description, is_active, created_at FROM semantic_dimensions WHERE model_id = $1::uuid ORDER BY is_active DESC, name`
	dims, err := platformdb.QuerySliceErr(ctx, r.db, "list all dimensions", query, []any{modelID}, scanDimension)
	if err != nil {
		return nil, err
	}
	if err := r.attachEnumMappings(ctx, modelID, dims); err != nil {
		return nil, err
	}
	return dims, nil
}

// Enum mapping operations

// GetEnumMappings returns the enum value mappings for a single dimension,
// ordered by sort_order then raw_value.
func (r *Repository) GetEnumMappings(ctx context.Context, dimensionID string) ([]EnumMapping, error) {
	query := `SELECT id::text, dimension_id::text, raw_value, label, description, sort_order FROM enum_mappings WHERE dimension_id = $1::uuid ORDER BY sort_order, raw_value`
	return platformdb.QuerySliceErr(ctx, r.db, "get enum mappings", query, []any{dimensionID}, scanEnumMapping)
}

// ReplaceEnumMappings replaces all enum mappings for a dimension with the
// supplied set in a single transaction, then marks the model as draft.
func (r *Repository) ReplaceEnumMappings(ctx context.Context, modelID, dimensionID string, mappings []EnumMapping) error {
	if err := platformdb.RunInTx(ctx, r.db, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, `DELETE FROM enum_mappings WHERE dimension_id = $1::uuid`, dimensionID); err != nil {
			return fmt.Errorf("clear enum mappings: %w", err)
		}
		if len(mappings) == 0 {
			return nil
		}
		stmt, err := tx.PrepareContext(ctx, `INSERT INTO enum_mappings (dimension_id, raw_value, label, description, sort_order) VALUES ($1::uuid, $2, $3, $4, $5)`)
		if err != nil {
			return fmt.Errorf("prepare enum mappings: %w", err)
		}
		defer func() { _ = stmt.Close() }()
		for i := range mappings {
			m := &mappings[i]
			if _, err := stmt.ExecContext(ctx, dimensionID, m.RawValue, m.Label, platformdb.NullIfEmptyPtr(m.Description), m.SortOrder); err != nil {
				return fmt.Errorf("insert enum mapping %q: %w", m.RawValue, err)
			}
		}
		return nil
	}); err != nil {
		return err
	}
	return r.MarkModelDraft(ctx, modelID)
}

// attachEnumMappings loads enum mappings for every dimension in the model in a
// single query and assigns them to their owning dimension. Avoids one round
// trip per dimension when building the full model.
func (r *Repository) attachEnumMappings(ctx context.Context, modelID string, dims []Dimension) error {
	if len(dims) == 0 {
		return nil
	}
	query := `SELECT e.id::text, e.dimension_id::text, e.raw_value, e.label, e.description, e.sort_order
		FROM enum_mappings e
		JOIN semantic_dimensions d ON d.id = e.dimension_id
		WHERE d.model_id = $1::uuid
		ORDER BY e.sort_order, e.raw_value`
	rows, err := platformdb.QuerySliceErr(ctx, r.db, "list enum mappings for model", query, []any{modelID}, scanEnumMapping)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}
	byDim := make(map[string][]EnumMapping, len(dims))
	for _, m := range rows {
		byDim[m.DimensionID] = append(byDim[m.DimensionID], m)
	}
	for i := range dims {
		if vals, ok := byDim[dims[i].ID]; ok {
			dims[i].EnumValues = vals
		}
	}
	return nil
}

// DeleteDimension soft-deletes a dimension by setting is_active = false.
func (r *Repository) DeleteDimension(ctx context.Context, modelID, dimensionID string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE semantic_dimensions SET is_active = false WHERE id = $1::uuid AND model_id = $2::uuid`, dimensionID, modelID)
	if err != nil {
		return fmt.Errorf("delete dimension: %w", err)
	}
	return r.MarkModelDraft(ctx, modelID)
}

// UpdateDimension updates an existing dimension.
func (r *Repository) UpdateDimension(ctx context.Context, d *Dimension) error {
	query := `UPDATE semantic_dimensions SET name = $2, label = $3, column_ref = $4, type = $5, time_grain = $6, synonyms = $7, description = $8, is_active = $9 WHERE id = $1::uuid AND model_id = $10::uuid`
	_, err := r.db.ExecContext(ctx, query, d.ID, d.Name, d.Label, d.ColumnRef, d.Type, platformdb.NullIfEmpty(d.TimeGrain), d.Synonyms, d.Description, d.IsActive, d.ModelID)
	if err != nil {
		return fmt.Errorf("update dimension: %w", err)
	}
	return r.MarkModelDraft(ctx, d.ModelID)
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
	query := `SELECT id::text, model_id::text, name, label, expression, aggregation, format, synonyms, description, is_active, created_at FROM semantic_metrics WHERE model_id = $1::uuid AND is_active = true ORDER BY name`
	return platformdb.QuerySliceErr(ctx, r.db, "get metrics", query, []any{modelID}, scanMetric)
}

// ListAllMetrics returns every metric (active and inactive) for a model.
func (r *Repository) ListAllMetrics(ctx context.Context, modelID string) ([]Metric, error) {
	query := `SELECT id::text, model_id::text, name, label, expression, aggregation, format, synonyms, description, is_active, created_at FROM semantic_metrics WHERE model_id = $1::uuid ORDER BY is_active DESC, name`
	return platformdb.QuerySliceErr(ctx, r.db, "list all metrics", query, []any{modelID}, scanMetric)
}

// DeleteMetric soft-deletes a metric by setting is_active = false.
func (r *Repository) DeleteMetric(ctx context.Context, modelID, metricID string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE semantic_metrics SET is_active = false WHERE id = $1::uuid AND model_id = $2::uuid`, metricID, modelID)
	if err != nil {
		return fmt.Errorf("delete metric: %w", err)
	}
	return r.MarkModelDraft(ctx, modelID)
}

// UpdateMetric updates an existing metric.
func (r *Repository) UpdateMetric(ctx context.Context, m *Metric) error {
	query := `UPDATE semantic_metrics SET name = $2, label = $3, expression = $4, aggregation = $5, format = $6, synonyms = $7, description = $8, is_active = $9 WHERE id = $1::uuid AND model_id = $10::uuid`
	_, err := r.db.ExecContext(ctx, query, m.ID, m.Name, m.Label, m.Expression, m.Aggregation, m.Format, m.Synonyms, m.Description, m.IsActive, m.ModelID)
	if err != nil {
		return fmt.Errorf("update metric: %w", err)
	}
	return r.MarkModelDraft(ctx, m.ModelID)
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
	query := `SELECT id::text, model_id::text, name, from_schema, from_table, from_column, to_schema, to_table, to_column, join_type, relationship, is_active, created_at FROM semantic_joins WHERE model_id = $1::uuid AND is_active = true ORDER BY name`
	return platformdb.QuerySliceErr(ctx, r.db, "get joins", query, []any{modelID}, scanJoin)
}

// ListAllJoins returns every join (active and inactive) for a model.
func (r *Repository) ListAllJoins(ctx context.Context, modelID string) ([]Join, error) {
	query := `SELECT id::text, model_id::text, name, from_schema, from_table, from_column, to_schema, to_table, to_column, join_type, relationship, is_active, created_at FROM semantic_joins WHERE model_id = $1::uuid ORDER BY is_active DESC, name`
	return platformdb.QuerySliceErr(ctx, r.db, "list all joins", query, []any{modelID}, scanJoin)
}

// DeleteJoin soft-deletes a join by setting is_active = false.
func (r *Repository) DeleteJoin(ctx context.Context, modelID, joinID string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE semantic_joins SET is_active = false WHERE id = $1::uuid AND model_id = $2::uuid`, joinID, modelID)
	if err != nil {
		return fmt.Errorf("delete join: %w", err)
	}
	return r.MarkModelDraft(ctx, modelID)
}

// UpdateJoin updates an existing join.
func (r *Repository) UpdateJoin(ctx context.Context, j *Join) error {
	query := `UPDATE semantic_joins SET name = $2, from_schema = $3, from_table = $4, from_column = $5, to_schema = $6, to_table = $7, to_column = $8, join_type = $9, relationship = $10, is_active = $11 WHERE id = $1::uuid AND model_id = $12::uuid`
	_, err := r.db.ExecContext(ctx, query, j.ID, j.Name, j.FromSchema, j.FromTable, j.FromColumn, j.ToSchema, j.ToTable, j.ToColumn, j.JoinType, j.Relationship, j.IsActive, j.ModelID)
	if err != nil {
		return fmt.Errorf("update join: %w", err)
	}
	return r.MarkModelDraft(ctx, j.ModelID)
}

// GetFullModel retrieves a model with its dimensions, metrics, and joins.
//
// Dimensions, metrics, and joins are fetched concurrently because they are
// independent queries against separate tables; serializing them dominated
// /api/ai/query latency on cold cache. The model header is still fetched
// first so we can short-circuit on a missing id without launching three
// extra queries.
func (r *Repository) GetFullModel(ctx context.Context, id string) (*SemanticModel, error) {
	model, err := r.GetModel(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get full model: %w", err)
	}

	var (
		dims    []Dimension
		mets    []Metric
		joins   []Join
		dimErr  error
		metErr  error
		joinErr error
	)
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		dims, dimErr = r.GetDimensions(ctx, id)
	}()
	go func() {
		defer wg.Done()
		mets, metErr = r.GetMetrics(ctx, id)
	}()
	go func() {
		defer wg.Done()
		joins, joinErr = r.GetJoins(ctx, id)
	}()
	wg.Wait()

	switch {
	case dimErr != nil:
		return nil, fmt.Errorf("get dimensions: %w", dimErr)
	case metErr != nil:
		return nil, fmt.Errorf("get metrics: %w", metErr)
	case joinErr != nil:
		return nil, fmt.Errorf("get joins: %w", joinErr)
	}

	model.Dimensions = dims
	model.Metrics = mets
	model.Joins = joins
	return model, nil
}

func (r *Repository) GetPublishedFullModel(ctx context.Context, id string) (*SemanticModel, error) {
	model, err := r.GetModel(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get published full model: %w", err)
	}
	published, err := r.latestPublishedSnapshot(ctx, model.ID)
	if err == nil {
		if len(published.Dimensions) > 0 {
			return published, nil
		}
	} else if !errors.Is(err, sql.ErrNoRows) {
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

	payload, err := json.Marshal(model)
	if err != nil {
		return nil, fmt.Errorf("publish model marshal context: %w", err)
	}
	validationPayload, err := json.Marshal(validation)
	if err != nil {
		return nil, fmt.Errorf("publish model marshal validation: %w", err)
	}
	if err := platformdb.RunInTx(ctx, r.db, func(tx *sql.Tx) error {
		return r.writePublishedVersionTx(ctx, tx, model.ID, nextVersion, payload, validationPayload, publishedBy)
	}); err != nil {
		return nil, fmt.Errorf("publish model: %w", err)
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

	if err := platformdb.RunInTx(ctx, r.db, func(tx *sql.Tx) error {
		return r.writePublishedVersionTx(ctx, tx, current.ID, nextVersion, payload, validationPayload, publishedBy)
	}); err != nil {
		return nil, fmt.Errorf("rollback model: %w", err)
	}
	published, err := r.GetPublishedFullModel(ctx, current.ID)
	if err != nil {
		return nil, fmt.Errorf("rollback model reload: %w", err)
	}
	return &PublishResult{Model: published, Validation: validation, Version: nextVersion}, nil
}

func (r *Repository) writePublishedVersionTx(
	ctx context.Context,
	tx *sql.Tx,
	modelID string,
	version int,
	contextPayload, validationPayload []byte,
	publishedBy string,
) error {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO semantic_context_snapshots (model_id, version, context, validation_result, created_by)
		VALUES ($1, $2, $3::jsonb, $4::jsonb, NULLIF($5, ''))
	`, modelID, version, contextPayload, validationPayload, publishedBy); err != nil {
		return fmt.Errorf("insert snapshot: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE semantic_models
		SET status = 'published',
		    version = $2,
		    published_at = now(),
		    published_by = NULLIF($3, ''),
		    updated_at = now()
		WHERE id = $1::uuid
	`, modelID, version, publishedBy); err != nil {
		return fmt.Errorf("update model: %w", err)
	}
	return nil
}

func (r *Repository) MarkModelDraft(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE semantic_models
		SET status = 'draft',
		    draft_updated_at = now(),
		    updated_at = now()
		WHERE id = $1::uuid
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
		WHERE model_id = $1::uuid
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
		WHERE model_id = $1::uuid AND version = $2
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
	return `SELECT id::text, datasource_id::text, name, label, description, base_schema, base_table, synonyms, excluded_schemas, is_active,
		status, version, published_at, published_by, draft_updated_at, created_by, created_at, updated_at
		FROM semantic_models`
}

func scanDimension(s platformdb.Scanner) (Dimension, error) {
	var d Dimension
	var timeGrain sql.NullString
	if err := s.Scan(&d.ID, &d.ModelID, &d.Name, &d.Label, &d.ColumnRef, &d.Type, &timeGrain, &platformdb.NullStringArray{S: &d.Synonyms}, &d.Description, &d.IsActive, &d.CreatedAt); err != nil {
		return d, fmt.Errorf("scan dimension: %w", err)
	}
	if timeGrain.Valid {
		d.TimeGrain = timeGrain.String
	}
	return d, nil
}

func scanEnumMapping(s platformdb.Scanner) (EnumMapping, error) {
	var e EnumMapping
	if err := s.Scan(&e.ID, &e.DimensionID, &e.RawValue, &e.Label, &e.Description, &e.SortOrder); err != nil {
		return e, fmt.Errorf("scan enum mapping: %w", err)
	}
	return e, nil
}

func scanMetric(s platformdb.Scanner) (Metric, error) {
	var m Metric
	if err := s.Scan(&m.ID, &m.ModelID, &m.Name, &m.Label, &m.Expression, &m.Aggregation, &m.Format, &platformdb.NullStringArray{S: &m.Synonyms}, &m.Description, &m.IsActive, &m.CreatedAt); err != nil {
		return m, fmt.Errorf("scan metric: %w", err)
	}
	return m, nil
}

func scanJoin(s platformdb.Scanner) (Join, error) {
	var j Join
	if err := s.Scan(&j.ID, &j.ModelID, &j.Name, &j.FromSchema, &j.FromTable, &j.FromColumn, &j.ToSchema, &j.ToTable, &j.ToColumn, &j.JoinType, &j.Relationship, &j.IsActive, &j.CreatedAt); err != nil {
		return j, fmt.Errorf("scan join: %w", err)
	}
	return j, nil
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
		&platformdb.NullStringArray{S: &m.Synonyms},
		&platformdb.NullStringArray{S: &m.ExcludedSchemas},
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
