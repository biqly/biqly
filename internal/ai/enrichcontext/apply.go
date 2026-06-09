package enrichcontext

import (
	"context"
	"fmt"
	"strings"

	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/semantic"
)

func applyItems(
	ctx context.Context,
	model *semantic.SemanticModel,
	glossary []metadata.BusinessGlossaryRow,
	meta *metadata.Repository,
	sem *semantic.Repository,
	items []ApplyItem,
) (applied, skipped int, errs []string) {
	if model == nil || meta == nil || sem == nil {
		return 0, len(items), []string{"model or repositories missing"}
	}
	glossaryByID := make(map[string]metadata.BusinessGlossaryRow, len(glossary))
	for _, row := range glossary {
		glossaryByID[row.ID] = row
	}
	dimByID := make(map[string]semantic.Dimension, len(model.Dimensions))
	for _, d := range model.Dimensions {
		dimByID[d.ID] = d
	}
	metByID := make(map[string]semantic.Metric, len(model.Metrics))
	for _, m := range model.Metrics {
		metByID[m.ID] = m
	}

	for _, item := range items {
		value := strings.TrimSpace(item.Value)
		if value == "" {
			skipped++
			continue
		}
		if err := applyOne(ctx, model, glossaryByID, dimByID, metByID, meta, sem, item.GapID, value); err != nil {
			errs = append(errs, err.Error())
			skipped++
			continue
		}
		applied++
	}
	return applied, skipped, errs
}

func applyOne(
	ctx context.Context,
	model *semantic.SemanticModel,
	glossaryByID map[string]metadata.BusinessGlossaryRow,
	dimByID map[string]semantic.Dimension,
	metByID map[string]semantic.Metric,
	meta *metadata.Repository,
	sem *semantic.Repository,
	gapID, value string,
) error {
	switch {
	case strings.HasPrefix(gapID, "column:"):
		colID := strings.TrimPrefix(gapID, "column:")
		return meta.UpdateColumnDescription(ctx, colID, &value)
	case strings.HasPrefix(gapID, "dimension:"):
		dimID := strings.TrimPrefix(gapID, "dimension:")
		d, ok := dimByID[dimID]
		if !ok {
			return fmt.Errorf("dimension %s not found on model", dimID)
		}
		d.Description = &value
		return sem.UpdateDimension(ctx, &d)
	case strings.HasPrefix(gapID, "metric:"):
		metID := strings.TrimPrefix(gapID, "metric:")
		m, ok := metByID[metID]
		if !ok {
			return fmt.Errorf("metric %s not found on model", metID)
		}
		m.Description = &value
		return sem.UpdateMetric(ctx, &m)
	case strings.HasPrefix(gapID, "glossary:"):
		gID := strings.TrimPrefix(gapID, "glossary:")
		row, ok := glossaryByID[gID]
		if !ok {
			return fmt.Errorf("glossary term %s not found", gID)
		}
		return meta.UpdateBusinessGlossary(ctx, gID, metadata.BusinessGlossaryUpdate{
			Term:       row.Term,
			Definition: value,
			MapsToType: row.MapsToType,
			MapsToName: row.MapsToName,
			Aliases:    row.Aliases,
			AIContext:  row.AIContext,
		})
	case strings.HasPrefix(gapID, "enum:"):
		parts := strings.SplitN(strings.TrimPrefix(gapID, "enum:"), ":", 2)
		if len(parts) != 2 {
			return fmt.Errorf("invalid enum gap id %q", gapID)
		}
		dimID, rawValue := parts[0], parts[1]
		d, ok := dimByID[dimID]
		if !ok {
			return fmt.Errorf("dimension %s not found on model", dimID)
		}
		updated := false
		mappings := make([]semantic.EnumMapping, len(d.EnumValues))
		copy(mappings, d.EnumValues)
		for i := range mappings {
			if mappings[i].RawValue == rawValue {
				mappings[i].Label = value
				updated = true
				break
			}
		}
		if !updated {
			return fmt.Errorf("enum value %q not found on dimension %s", rawValue, dimID)
		}
		return sem.ReplaceEnumMappings(ctx, model.ID, dimID, mappings)
	default:
		return fmt.Errorf("gap %q is not applyable", gapID)
	}
}
