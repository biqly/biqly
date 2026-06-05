package metadata

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/biqly/biqly/internal/i18n"
	platformdb "github.com/biqly/biqly/internal/platform/db"
)

// PromptTemplate is one named prompt section for a locale.
type PromptTemplate struct {
	Name      string    `json:"name"`
	Locale    string    `json:"locale"`
	Version   int       `json:"version"`
	Content   string    `json:"content"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CountPromptTemplates returns how many rows exist (used for first-run seeding).
func (r *Repository) CountPromptTemplates(ctx context.Context) (int, error) {
	var n int
	if err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM ai_prompt_templates`).Scan(&n); err != nil {
		return 0, fmt.Errorf("count prompt templates: %w", err)
	}
	return n, nil
}

// GetPromptTemplate loads the active section. Missing rows return ("", nil).
func (r *Repository) GetPromptTemplate(ctx context.Context, name string, loc i18n.Locale) (string, error) {
	content, _, err := r.GetPromptTemplateVersion(ctx, name, loc)
	return content, err
}

// GetPromptTemplateVersion loads the active section with its version.
func (r *Repository) GetPromptTemplateVersion(ctx context.Context, name string, loc i18n.Locale) (string, int, error) {
	if loc == "" {
		loc = i18n.DefaultLocale
	}
	var content string
	var version int
	err := r.db.QueryRowContext(ctx, `
		SELECT content, version FROM ai_prompt_templates
		WHERE name = $1 AND locale = $2 AND is_active = TRUE
		ORDER BY version DESC LIMIT 1`,
		name, string(loc),
	).Scan(&content, &version)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", 0, nil
		}
		return "", 0, fmt.Errorf("get prompt template: %w", err)
	}
	return content, version, nil
}

// UpsertPromptTemplate creates a new active version for a prompt section.
func (r *Repository) UpsertPromptTemplate(ctx context.Context, name string, loc i18n.Locale, content string) error {
	if loc == "" {
		loc = i18n.DefaultLocale
	}
	return platformdb.RunInTx(ctx, r.db, func(tx *sql.Tx) error {
		var nextVersion int
		if err := tx.QueryRowContext(ctx, `
			SELECT COALESCE(MAX(version), 0) + 1
			FROM ai_prompt_templates
			WHERE name = $1 AND locale = $2`,
			name, string(loc),
		).Scan(&nextVersion); err != nil {
			return fmt.Errorf("next prompt template version: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE ai_prompt_templates
			SET is_active = FALSE, updated_at = now()
			WHERE name = $1 AND locale = $2 AND is_active = TRUE`,
			name, string(loc),
		); err != nil {
			return fmt.Errorf("deactivate prompt template versions: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO ai_prompt_templates (name, locale, version, content, is_active, created_at, updated_at)
			VALUES ($1, $2, $3, $4, TRUE, now(), now())`,
			name, string(loc), nextVersion, content,
		); err != nil {
			return fmt.Errorf("upsert prompt template: %w", err)
		}
		return nil
	})
}

// ListPromptTemplates returns all rows including inactive versions.
func (r *Repository) ListPromptTemplates(ctx context.Context) ([]PromptTemplate, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT name, locale, version, content, is_active, created_at, updated_at
		FROM ai_prompt_templates
		ORDER BY name, locale, version DESC`)
	if err != nil {
		return nil, fmt.Errorf("list prompt templates: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			_ = closeErr
		}
	}()
	out := make([]PromptTemplate, 0, 16)
	for rows.Next() {
		var t PromptTemplate
		if err := rows.Scan(&t.Name, &t.Locale, &t.Version, &t.Content, &t.IsActive, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan prompt template: %w", err)
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

// DeleteAllPromptTemplates removes every row (used before full reseed).
func (r *Repository) DeleteAllPromptTemplates(ctx context.Context) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM ai_prompt_templates`)
	if err != nil {
		return fmt.Errorf("delete prompt templates: %w", err)
	}
	return nil
}

// GetPromptTemplateByVersion loads a specific version of a prompt section. Missing rows return ("", nil).
func (r *Repository) GetPromptTemplateByVersion(ctx context.Context, name string, loc i18n.Locale, version int) (string, error) {
	if loc == "" {
		loc = i18n.DefaultLocale
	}
	var content string
	err := r.db.QueryRowContext(ctx, `
		SELECT content FROM ai_prompt_templates
		WHERE name = $1 AND locale = $2 AND version = $3`,
		name, string(loc), version,
	).Scan(&content)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("get prompt template by version: %w", err)
	}
	return content, nil
}
