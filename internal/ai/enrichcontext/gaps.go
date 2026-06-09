package enrichcontext

import (
	"fmt"
	"sort"
	"strings"

	"github.com/biqly/biqly/internal/metadata"
	"github.com/biqly/biqly/internal/semantic"
)

type columnKey struct {
	schema string
	table  string
	column string
}

func columnKeyFromRef(ref string) (columnKey, bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return columnKey{}, false
	}
	parts := strings.Split(ref, ".")
	switch len(parts) {
	case 2:
		return columnKey{table: parts[0], column: parts[1]}, true
	case 3:
		return columnKey{schema: parts[0], table: parts[1], column: parts[2]}, true
	default:
		return columnKey{}, false
	}
}

func columnIndexKey(k columnKey) string {
	return strings.ToLower(k.schema) + "|" + strings.ToLower(k.table) + "|" + strings.ToLower(k.column)
}

func isBlank(s string) bool {
	return strings.TrimSpace(s) == ""
}

func isBlankPtr(s *string) bool {
	return s == nil || isBlank(*s)
}

type synonymTarget struct {
	kind string
	name string
	from string
}

func detectGaps(model *semantic.SemanticModel, glossary []metadata.BusinessGlossaryRow, columns []metadata.Column) []Gap {
	if model == nil {
		return nil
	}
	colByKey := make(map[string]metadata.Column, len(columns))
	for _, c := range columns {
		k := columnKey{schema: c.SchemaName, table: c.TableName, column: c.ColumnName}
		colByKey[columnIndexKey(k)] = c
	}

	var gaps []Gap
	seenColumn := make(map[string]struct{})

	for _, d := range model.Dimensions {
		if isBlankPtr(d.Description) {
			gaps = append(gaps, Gap{
				ID:      "dimension:" + d.ID,
				Kind:    GapDimensionMissingDescription,
				Summary: fmt.Sprintf("Dimension %q has no description", d.Name),
				Entity: map[string]string{
					"dimension_id":   d.ID,
					"dimension_name": d.Name,
					"column_ref":     d.ColumnRef,
				},
				Applyable: true,
			})
		}
		if k, ok := columnKeyFromRef(d.ColumnRef); ok {
			key := columnIndexKey(k)
			if _, dup := seenColumn[key]; !dup {
				seenColumn[key] = struct{}{}
				if col, found := colByKey[key]; found && isBlankPtr(col.Description) {
					gaps = append(gaps, Gap{
						ID:      "column:" + col.ID,
						Kind:    GapColumnMissingDescription,
						Summary: fmt.Sprintf("Column %s.%s has no description", col.TableName, col.ColumnName),
						Detail:  d.ColumnRef,
						Entity: map[string]string{
							"column_id":   col.ID,
							"schema":      col.SchemaName,
							"table":       col.TableName,
							"column_name": col.ColumnName,
						},
						Applyable: true,
					})
				}
			}
		}
		for _, ev := range d.EnumValues {
			if isBlank(ev.Label) {
				gaps = append(gaps, Gap{
					ID:      fmt.Sprintf("enum:%s:%s", d.ID, ev.RawValue),
					Kind:    GapEnumMissingLabel,
					Summary: fmt.Sprintf("Enum value %q on dimension %q has no label", ev.RawValue, d.Name),
					Entity: map[string]string{
						"dimension_id":   d.ID,
						"dimension_name": d.Name,
						"raw_value":      ev.RawValue,
					},
					Applyable: true,
				})
			}
		}
	}

	for _, m := range model.Metrics {
		if isBlankPtr(m.Description) {
			gaps = append(gaps, Gap{
				ID:      "metric:" + m.ID,
				Kind:    GapMetricMissingDescription,
				Summary: fmt.Sprintf("Metric %q has no description", m.Name),
				Entity: map[string]string{
					"metric_id":   m.ID,
					"metric_name": m.Name,
					"expression":  m.Expression,
				},
				Applyable: true,
			})
		}
	}

	for _, row := range glossary {
		if isBlank(row.Definition) {
			gaps = append(gaps, Gap{
				ID:      "glossary:" + row.ID,
				Kind:    GapGlossaryMissingDefinition,
				Summary: fmt.Sprintf("Glossary term %q has no definition", row.Term),
				Entity: map[string]string{
					"glossary_id":  row.ID,
					"term":         row.Term,
					"maps_to_type": row.MapsToType,
					"maps_to_name": row.MapsToName,
				},
				Applyable: true,
			})
		}
	}

	gaps = append(gaps, detectGlossaryCollisions(glossary)...)
	gaps = append(gaps, detectModelSynonymCollisions(model)...)

	return sortGaps(gaps)
}

type glossaryCollisionTarget struct {
	mapsToType string
	mapsToName string
	sourceTerm string
}

func detectGlossaryCollisions(rows []metadata.BusinessGlossaryRow) []Gap {
	byTerm := make(map[string][]glossaryCollisionTarget)
	for _, row := range rows {
		terms := append([]string{row.Term}, row.Aliases...)
		for _, t := range terms {
			norm := strings.ToLower(strings.TrimSpace(t))
			if norm == "" {
				continue
			}
			byTerm[norm] = append(byTerm[norm], glossaryCollisionTarget{
				mapsToType: row.MapsToType,
				mapsToName: row.MapsToName,
				sourceTerm: row.Term,
			})
		}
	}

	terms := make([]string, 0, len(byTerm))
	for term := range byTerm {
		terms = append(terms, term)
	}
	sort.Strings(terms)

	var gaps []Gap
	for _, term := range terms {
		targets := uniqueGlossaryTargets(byTerm[term])
		if len(targets) < 2 {
			continue
		}
		var detailParts []string
		for _, t := range targets {
			detailParts = append(detailParts, fmt.Sprintf("%s → %s/%s", t.sourceTerm, t.mapsToType, t.mapsToName))
		}
		gaps = append(gaps, Gap{
			ID:        "collision:glossary:" + term,
			Kind:      GapSynonymCollision,
			Summary:   fmt.Sprintf("Glossary term %q maps to multiple targets", term),
			Detail:    strings.Join(detailParts, "; "),
			Entity:    map[string]string{"term": term},
			Applyable: false,
		})
	}
	return gaps
}

func uniqueGlossaryTargets(targets []glossaryCollisionTarget) []glossaryCollisionTarget {
	seen := make(map[string]struct{})
	var out []glossaryCollisionTarget
	for _, t := range targets {
		key := strings.ToLower(t.mapsToType) + "|" + strings.ToLower(t.mapsToName)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, t)
	}
	return out
}

func detectModelSynonymCollisions(model *semantic.SemanticModel) []Gap {
	bySynonym := make(map[string][]synonymTarget)
	add := func(synonym, kind, name, from string) {
		synonym = strings.ToLower(strings.TrimSpace(synonym))
		if synonym == "" {
			return
		}
		bySynonym[synonym] = append(bySynonym[synonym], synonymTarget{kind: kind, name: name, from: from})
	}
	for _, d := range model.Dimensions {
		add(d.Name, "dimension", d.Name, "dimension:"+d.Name)
		for _, s := range d.Synonyms {
			add(s, "dimension", d.Name, "dimension:"+d.Name)
		}
	}
	for _, m := range model.Metrics {
		add(m.Name, "metric", m.Name, "metric:"+m.Name)
		for _, s := range m.Synonyms {
			add(s, "metric", m.Name, "metric:"+m.Name)
		}
	}

	synonyms := make([]string, 0, len(bySynonym))
	for s := range bySynonym {
		synonyms = append(synonyms, s)
	}
	sort.Strings(synonyms)

	var gaps []Gap
	for _, syn := range synonyms {
		targets := uniqueSynonymTargets(bySynonym[syn])
		if len(targets) < 2 {
			continue
		}
		var detailParts []string
		for _, t := range targets {
			detailParts = append(detailParts, fmt.Sprintf("%s/%s (%s)", t.kind, t.name, t.from))
		}
		gaps = append(gaps, Gap{
			ID:        "collision:model:" + syn,
			Kind:      GapSynonymCollision,
			Summary:   fmt.Sprintf("Synonym %q matches multiple semantic fields", syn),
			Detail:    strings.Join(detailParts, "; "),
			Entity:    map[string]string{"synonym": syn},
			Applyable: false,
		})
	}
	return gaps
}

func uniqueSynonymTargets(targets []synonymTarget) []synonymTarget {
	seen := make(map[string]struct{})
	var out []synonymTarget
	for _, t := range targets {
		key := strings.ToLower(t.kind) + "|" + strings.ToLower(t.name)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, t)
	}
	return out
}

func sortGaps(gaps []Gap) []Gap {
	sort.Slice(gaps, func(i, j int) bool {
		if gaps[i].Kind != gaps[j].Kind {
			return gaps[i].Kind < gaps[j].Kind
		}
		return gaps[i].ID < gaps[j].ID
	})
	return gaps
}
