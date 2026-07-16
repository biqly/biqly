package metadata

import (
	"context"
	"errors"
	"fmt"

	"github.com/biqly/biqly/internal/i18n"
	platformdb "github.com/biqly/biqly/internal/platform/db"
	"github.com/biqly/biqly/internal/platform/db/pgarray"
)

// Entity types stored in entity_translations.entity_type.
const (
	EntityTypeTable             = "table"
	EntityTypeColumn            = "column"
	EntityTypeRelation          = "relation"
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
	rows, err := r.db.QueryContext(ctx, q, entityType, pgarray.Strings(entityIDs), pgarray.Strings(langs))
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

// EntitiesWithTranslation returns the subset of entityIDs that already have at
// least one stored translation row in exactly `lang` (no English fallback).
// Used to skip entities already translated for a locale before calling the LLM.
func (r *Repository) EntitiesWithTranslation(ctx context.Context, entityType string, entityIDs []string, lang i18n.Locale) (map[string]bool, error) {
	out := make(map[string]bool, len(entityIDs))
	if len(entityIDs) == 0 {
		return out, nil
	}
	const q = `
		SELECT DISTINCT entity_id::text
		FROM entity_translations
		WHERE entity_type = $1 AND entity_id = ANY($2::uuid[]) AND lang = $3
	`
	rows, err := r.db.QueryContext(ctx, q, entityType, pgarray.Strings(entityIDs), string(lang))
	if err != nil {
		return nil, fmt.Errorf("query translated entities: %w", err)
	}
	defer func() { _ = rows.Close() }()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan translated entity id: %w", err)
		}
		out[id] = true
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("translated entity rows: %w", err)
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

func overlayDescriptionTranslations(tr map[string]EntityTranslations, index map[string]int, apply func(i int, description string)) {
	for id, m := range tr {
		if v, ok := m[TranslationFieldDescription]; ok && v != "" {
			apply(index[id], v)
		}
	}
}

func applyDescriptionTranslations[E any](
	ctx context.Context,
	r *Repository,
	entities []E,
	entityType string,
	entityID func(E) string,
	setDescription func(int, string),
	loc i18n.Locale,
) error {
	if len(entities) == 0 {
		return nil
	}
	ids := make([]string, 0, len(entities))
	index := make(map[string]int, len(entities))
	for i, entity := range entities {
		id := entityID(entity)
		ids = append(ids, id)
		index[id] = i
	}
	tr, err := r.GetEntityTranslations(ctx, entityType, ids, loc)
	if err != nil {
		return err
	}
	overlayDescriptionTranslations(tr, index, setDescription)
	return nil
}

// ApplyTableTranslations overlays localized description (and label) onto a
// slice of tables in-place. Tables without a translation keep their raw
// description.
func (r *Repository) ApplyTableTranslations(ctx context.Context, tables []Table, loc i18n.Locale) error {
	return applyDescriptionTranslations(ctx, r, tables, EntityTypeTable,
		func(t Table) string { return t.ID },
		func(i int, description string) { tables[i].Description = new(description) },
		loc,
	)
}

// ApplyColumnTranslations overlays localized description onto a slice of
// columns in-place.
func (r *Repository) ApplyColumnTranslations(ctx context.Context, cols []Column, loc i18n.Locale) error {
	return applyDescriptionTranslations(ctx, r, cols, EntityTypeColumn,
		func(c Column) string { return c.ID },
		func(i int, description string) { cols[i].Description = new(description) },
		loc,
	)
}

// ApplyRelationTranslations overlays localized description onto a slice of
// relation details in-place.
func (r *Repository) ApplyRelationTranslations(ctx context.Context, rels []RelationDetail, loc i18n.Locale) error {
	return applyDescriptionTranslations(ctx, r, rels, EntityTypeRelation,
		func(rel RelationDetail) string { return rel.ID },
		func(i int, description string) { rels[i].Description = description },
		loc,
	)
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
