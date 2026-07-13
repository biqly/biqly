package handlers

import (
	"slices"
	"testing"

	"github.com/biqly/biqly/internal/semantic"
)

// TestFilterCompositesByScope verifies that composite listing enforces
// per-datasource access: only composites whose datasource is in the caller's
// allowed set survive. This guards the S2 fix against regressing to the prior
// cross-tenant read behavior.
func TestFilterCompositesByScope(t *testing.T) {
	composites := []semantic.CompositeModel{
		{ID: "c1", DatasourceID: "ds-1"},
		{ID: "c2", DatasourceID: "ds-2"},
		{ID: "c3", DatasourceID: "ds-3"},
		{ID: "c4", DatasourceID: "ds-1"},
	}

	t.Run("keeps only allowed datasources", func(t *testing.T) {
		allowed := map[string]struct{}{"ds-1": {}, "ds-3": {}}
		got := compositeIDs(filterCompositesByScope(composites, allowed))
		want := []string{"c1", "c3", "c4"}
		if !slices.Equal(got, want) {
			t.Errorf("expected %v, got %v", want, got)
		}
	})

	t.Run("empty allowed set yields no composites", func(t *testing.T) {
		got := filterCompositesByScope(composites, map[string]struct{}{})
		if len(got) != 0 {
			t.Errorf("expected no composites for empty scope, got %v", compositeIDs(got))
		}
	})

	t.Run("returns non-nil empty slice", func(t *testing.T) {
		got := filterCompositesByScope(nil, map[string]struct{}{"ds-1": {}})
		if got == nil {
			t.Error("expected non-nil empty slice")
		}
		if len(got) != 0 {
			t.Errorf("expected empty result, got %v", compositeIDs(got))
		}
	})
}

func compositeIDs(composites []semantic.CompositeModel) []string {
	ids := make([]string, len(composites))
	for i, c := range composites {
		ids[i] = c.ID
	}
	return ids
}
