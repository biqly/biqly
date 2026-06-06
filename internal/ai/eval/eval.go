package eval

import (
	"fmt"
	"sort"

	"github.com/biqly/biqly/internal/query"
	"github.com/biqly/biqly/internal/semantic"
)

// GoldenCase is one entry in the text-to-SQL evaluation set: a natural-language
// question paired with the canonical LogicalQuery a correct system would emit
// against the given semantic model.
type GoldenCase struct {
	ID       string
	Question string
	Model    *semantic.SemanticModel
	Expected query.LogicalQuery
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
		out = append(out, it.Type+":"+it.Name)
	}
	return out
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
		ka[i] = a[i].Field
	}
	for i := range b {
		kb[i] = b[i].Field
	}
	sort.Strings(ka)
	sort.Strings(kb)
	return equalStrings(ka, kb)
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
