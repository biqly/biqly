package handlers

import (
	"context"
	"net/http"
	"strings"

	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/i18n"
	"github.com/biqly/biqly/internal/metadata"
	pkgsemantic "github.com/biqly/biqly/pkg/semantic"
)

// pendingTranslation is one entity whose label/description still needs a
// translation row for the requested locale.
type pendingTranslation struct {
	entityType  string
	id          string
	label       string
	description string
}

// TranslateSemanticModel ensures the request-locale has cached translations for
// a semantic model and its dimensions/metrics. Missing label/description text is
// translated via the AI translation layer and persisted to entity_translations,
// which the catalog model-read then overlays. Idempotent and cheap once cached:
// the LLM is only invoked for entries that have text but no stored translation
// in the requested locale, so repeated calls are a couple of indexed lookups.
func (h *AIHandler) TranslateSemanticModel(w http.ResponseWriter, r *http.Request) {
	id, ok := requireURLParam(w, r, "id")
	if !ok {
		return
	}
	ctx := r.Context()
	loc := i18n.FromContext(ctx)

	// English is the stored base language; nothing to generate. Also a no-op
	// when the translation layer is unconfigured.
	if loc == i18n.DefaultLocale || h.deps.Translator == nil || h.deps.SemanticRepo == nil || h.deps.MetaRepo == nil {
		writeJSON(w, http.StatusOK, map[string]int{"translated": 0})
		return
	}

	model, err := h.deps.SemanticRepo.GetFullModel(ctx, id)
	if err != nil {
		writeInternalError(ctx, w, http.StatusNotFound, "model not found", err)
		return
	}

	// force=true re-translates every entry, overwriting cached values — used by
	// the "re-translate" action when the source (English) text has changed and
	// the cached translation is stale.
	force := r.URL.Query().Get("force") == "true"

	n, err := h.ensureModelTranslations(ctx, model, loc, force)
	if err != nil {
		writeInternalError(ctx, w, http.StatusBadGateway, "semantic model translation failed", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"translated": n})
}

func (h *AIHandler) ensureModelTranslations(ctx context.Context, model *pkgsemantic.SemanticModel, loc i18n.Locale, force bool) (int, error) {
	pending, err := h.collectPendingTranslations(ctx, model, loc, force)
	if err != nil {
		return 0, err
	}
	if len(pending) == 0 {
		return 0, nil
	}

	fields := make([]ai.TranslatableField, len(pending))
	for i, p := range pending {
		fields[i] = ai.TranslatableField{Key: p.id, Label: p.label, Description: p.description}
	}
	translated, err := h.deps.Translator.TranslateFields(ctx, fields)
	if err != nil {
		return 0, err
	}

	count := 0
	for i := range translated {
		p := pending[i] // TranslateFields preserves order
		wrote := false
		if v := strings.TrimSpace(translated[i].Label); v != "" {
			if err := h.upsertTranslation(ctx, p.entityType, p.id, loc, metadata.TranslationFieldLabel, v); err != nil {
				return count, err
			}
			wrote = true
		}
		if v := strings.TrimSpace(translated[i].Description); v != "" {
			if err := h.upsertTranslation(ctx, p.entityType, p.id, loc, metadata.TranslationFieldDescription, v); err != nil {
				return count, err
			}
			wrote = true
		}
		if wrote {
			count++
		}
	}
	return count, nil
}

func (h *AIHandler) upsertTranslation(ctx context.Context, entityType, id string, loc i18n.Locale, field, value string) error {
	return h.deps.MetaRepo.UpsertTranslation(ctx, metadata.Translation{
		EntityType: entityType,
		EntityID:   id,
		Lang:       string(loc),
		Field:      field,
		Value:      value,
	})
}

// collectPendingTranslations returns the model, dimension, and metric entities
// that carry label/description text and still need a translation for loc. When
// force is true every entity with text is returned (re-translate, overwriting
// the cache); otherwise entities already translated for loc are skipped.
func (h *AIHandler) collectPendingTranslations(ctx context.Context, model *pkgsemantic.SemanticModel, loc i18n.Locale, force bool) ([]pendingTranslation, error) {
	var pending []pendingTranslation

	modelDone, err := h.translatedSet(ctx, metadata.EntityTypeSemanticModel, []string{model.ID}, loc, force)
	if err != nil {
		return nil, err
	}
	if !modelDone[model.ID] {
		if p, ok := newPendingTranslation(metadata.EntityTypeSemanticModel, model.ID, model.Label, model.Description); ok {
			pending = append(pending, p)
		}
	}

	dimIDs := make([]string, len(model.Dimensions))
	for i := range model.Dimensions {
		dimIDs[i] = model.Dimensions[i].ID
	}
	dimDone, err := h.translatedSet(ctx, metadata.EntityTypeSemanticDimension, dimIDs, loc, force)
	if err != nil {
		return nil, err
	}
	for i := range model.Dimensions {
		d := &model.Dimensions[i]
		if dimDone[d.ID] {
			continue
		}
		if p, ok := newPendingTranslation(metadata.EntityTypeSemanticDimension, d.ID, d.Label, d.Description); ok {
			pending = append(pending, p)
		}
	}

	metricIDs := make([]string, len(model.Metrics))
	for i := range model.Metrics {
		metricIDs[i] = model.Metrics[i].ID
	}
	metricDone, err := h.translatedSet(ctx, metadata.EntityTypeSemanticMetric, metricIDs, loc, force)
	if err != nil {
		return nil, err
	}
	for i := range model.Metrics {
		m := &model.Metrics[i]
		if metricDone[m.ID] {
			continue
		}
		if p, ok := newPendingTranslation(metadata.EntityTypeSemanticMetric, m.ID, m.Label, m.Description); ok {
			pending = append(pending, p)
		}
	}

	return pending, nil
}

// translatedSet returns the entity ids already translated for loc, or an empty
// set when force is set so every entity is re-translated.
func (h *AIHandler) translatedSet(ctx context.Context, entityType string, ids []string, loc i18n.Locale, force bool) (map[string]bool, error) {
	if force {
		return map[string]bool{}, nil
	}
	return h.deps.MetaRepo.EntitiesWithTranslation(ctx, entityType, ids, loc)
}

func newPendingTranslation(entityType, id string, label, description *string) (pendingTranslation, bool) {
	l := strOrEmpty(label)
	d := strOrEmpty(description)
	if strings.TrimSpace(l) == "" && strings.TrimSpace(d) == "" {
		return pendingTranslation{}, false
	}
	return pendingTranslation{entityType: entityType, id: id, label: l, description: d}, true
}

func strOrEmpty(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
