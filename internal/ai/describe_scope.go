package ai

import (
	"sort"
	"strings"
)

func DescribeBatchScopeSchemas(tables []DescribeBatchTable) []string {
	seen := make(map[string]struct{}, len(tables))
	for _, t := range tables {
		s := strings.TrimSpace(t.Schema)
		if s == "" {
			continue
		}
		seen[s] = struct{}{}
	}
	if len(seen) == 0 {
		return nil
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func SchemasOverlap(a, b []string) bool {
	if len(a) == 0 || len(b) == 0 {
		return false
	}
	set := make(map[string]struct{}, len(a))
	for _, s := range a {
		set[s] = struct{}{}
	}
	for _, s := range b {
		if _, ok := set[s]; ok {
			return true
		}
	}
	return false
}

type DescribeBatchJobProgress struct {
	Total          int      `json:"total"`
	Index          int      `json:"index"`
	CurrentSchema  string   `json:"current_schema,omitempty"`
	CurrentTable   string   `json:"current_table,omitempty"`
	Completed      []string `json:"completed,omitempty"`
	PendingPreview []string `json:"pending_preview,omitempty"`
}

func DescribeBatchTableKey(schema, table string) string {
	return strings.TrimSpace(schema) + "." + strings.TrimSpace(table)
}
