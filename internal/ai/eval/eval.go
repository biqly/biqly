// Package eval implements the text-to-SQL evaluation service.
package eval

import (
	"fmt"
	"sort"
	"strings"

	"github.com/biqly/biqly/internal/ai/prompt"
	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
)

// GoldenCase is one entry in the text-to-SQL evaluation set: a natural-language
// question paired with the canonical LogicalQuery a correct system would emit
// against the given semantic model.
type GoldenCase struct {
	ID          string
	Question    string
	Model       *semantic.SemanticModel
	Samples     []prompt.TableSample
	LogicalOnly bool
	Expected    query.LogicalQuery
}

// Result EvalResult records what the runner observed for one case.
type Result struct {
	Case   GoldenCase
	Got    *query.LogicalQuery
	Match  bool
	Reason string // empty on match; otherwise the first divergence
}

// LogicalQueryEqual compares two LogicalQueries for semantic equivalence.
// Lists where order is irrelevant (filters, group_by, select items of the
// same kind) are sorted before comparison; order_by is order-sensitive
// because it changes ranking semantics.
func LogicalQueryEqual(a, b *query.LogicalQuery) (bool, string) {
	if a == nil || b == nil {
		return a == b, "one query is nil"
	}
	if a.Limit != b.Limit {
		return false, "limit differs"
	}
	if a.Offset != b.Offset {
		return false, "offset differs"
	}
	if !selectsEqual(a.Select, b.Select) {
		return false, "select differs"
	}
	if !filtersEqual(a.Filters, b.Filters) {
		return false, "filters differ"
	}
	if !groupByEqual(a.GroupBy, b.GroupBy) {
		return false, "group_by differs"
	}
	if !filtersEqual(a.Having, b.Having) {
		return false, "having differs"
	}
	if !orderByEqual(a.OrderBy, b.OrderBy) {
		return false, "order_by differs"
	}
	return true, ""
}

func selectsEqual(a, b []query.SelectItem) bool {
	if len(a) != len(b) {
		return false
	}
	ka := selectKeys(a)
	kb := selectKeys(b)
	sort.Strings(ka)
	sort.Strings(kb)
	return equalStrings(ka, kb)
}

func selectKeys(items []query.SelectItem) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		key := it.Type + ":" + it.Name
		if len(it.Filters) > 0 {
			key += "|filters:" + joinedFilterKeys(it.Filters)
		}
		if it.Formula != nil {
			key += "|formula:" + formulaKey(it.Formula)
		}
		if it.Window != nil {
			key += "|window:" + windowKey(it.Window)
		}
		out = append(out, key)
	}
	return out
}

func joinedFilterKeys(filters []query.Filter) string {
	keys := filterKeys(filters)
	sort.Strings(keys)
	return strings.Join(keys, ";")
}

func formulaKey(formula *query.FormulaSpec) string {
	return strings.Join([]string{
		"op=" + formula.Op,
		"left=" + measureRefKey(formula.Left),
		"right=" + measureRefKey(formula.Right),
	}, "|")
}

func measureRefKey(ref query.MeasureRef) string {
	key := "metric=" + ref.Metric
	if len(ref.Filters) > 0 {
		key += ",filters:" + joinedFilterKeys(ref.Filters)
	}
	return key
}

func windowKey(window *query.WindowSpec) string {
	parts := []string{
		"aggregation=" + window.Aggregation,
		"expression=" + window.Expression,
		"metric=" + window.Metric,
	}
	if len(window.PartitionBy) > 0 {
		parts = append(parts, "partition_by="+strings.Join(window.PartitionBy, ","))
	}
	if len(window.OrderBy) > 0 {
		parts = append(parts, "order_by="+orderByKey(window.OrderBy))
	}
	if window.Offset != 0 {
		parts = append(parts, fmt.Sprintf("offset=%d", window.Offset))
	}
	if window.Frame != "" {
		parts = append(parts, "frame="+window.Frame)
	}
	return strings.Join(parts, "|")
}

func orderByKey(orderBy []query.OrderBy) string {
	parts := make([]string, 0, len(orderBy))
	for _, ob := range orderBy {
		parts = append(parts, ob.Field+":"+ob.Direction)
	}
	return strings.Join(parts, ",")
}

func filtersEqual(a, b []query.Filter) bool {
	if len(a) != len(b) {
		return false
	}
	ka := filterKeys(a)
	kb := filterKeys(b)
	sort.Strings(ka)
	sort.Strings(kb)
	return equalStrings(ka, kb)
}

func filterKeys(filters []query.Filter) []string {
	out := make([]string, 0, len(filters))
	for _, f := range filters {
		out = append(out, f.Field+"|"+f.Operator+"|"+fmt.Sprint(f.Value))
	}
	return out
}

func groupByEqual(a, b []query.GroupBy) bool {
	if len(a) != len(b) {
		return false
	}
	ka := make([]string, len(a))
	kb := make([]string, len(b))
	for i := range a {
		ka[i] = groupByKey(a[i])
	}
	for i := range b {
		kb[i] = groupByKey(b[i])
	}
	sort.Strings(ka)
	sort.Strings(kb)
	return equalStrings(ka, kb)
}

func groupByKey(gb query.GroupBy) string {
	if gb.TimeGrain == "" {
		return gb.Field
	}
	return gb.Field + "@" + gb.TimeGrain
}

func orderByEqual(a, b []query.OrderBy) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Field != b[i].Field || a[i].Direction != b[i].Direction {
			return false
		}
	}
	return true
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
