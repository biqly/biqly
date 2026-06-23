package handlers

import (
	"context"

	"github.com/biqly/biqly/internal/i18n"
	"github.com/biqly/biqly/internal/metadata"
	pkgsemantic "github.com/biqly/biqly/pkg/semantic"
)

// applyModelTranslations overlays locale-specific label/description text from
// entity_translations onto a semantic model and its dimensions/metrics, in
// place. The stored English values (the base columns) remain whenever no
// override exists for the requested locale. It is a best-effort, no-op for the
// default locale or when the metadata repo is unavailable, so a translation
// lookup failure never breaks the model read.
func (h *SemanticHandler) applyModelTranslations(ctx context.Context, model *pkgsemantic.SemanticModel) {
	if model == nil {
		return
	}
	loc := i18n.FromContext(ctx)
	if loc == i18n.DefaultLocale {
		return
	}
	repo := h.deps.MetaRepo
	if repo == nil {
		return
	}

	if tr, err := repo.GetEntityTranslations(ctx, metadata.EntityTypeSemanticModel, []string{model.ID}, loc); err == nil {
		applyTranslationOverlay([]*pkgsemantic.SemanticModel{model}, tr,
			func(m **pkgsemantic.SemanticModel) string { return (*m).ID },
			func(m **pkgsemantic.SemanticModel, v string) { (*m).Label = new(v) },
			func(m **pkgsemantic.SemanticModel, v string) { (*m).Description = new(v) },
		)
	}

	if tr := fetchEntityTranslations(ctx, repo, metadata.EntityTypeSemanticDimension, model.Dimensions, loc, func(d *pkgsemantic.Dimension) string { return d.ID }); tr != nil {
		applyTranslationOverlay(model.Dimensions, tr,
			func(d *pkgsemantic.Dimension) string { return d.ID },
			func(d *pkgsemantic.Dimension, v string) { d.Label = new(v) },
			func(d *pkgsemantic.Dimension, v string) { d.Description = new(v) },
		)
	}

	if tr := fetchEntityTranslations(ctx, repo, metadata.EntityTypeSemanticMetric, model.Metrics, loc, func(m *pkgsemantic.Metric) string { return m.ID }); tr != nil {
		applyTranslationOverlay(model.Metrics, tr,
			func(m *pkgsemantic.Metric) string { return m.ID },
			func(m *pkgsemantic.Metric, v string) { m.Label = new(v) },
			func(m *pkgsemantic.Metric, v string) { m.Description = new(v) },
		)
	}
}

// fetchEntityTranslations collects the entity ids and queries the translation
// store for the requested locale. Returns nil when there is nothing to overlay.
func fetchEntityTranslations[T any](
	ctx context.Context,
	repo *metadata.Repository,
	entityType string,
	items []T,
	loc i18n.Locale,
	id func(*T) string,
) map[string]metadata.EntityTranslations {
	if len(items) == 0 {
		return nil
	}
	ids := make([]string, len(items))
	for i := range items {
		ids[i] = id(&items[i])
	}
	tr, err := repo.GetEntityTranslations(ctx, entityType, ids, loc)
	if err != nil || len(tr) == 0 {
		return nil
	}
	return tr
}

// applyTranslationOverlay writes localized label/description values onto each
// item whose id has a matching translation row. Empty translation values are
// ignored so a partial row never blanks out the base text. Pure (no I/O) so it
// is unit-testable without a database.
func applyTranslationOverlay[T any](
	items []T,
	tr map[string]metadata.EntityTranslations,
	id func(*T) string,
	setLabel func(*T, string),
	setDescription func(*T, string),
) {
	if len(tr) == 0 {
		return
	}
	for i := range items {
		fields, ok := tr[id(&items[i])]
		if !ok {
			continue
		}
		if v := fields[metadata.TranslationFieldLabel]; v != "" {
			setLabel(&items[i], v)
		}
		if v := fields[metadata.TranslationFieldDescription]; v != "" {
			setDescription(&items[i], v)
		}
	}
}
