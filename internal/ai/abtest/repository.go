package abtest

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

type rowScanner interface {
	Scan(dest ...any) error
}

type rowsScanner interface {
	Next() bool
	Scan(dest ...any) error
	Err() error
	Close() error
}

type dbRunner interface {
	QueryRowContext(ctx context.Context, query string, args ...any) rowScanner
	QueryContext(ctx context.Context, query string, args ...any) (rowsScanner, error)
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
}

type sqlDBRunner struct {
	db *sql.DB
}

func (r sqlDBRunner) QueryRowContext(ctx context.Context, query string, args ...any) rowScanner {
	return r.db.QueryRowContext(ctx, query, args...)
}

func (r sqlDBRunner) QueryContext(ctx context.Context, query string, args ...any) (rowsScanner, error) {
	return r.db.QueryContext(ctx, query, args...) //nolint:rowserrcheck // callers check rows.Err() after iteration
}

func (r sqlDBRunner) ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return r.db.ExecContext(ctx, query, args...)
}

// Repository handles database operations for prompt A/B experiments.
type Repository struct {
	db dbRunner
}

// NewRepository creates a new prompt A/B experiment repository.
func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: sqlDBRunner{db: db}}
}

func newRepositoryWithRunner(runner dbRunner) *Repository {
	return &Repository{db: runner}
}

// CreateExperiment inserts a new experiment.
func (r *Repository) CreateExperiment(ctx context.Context, experiment *Experiment) error {
	if experiment.Status == ExperimentStatusRunning {
		if err := r.validateRunningExperiment(ctx, experiment); err != nil {
			return err
		}
	}
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO ab_experiments (name, description, template_name, locale, status, started_at, ended_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING id, created_at, updated_at`,
		experiment.Name,
		experiment.Description,
		experiment.TemplateName,
		experiment.Locale,
		string(experiment.Status),
		experiment.StartedAt,
		experiment.EndedAt,
		nullString(experiment.CreatedBy),
	).Scan(&experiment.ID, &experiment.CreatedAt, &experiment.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create ab experiment: %w", err)
	}
	return nil
}

// UpdateExperiment updates an experiment. Moving to running validates variants.
func (r *Repository) UpdateExperiment(ctx context.Context, experiment *Experiment) error {
	if experiment.Status == ExperimentStatusRunning {
		if err := r.validateRunningExperiment(ctx, experiment); err != nil {
			return err
		}
	}
	_, err := r.db.ExecContext(ctx, `
		UPDATE ab_experiments
		SET name = $2,
		    description = $3,
		    template_name = $4,
		    locale = $5,
		    status = $6,
		    started_at = $7,
		    ended_at = $8,
		    updated_at = now()
		WHERE id = $1`,
		experiment.ID,
		experiment.Name,
		experiment.Description,
		experiment.TemplateName,
		experiment.Locale,
		string(experiment.Status),
		experiment.StartedAt,
		experiment.EndedAt,
	)
	if err != nil {
		return fmt.Errorf("update ab experiment: %w", err)
	}
	return nil
}

// GetExperiment returns one experiment by id.
func (r *Repository) GetExperiment(ctx context.Context, id string) (*Experiment, error) {
	experiment := &Experiment{}
	err := scanExperiment(r.db.QueryRowContext(ctx, `
		SELECT id, name, description, template_name, locale, status, started_at, ended_at, created_by, created_at, updated_at
		FROM ab_experiments
		WHERE id = $1`,
		id,
	), experiment)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("ab experiment not found: %w", err)
	}
	if err != nil {
		return nil, fmt.Errorf("get ab experiment: %w", err)
	}
	return experiment, nil
}

// ListExperiments returns experiments, optionally filtered by status.
func (r *Repository) ListExperiments(ctx context.Context, status string) ([]Experiment, error) {
	query := `
		SELECT id, name, description, template_name, locale, status, started_at, ended_at, created_by, created_at, updated_at
		FROM ab_experiments`
	args := []any{}
	if status != "" {
		query += ` WHERE status = $1`
		args = append(args, status)
	}
	query += ` ORDER BY created_at DESC`
	return r.queryExperiments(ctx, query, args...)
}

// AddVariant inserts a variant after verifying its template version exists.
func (r *Repository) AddVariant(ctx context.Context, variant *Variant) error {
	experiment, err := r.GetExperiment(ctx, variant.ExperimentID)
	if err != nil {
		return err
	}
	if err := r.validatePromptTemplateVersion(ctx, experiment, variant.TemplateVersion); err != nil {
		return err
	}
	err = r.db.QueryRowContext(ctx, `
		INSERT INTO ab_variants (experiment_id, name, template_version, traffic_pct, is_control)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id`,
		variant.ExperimentID,
		variant.Name,
		variant.TemplateVersion,
		variant.TrafficPct,
		variant.IsControl,
	).Scan(&variant.ID)
	if err != nil {
		return fmt.Errorf("add ab variant: %w", err)
	}
	return nil
}

// UpdateVariant updates a variant after verifying its template version exists.
func (r *Repository) UpdateVariant(ctx context.Context, variant *Variant) error {
	experiment, err := r.GetExperiment(ctx, variant.ExperimentID)
	if err != nil {
		return err
	}
	if err := r.validatePromptTemplateVersion(ctx, experiment, variant.TemplateVersion); err != nil {
		return err
	}
	_, err = r.db.ExecContext(ctx, `
		UPDATE ab_variants
		SET name = $2,
		    template_version = $3,
		    traffic_pct = $4,
		    is_control = $5
		WHERE id = $1`,
		variant.ID,
		variant.Name,
		variant.TemplateVersion,
		variant.TrafficPct,
		variant.IsControl,
	)
	if err != nil {
		return fmt.Errorf("update ab variant: %w", err)
	}
	return nil
}

// ListVariants returns variants for one experiment.
func (r *Repository) ListVariants(ctx context.Context, experimentID string) (variants []Variant, err error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, experiment_id, name, template_version, traffic_pct, is_control
		FROM ab_variants
		WHERE experiment_id = $1
		ORDER BY id`,
		experimentID,
	)
	if err != nil {
		return nil, fmt.Errorf("list ab variants: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close ab variant rows: %w", closeErr))
		}
	}()
	variants = make([]Variant, 0, 2)
	for rows.Next() {
		var variant Variant
		if err := rows.Scan(&variant.ID, &variant.ExperimentID, &variant.Name, &variant.TemplateVersion, &variant.TrafficPct, &variant.IsControl); err != nil {
			return nil, fmt.Errorf("scan ab variant: %w", err)
		}
		variants = append(variants, variant)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ab variants: %w", err)
	}
	return variants, nil
}

// DeleteVariant removes one variant.
func (r *Repository) DeleteVariant(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM ab_variants WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete ab variant: %w", err)
	}
	return nil
}

// GetRunningExperimentsForTemplate returns running experiments for one template and locale.
func (r *Repository) GetRunningExperimentsForTemplate(ctx context.Context, templateName, locale string) ([]Experiment, error) {
	return r.queryExperiments(ctx, `
		SELECT id, name, description, template_name, locale, status, started_at, ended_at, created_by, created_at, updated_at
		FROM ab_experiments
		WHERE template_name = $1 AND locale = $2 AND status = $3
		ORDER BY created_at DESC`,
		templateName,
		locale,
		string(ExperimentStatusRunning),
	)
}

func (r *Repository) validateRunningExperiment(ctx context.Context, experiment *Experiment) error {
	variants, err := r.ListVariants(ctx, experiment.ID)
	if err != nil {
		return err
	}
	if err := ValidateVariantsForAllocation(variants); err != nil {
		return fmt.Errorf("validate ab experiment variants: %w", err)
	}
	for _, variant := range variants {
		if err := r.validatePromptTemplateVersion(ctx, experiment, variant.TemplateVersion); err != nil {
			return err
		}
	}
	return nil
}

func (r *Repository) validatePromptTemplateVersion(ctx context.Context, experiment *Experiment, version int) error {
	var exists int
	err := r.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM ai_prompt_templates
		WHERE name = $1 AND locale = $2 AND version = $3`,
		experiment.TemplateName,
		experiment.Locale,
		version,
	).Scan(&exists)
	if err != nil {
		return fmt.Errorf("validate prompt template version: %w", err)
	}
	if exists == 0 {
		return fmt.Errorf("prompt template version does not exist: name=%s locale=%s version=%d", experiment.TemplateName, experiment.Locale, version)
	}
	return nil
}

func (r *Repository) queryExperiments(ctx context.Context, query string, args ...any) (experiments []Experiment, err error) {
	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query ab experiments: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close ab experiment rows: %w", closeErr))
		}
	}()
	experiments = make([]Experiment, 0, 4)
	for rows.Next() {
		var experiment Experiment
		if err := scanExperiment(rows, &experiment); err != nil {
			return nil, fmt.Errorf("scan ab experiment: %w", err)
		}
		experiments = append(experiments, experiment)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate ab experiments: %w", err)
	}
	return experiments, nil
}

func scanExperiment(row rowScanner, experiment *Experiment) error {
	var status string
	var startedAt, endedAt sql.NullTime
	var createdBy sql.NullString
	if err := row.Scan(
		&experiment.ID,
		&experiment.Name,
		&experiment.Description,
		&experiment.TemplateName,
		&experiment.Locale,
		&status,
		&startedAt,
		&endedAt,
		&createdBy,
		&experiment.CreatedAt,
		&experiment.UpdatedAt,
	); err != nil {
		return err
	}
	experiment.Status = ExperimentStatus(status)
	if startedAt.Valid {
		experiment.StartedAt = new(startedAt.Time)
	}
	if endedAt.Valid {
		experiment.EndedAt = new(endedAt.Time)
	}
	if createdBy.Valid {
		experiment.CreatedBy = createdBy.String
	}
	return nil
}

func nullString(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}

var _ dbRunner = sqlDBRunner{}
var _ rowsScanner = (*sql.Rows)(nil)
var _ rowScanner = (*sql.Row)(nil)
