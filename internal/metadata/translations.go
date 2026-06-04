package metadata

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/biqly/biqly/internal/i18n"
	platformdb "github.com/biqly/biqly/internal/platform/db"
	"github.com/lib/pq"
)

// Entity types stored in entity_translations.entity_type.
const (
	EntityTypeTable             = "table"
	EntityTypeColumn            = "column"
	EntityTypeSemanticModel     = "semantic_model"
	EntityTypeSemanticDimension = "semantic_dimension"
	EntityTypeSemanticMetric    = "semantic_metric"
)

// Translation field names stored in entity_translations.field.
const (
	TranslationFieldDescription = "description"
	TranslationFieldLabel       = "label"
)

// Translation is a single localized override for an entity field.
type Translation struct {
	EntityType string
	EntityID   string
	Lang       string
	Field      string
	Value      string
}

// EntityTranslations is the per-entity { field → value } map for one language.
type EntityTranslations map[string]string

// TranslationKey identifies an entity for batch translation lookup.
type TranslationKey struct {
	EntityType string
	EntityID   string
}

// GetEntityTranslations fetches translations for a batch of entities in the
// requested language. The result key is the EntityID. When a row is missing
// in `lang` and `lang != DefaultLocale`, the DefaultLocale row is returned
// as a fallback (English overlay) when present. Callers should still fall
// back to the raw `description` column on the source row when nothing is
// found here.
func (r *Repository) GetEntityTranslations(
	ctx context.Context,
	entityType string,
	entityIDs []string,
	lang i18n.Locale,
) (map[string]EntityTranslations, error) {
	out := make(map[string]EntityTranslations, len(entityIDs))
	if len(entityIDs) == 0 {
		return out, nil
	}
	const q = `
		SELECT entity_id::text, lang, field, value
		FROM entity_translations
		WHERE entity_type = $1
		  AND entity_id = ANY($2::uuid[])
		  AND lang = ANY($3::text[])
	`
	langs := uniqueLangs(string(lang), string(i18n.DefaultLocale))
	rows, err := r.db.QueryContext(ctx, q, entityType, pq.Array(entityIDs), pq.Array(langs))
	if err != nil {
		return nil, fmt.Errorf("query entity translations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	type langField struct{ lang, field string }
	bag := make(map[string]map[langField]string)
	for rows.Next() {
		var id, l, field, value string
		if err := rows.Scan(&id, &l, &field, &value); err != nil {
			return nil, fmt.Errorf("scan entity translation: %w", err)
		}
		m, ok := bag[id]
		if !ok {
			m = make(map[langField]string)
			bag[id] = m
		}
		m[langField{lang: l, field: field}] = value
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("entity translation rows: %w", err)
	}

	for id, m := range bag {
		merged := EntityTranslations{}
		// English (fallback) first, primary lang overwrites.
		for _, l := range []string{string(i18n.DefaultLocale), string(lang)} {
			for k, v := range m {
				if k.lang == l && v != "" {
					merged[k.field] = v
				}
			}
		}
		if len(merged) > 0 {
			out[id] = merged
		}
	}
	return out, nil
}

// UpsertTranslation writes a single (entity_type, entity_id, lang, field)
// localized value. An empty value deletes the row.
func (r *Repository) UpsertTranslation(ctx context.Context, t Translation) error {
	if t.EntityType == "" || t.EntityID == "" || t.Lang == "" || t.Field == "" {
		return errors.New("translation: missing required fields")
	}
	if t.Value == "" {
		const del = `
			DELETE FROM entity_translations
			WHERE entity_type = $1 AND entity_id = $2::uuid AND lang = $3 AND field = $4
		`
		if _, err := r.db.ExecContext(ctx, del, t.EntityType, t.EntityID, t.Lang, t.Field); err != nil {
			return fmt.Errorf("delete entity translation: %w", err)
		}
		return nil
	}
	const up = `
		INSERT INTO entity_translations (entity_type, entity_id, lang, field, value, updated_at)
		VALUES ($1, $2::uuid, $3, $4, $5, now())
		ON CONFLICT (entity_type, entity_id, lang, field)
		DO UPDATE SET value = EXCLUDED.value, updated_at = now()
	`
	if _, err := r.db.ExecContext(ctx, up, t.EntityType, t.EntityID, t.Lang, t.Field, t.Value); err != nil {
		return fmt.Errorf("upsert entity translation: %w", err)
	}
	return nil
}

// ListEntityTranslations returns every stored translation for a single entity
// (across all languages). Useful for the "edit description" panel.
func (r *Repository) ListEntityTranslations(ctx context.Context, entityType, entityID string) ([]Translation, error) {
	const q = `
		SELECT entity_type, entity_id::text, lang, field, value
		FROM entity_translations
		WHERE entity_type = $1 AND entity_id = $2::uuid
		ORDER BY lang, field
	`
	rows, err := platformdb.QuerySliceErr(ctx, r.db, "list entity translations", q,
		[]any{entityType, entityID},
		func(s platformdb.Scanner) (*Translation, error) {
			var t Translation
			if err := s.Scan(&t.EntityType, &t.EntityID, &t.Lang, &t.Field, &t.Value); err != nil {
				return nil, fmt.Errorf("scan translation: %w", err)
			}
			return &t, nil
		})
	if err != nil {
		return nil, err
	}
	out := make([]Translation, 0, len(rows))
	for _, t := range rows {
		out = append(out, *t)
	}
	return out, nil
}

// LocalizedDescription returns the best-fit description for an entity given a
// requested locale, falling back through: requested → DefaultLocale → raw.
func LocalizedDescription(translations EntityTranslations, raw sql.NullString) *string {
	if v, ok := translations[TranslationFieldDescription]; ok && v != "" {
		return new(v)
	}
	if raw.Valid && raw.String != "" {
		return new(raw.String)
	}
	return nil
}

// ApplyTableTranslations overlays localized description (and label) onto a
// slice of tables in-place. Tables without a translation keep their raw
// description.
func (r *Repository) ApplyTableTranslations(ctx context.Context, tables []Table, loc i18n.Locale) error {
	if len(tables) == 0 {
		return nil
	}
	ids := make([]string, 0, len(tables))
	for _, t := range tables {
		ids = append(ids, t.ID)
	}
	tr, err := r.GetEntityTranslations(ctx, EntityTypeTable, ids, loc)
	if err != nil {
		return err
	}
	for i := range tables {
		if m, ok := tr[tables[i].ID]; ok {
			if v, ok := m[TranslationFieldDescription]; ok && v != "" {
				tables[i].Description = new(v)
			}
		}
	}
	return nil
}

// ApplyColumnTranslations overlays localized description onto a slice of
// columns in-place.
func (r *Repository) ApplyColumnTranslations(ctx context.Context, cols []Column, loc i18n.Locale) error {
	if len(cols) == 0 {
		return nil
	}
	ids := make([]string, 0, len(cols))
	for _, c := range cols {
		ids = append(ids, c.ID)
	}
	tr, err := r.GetEntityTranslations(ctx, EntityTypeColumn, ids, loc)
	if err != nil {
		return err
	}
	for i := range cols {
		if m, ok := tr[cols[i].ID]; ok {
			if v, ok := m[TranslationFieldDescription]; ok && v != "" {
				cols[i].Description = new(v)
			}
		}
	}
	return nil
}

func uniqueLangs(langs ...string) []string {
	seen := make(map[string]struct{}, len(langs))
	out := make([]string, 0, len(langs))
	for _, l := range langs {
		if l == "" {
			continue
		}
		if _, ok := seen[l]; ok {
			continue
		}
		seen[l] = struct{}{}
		out = append(out, l)
	}
	return out
}
