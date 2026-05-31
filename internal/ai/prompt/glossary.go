package prompt

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/biqly/biqly/internal/ai/routing"
	"github.com/biqly/biqly/internal/semantic"
)

const (
	maxGlossaryPromptEntries = 25
	maxGlossaryDefRunes      = 160
)

// GlossaryEntry maps a business phrase to a semantic field for prompt injection.
type GlossaryEntry struct {
	Term       string
	Definition string
	MapsToType string // dimension | metric | model
	MapsToName string
	Source     string // catalog | glossary
}

// ExternalGlossaryInput is a curated term from the business_glossary_terms table.
type ExternalGlossaryInput struct {
	Term       string
	Definition string
	MapsToType string
	MapsToName string
	Aliases    []string
}

// GlossaryFromSemanticModel derives term→field mappings from model/dimension/metric synonyms.
func GlossaryFromSemanticModel(model *semantic.SemanticModel) []GlossaryEntry {
	if model == nil {
		return nil
	}
	seen := make(map[string]struct{})
	var out []GlossaryEntry

	add := func(term, def, kind, name string) {
		term = strings.TrimSpace(term)
		if term == "" || name == "" {
			return
		}
		key := strings.ToLower(kind) + "|" + strings.ToLower(name) + "|" + strings.ToLower(term)
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		out = append(out, GlossaryEntry{
			Term:       term,
			Definition: def,
			MapsToType: kind,
			MapsToName: name,
			Source:     "catalog",
		})
	}

	for _, syn := range model.Synonyms {
		add(syn, "semantic model alias", "model", model.Name)
	}
	if model.Label != nil {
		add(*model.Label, "model label", "model", model.Name)
	}

	for _, d := range model.Dimensions {
		def := dimensionGlossaryDef(d)
		add(d.Name, def, "dimension", d.Name)
		if d.Label != nil {
			add(*d.Label, def, "dimension", d.Name)
		}
		for _, syn := range d.Synonyms {
			add(syn, def, "dimension", d.Name)
		}
	}

	for _, m := range model.Metrics {
		def := metricGlossaryDef(m)
		add(m.Name, def, "metric", m.Name)
		if m.Label != nil {
			add(*m.Label, def, "metric", m.Name)
		}
		for _, syn := range m.Synonyms {
			add(syn, def, "metric", m.Name)
		}
	}

	return out
}

func dimensionGlossaryDef(d semantic.Dimension) string {
	if d.Description != nil && strings.TrimSpace(*d.Description) != "" {
		return truncateRunes(strings.TrimSpace(*d.Description), maxGlossaryDefRunes)
	}
	return fmt.Sprintf("dimension (%s) → %s", d.Type, d.ColumnRef)
}

func metricGlossaryDef(m semantic.Metric) string {
	if m.Description != nil && strings.TrimSpace(*m.Description) != "" {
		return truncateRunes(strings.TrimSpace(*m.Description), maxGlossaryDefRunes)
	}
	return fmt.Sprintf("metric %s on %s", m.Aggregation, m.Expression)
}

// GlossaryFromExternal converts DB rows into glossary entries (source=glossary).
func GlossaryFromExternal(rows []ExternalGlossaryInput) []GlossaryEntry {
	out := make([]GlossaryEntry, 0, len(rows)*2)
	for _, row := range rows {
		def := strings.TrimSpace(row.Definition)
		addTerm := func(term string) {
			term = strings.TrimSpace(term)
			if term == "" {
				return
			}
			out = append(out, GlossaryEntry{
				Term:       term,
				Definition: def,
				MapsToType: row.MapsToType,
				MapsToName: row.MapsToName,
				Source:     "glossary",
			})
		}
		addTerm(row.Term)
		for _, a := range row.Aliases {
			addTerm(a)
		}
	}
	return out
}

// MergeGlossaryEntries combines catalog and external entries; external wins on duplicate terms.
func MergeGlossaryEntries(catalog, external []GlossaryEntry) []GlossaryEntry {
	byTerm := make(map[string]GlossaryEntry, len(catalog)+len(external))
	for _, e := range catalog {
		byTerm[strings.ToLower(e.Term)] = e
	}
	for _, e := range external {
		byTerm[strings.ToLower(e.Term)] = e
	}
	out := make([]GlossaryEntry, 0, len(byTerm))
	for _, e := range byTerm {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Term) < strings.ToLower(out[j].Term)
	})
	return out
}

// SelectGlossaryForQuestion returns entries most relevant to the user question.
// When nothing matches, returns a compact default slice (display dimensions + row_count).
func SelectGlossaryForQuestion(question string, entries []GlossaryEntry, model *semantic.SemanticModel) []GlossaryEntry {
	if len(entries) == 0 {
		return nil
	}
	qTokens := routing.TokenSet(question)
	type scored struct {
		e     GlossaryEntry
		score float64
	}
	var ranked []scored
	for _, e := range entries {
		sc := glossaryMatchScore(qTokens, e.Term)
		if sc <= 0 {
			continue
		}
		boost := 1.0
		if e.Source == "glossary" {
			boost = 1.25
		}
		ranked = append(ranked, scored{e: e, score: sc * boost})
	}
	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].score != ranked[j].score {
			return ranked[i].score > ranked[j].score
		}
		return ranked[i].e.Term < ranked[j].e.Term
	})

	if len(ranked) > 0 {
		n := maxGlossaryPromptEntries
		if len(ranked) < n {
			n = len(ranked)
		}
		out := make([]GlossaryEntry, n)
		for i := 0; i < n; i++ {
			out[i] = ranked[i].e
		}
		return out
	}

	return defaultGlossarySlice(entries, model)
}

func glossaryMatchScore(qTokens map[string]bool, term string) float64 {
	if len(qTokens) == 0 {
		return 0
	}
	tTokens := routing.TokenSet(term)
	if len(tTokens) == 0 {
		return 0
	}
	var hits float64
	for t := range tTokens {
		if qTokens[t] {
			hits++
		}
	}
	if hits == 0 {
		return 0
	}
	return hits / float64(len(tTokens))
}

func defaultGlossarySlice(entries []GlossaryEntry, model *semantic.SemanticModel) []GlossaryEntry {
	priority := make(map[string]int)
	if model != nil {
		for _, d := range model.Dimensions {
			if d.IsDisplay {
				priority["dimension|"+strings.ToLower(d.Name)] = 3
			}
		}
		for _, m := range model.Metrics {
			if m.Name == "row_count" {
				priority["metric|row_count"] = 4
			}
		}
	}

	type scored struct {
		e GlossaryEntry
		p int
	}
	pool := make([]scored, 0, len(entries))
	for _, e := range entries {
		p := 1
		if e.Source == "glossary" {
			p = 2
		}
		if v, ok := priority[strings.ToLower(e.MapsToType)+"|"+strings.ToLower(e.MapsToName)]; ok {
			p = v
		}
		pool = append(pool, scored{e: e, p: p})
	}
	sort.Slice(pool, func(i, j int) bool {
		if pool[i].p != pool[j].p {
			return pool[i].p > pool[j].p
		}
		return pool[i].e.Term < pool[j].e.Term
	})
	n := 12
	if len(pool) < n {
		n = len(pool)
	}
	out := make([]GlossaryEntry, n)
	for i := 0; i < n; i++ {
		out[i] = pool[i].e
	}
	return out
}

func (b *PromptBuilder) writeBusinessGlossary(sb *bytes.Buffer, entries []GlossaryEntry) {
	if len(entries) == 0 {
		return
	}
	sb.WriteString("\n\n## Business Glossary\n")
	sb.WriteString("Map business language in the question to exact catalog names. Prefer **glossary** rows over guessing.\n\n")
	for _, e := range entries {
		def := ""
		if e.Definition != "" {
			def = fmt.Sprintf(" — %s", e.Definition)
		}
		cur := ""
		if e.Source == "glossary" {
			cur = " [curated]"
		}
		fmt.Fprintf(sb, "- **%s** → `%s` (%s)%s%s\n", e.Term, e.MapsToName, e.MapsToType, def, cur)
	}
	sb.WriteString("\n")
}

func truncateRunes(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	return string(runes[:max]) + "…"
}

// TruncateRunes shortens s to at most max runes, appending an ellipsis when truncated.
func TruncateRunes(s string, max int) string {
	return truncateRunes(s, max)
}
