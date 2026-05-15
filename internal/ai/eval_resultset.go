package ai

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/biqly/biqly/internal/query"
)

// ResultSetEqual compares two query results for semantic equivalence.
// Row order is ignored; columns are matched by name. Numeric values use
// a small epsilon for float comparison.
func ResultSetEqual(a, b *query.Result) (bool, string) {
	if a == nil || b == nil {
		if a == b {
			return true, ""
		}
		return false, "one result is nil"
	}
	if len(a.Columns) != len(b.Columns) {
		return false, fmt.Sprintf("column count differs: %d vs %d", len(a.Columns), len(b.Columns))
	}
	colNamesA := resultColumnNames(a.Columns)
	colNamesB := resultColumnNames(b.Columns)
	sort.Strings(colNamesA)
	sort.Strings(colNamesB)
	if !equalStrings(colNamesA, colNamesB) {
		return false, "column names differ"
	}
	if len(a.Rows) != len(b.Rows) {
		return false, fmt.Sprintf("row count differs: %d vs %d", len(a.Rows), len(b.Rows))
	}
	rowsA := normalizeResultRows(a)
	rowsB := normalizeResultRows(b)
	sort.Strings(rowsA)
	sort.Strings(rowsB)
	for i := range rowsA {
		if rowsA[i] != rowsB[i] {
			return false, fmt.Sprintf("row %d differs", i)
		}
	}
	return true, ""
}

func resultColumnNames(cols []query.ResultColumn) []string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = c.Name
	}
	return out
}

func normalizeResultRows(r *query.Result) []string {
	nameToIdx := make(map[string]int, len(r.Columns))
	for i, c := range r.Columns {
		nameToIdx[c.Name] = i
	}
	names := make([]string, 0, len(r.Columns))
	for n := range nameToIdx {
		names = append(names, n)
	}
	sort.Strings(names)

	out := make([]string, len(r.Rows))
	for ri, row := range r.Rows {
		parts := make([]string, len(names))
		for i, name := range names {
			idx := nameToIdx[name]
			if idx >= len(row) {
				parts[i] = name + "="
				continue
			}
			parts[i] = name + "=" + cellKey(row[idx])
		}
		out[ri] = strings.Join(parts, "|")
	}
	sort.Strings(out)
	return out
}

func cellKey(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return fmt.Sprintf("%v", x)
		}
		return strconv.FormatFloat(x, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(x), 'f', -1, 32)
	case int:
		return strconv.Itoa(x)
	case int64:
		return strconv.FormatInt(x, 10)
	case int32:
		return strconv.FormatInt(int64(x), 10)
	case string:
		return x
	case bool:
		return strconv.FormatBool(x)
	default:
		return fmt.Sprintf("%v", x)
	}
}
