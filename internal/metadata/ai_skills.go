package metadata

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/biqly/biqly/internal/platform/db/pgarray"
)

// ErrSkillNotFound is returned when a skill id does not exist.
var ErrSkillNotFound = errors.New("skill not found")

// SkillRow is a saved, parameterized LogicalQuery template ("skill") that can
// be re-run through the governed query path without fresh LLM generation.
type SkillRow struct {
	ID             string
	DatasourceID   string
	ModelID        string
	Name           string
	Description    string
	Question       string
	LogicalQuery   []byte
	Parameters     []byte
	Tags           []string
	CreatedBy      string
	Version        int
	IsActive       bool
	LastVerifiedAt *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// SkillInsert is the input for creating a skill.
type SkillInsert struct {
	DatasourceID string
	ModelID      string
	Name         string
	Description  string
	Question     string
	LogicalQuery []byte
	Parameters   []byte
	Tags         []string
	CreatedBy    string
}

// SkillUpdate is the input for updating a skill; the version is bumped.
type SkillUpdate struct {
	Name         string
	Description  string
	Question     string
	LogicalQuery []byte
	Parameters   []byte
	Tags         []string
	IsActive     bool
}

const skillColumns = `id, datasource_id, COALESCE(model_id::text, ''), name, description, question,
	logical_query, parameters, tags, created_by, version, is_active, last_verified_at, created_at, updated_at`

// InsertSkill stores a new skill and returns its generated id.
func (r *Repository) InsertSkill(ctx context.Context, in SkillInsert) (string, error) {
	var modelID any
	if in.ModelID != "" {
		modelID = in.ModelID
	}
	var id string
	err := r.db.QueryRowContext(ctx, `
		INSERT INTO ai_skills (datasource_id, model_id, name, description, question, logical_query, parameters, tags, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`, in.DatasourceID, modelID, in.Name, in.Description, in.Question, in.LogicalQuery, in.Parameters, pgarray.Strings(in.Tags), in.CreatedBy).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("insert skill: %w", err)
	}
	return id, nil
}

// ListSkills returns skills for a datasource, newest-updated first. An empty
// datasourceID lists across all datasources (admin/MCP catalog view).
func (r *Repository) ListSkills(ctx context.Context, datasourceID string) ([]SkillRow, error) {
	query := `SELECT ` + skillColumns + ` FROM ai_skills WHERE ($1 = '' OR datasource_id::text = $1) ORDER BY updated_at DESC`
	rows, err := r.db.QueryContext(ctx, query, datasourceID)
	if err != nil {
		return nil, fmt.Errorf("list skills: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []SkillRow
	for rows.Next() {
		row, err := scanSkill(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list skills rows: %w", err)
	}
	return out, nil
}

// GetSkill returns a single skill by id.
func (r *Repository) GetSkill(ctx context.Context, id string) (SkillRow, error) {
	row := r.db.QueryRowContext(ctx, `SELECT `+skillColumns+` FROM ai_skills WHERE id = $1`, id)
	skill, err := scanSkill(row)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SkillRow{}, fmt.Errorf("skill %s: %w", id, ErrSkillNotFound)
		}
		return SkillRow{}, err
	}
	return skill, nil
}

// UpdateSkill replaces the editable fields of a skill and bumps its version.
func (r *Repository) UpdateSkill(ctx context.Context, id string, in SkillUpdate) error {
	res, err := r.db.ExecContext(ctx, `
		UPDATE ai_skills
		SET name = $2, description = $3, question = $4, logical_query = $5, parameters = $6,
			tags = $7, is_active = $8, version = version + 1, updated_at = now()
		WHERE id = $1
	`, id, in.Name, in.Description, in.Question, in.LogicalQuery, in.Parameters, pgarray.Strings(in.Tags), in.IsActive)
	if err != nil {
		return fmt.Errorf("update skill: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("update skill affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("skill %s: %w", id, ErrSkillNotFound)
	}
	return nil
}

// DeleteSkill removes a skill.
func (r *Repository) DeleteSkill(ctx context.Context, id string) error {
	res, err := r.db.ExecContext(ctx, `DELETE FROM ai_skills WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("delete skill: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete skill affected: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("skill %s: %w", id, ErrSkillNotFound)
	}
	return nil
}

// TouchSkillVerified records a successful governed run of the skill.
func (r *Repository) TouchSkillVerified(ctx context.Context, id string) error {
	if _, err := r.db.ExecContext(ctx, `UPDATE ai_skills SET last_verified_at = now() WHERE id = $1`, id); err != nil {
		return fmt.Errorf("touch skill verified: %w", err)
	}
	return nil
}

// DatasourceForSkill resolves a skill id to its owning datasource for
// access-control middleware.
func (r *Repository) DatasourceForSkill(ctx context.Context, id string) (string, error) {
	var datasourceID string
	err := r.db.QueryRowContext(ctx, `SELECT datasource_id FROM ai_skills WHERE id = $1`, id).Scan(&datasourceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", fmt.Errorf("skill %s: %w", id, ErrSkillNotFound)
		}
		return "", fmt.Errorf("datasource for skill: %w", err)
	}
	return datasourceID, nil
}

func scanSkill(s interface{ Scan(...any) error }) (SkillRow, error) {
	var (
		row  SkillRow
		tags pgarray.StringArray
	)
	if err := s.Scan(
		&row.ID, &row.DatasourceID, &row.ModelID, &row.Name, &row.Description, &row.Question,
		&row.LogicalQuery, &row.Parameters, &tags, &row.CreatedBy, &row.Version, &row.IsActive,
		&row.LastVerifiedAt, &row.CreatedAt, &row.UpdatedAt,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return row, err
		}
		return row, fmt.Errorf("scan skill: %w", err)
	}
	row.Tags = []string(tags)
	return row, nil
}
