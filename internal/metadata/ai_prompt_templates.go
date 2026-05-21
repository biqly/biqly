package metadata

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/biqly/biqly/internal/i18n"
)

// PromptTemplate is one named prompt section for a locale.
type PromptTemplate struct {
	Name      string    `json:"name"`
	Locale    string    `json:"locale"`
	Content   string    `json:"content"`
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

// GetPromptTemplate loads a single section. Missing rows return ("", nil).
func (r *Repository) GetPromptTemplate(ctx context.Context, name string, loc i18n.Locale) (string, error) {
	if loc == "" {
		loc = i18n.DefaultLocale
	}
	var content string
	err := r.db.QueryRowContext(ctx, `
		SELECT content FROM ai_prompt_templates
		WHERE name = $1 AND locale = $2`,
		name, string(loc),
	).Scan(&content)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil
		}
		return "", fmt.Errorf("get prompt template: %w", err)
	}
	return content, nil
}

// UpsertPromptTemplate inserts or replaces a prompt section.
func (r *Repository) UpsertPromptTemplate(ctx context.Context, name string, loc i18n.Locale, content string) error {
	if loc == "" {
		loc = i18n.DefaultLocale
	}
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO ai_prompt_templates (name, locale, content, updated_at)
		VALUES ($1, $2, $3, now())
		ON CONFLICT (name, locale) DO UPDATE SET
			content = EXCLUDED.content,
			updated_at = now()`,
		name, string(loc), content,
	)
	if err != nil {
		return fmt.Errorf("upsert prompt template: %w", err)
	}
	return nil
}

// ListPromptTemplates returns all rows (admin/diagnostics).
func (r *Repository) ListPromptTemplates(ctx context.Context) ([]PromptTemplate, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT name, locale, content, updated_at FROM ai_prompt_templates ORDER BY name, locale`)
	if err != nil {
		return nil, fmt.Errorf("list prompt templates: %w", err)
	}
	defer func() { _ = rows.Close() }()
	var out []PromptTemplate
	for rows.Next() {
		var t PromptTemplate
		if err := rows.Scan(&t.Name, &t.Locale, &t.Content, &t.UpdatedAt); err != nil {
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
