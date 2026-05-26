package handlers

import (
	"testing"

	pkgmetadata "github.com/biqly/biqly/pkg/metadata"
	pkgquery "github.com/biqly/biqly/pkg/query"
)

func ptrString(s string) *string { return &s }

func sampleRows() []pkgmetadata.AIQueryHistoryEntry {
	return []pkgmetadata.AIQueryHistoryEntry{
		{ID: "1", UserID: ptrString("alice"), Question: "q1", AIResponse: map[string]string{"sql": "select 1"}},
		{ID: "2", UserID: ptrString("bob"), Question: "q2", AIResponse: map[string]string{"sql": "select 2"}},
		{ID: "3", UserID: ptrString("alice"), Question: "q3", AIResponse: map[string]string{"sql": "select 3"}},
		{ID: "4", UserID: nil, Question: "q-legacy"},
	}
}

func TestFilterAIHistory_AdminSeesAll(t *testing.T) {
	rows := sampleRows()
	got := FilterAIHistoryForUser(rows, "alice", []string{PermissionAIViewDetails})
	if len(got) != 4 {
		t.Fatalf("admin should see all rows, got %d", len(got))
	}
}

func TestFilterAIHistory_OwnerOnlyForNonAdmin(t *testing.T) {
	rows := sampleRows()
	got := FilterAIHistoryForUser(rows, "alice", []string{PermissionAIViewStatus})
	if len(got) != 2 {
		t.Fatalf("alice should see 2 rows, got %d", len(got))
	}
	for _, r := range got {
		if r.UserID == nil || *r.UserID != "alice" {
			t.Fatalf("non-owner row leaked: %+v", r)
		}
	}
}

func TestFilterAIHistory_LegacyEmptyUserPassThrough(t *testing.T) {
	rows := sampleRows()
	got := FilterAIHistoryForUser(rows, "", nil)
	if len(got) != 4 {
		t.Fatalf("legacy API-key mode should pass through all rows, got %d", len(got))
	}
}

func TestFilterAIHistory_OtherUserSeesNoneWhenNoDataExists(t *testing.T) {
	rows := sampleRows()
	got := FilterAIHistoryForUser(rows, "eve", []string{})
	if len(got) != 0 {
		t.Fatalf("eve should see no rows, got %d", len(got))
	}
}

func TestFilterAIHistory_PreservesOriginalOrder(t *testing.T) {
	rows := sampleRows()
	got := FilterAIHistoryForUser(rows, "alice", []string{})
	if len(got) != 2 || got[0].ID != "1" || got[1].ID != "3" {
		t.Fatalf("order not preserved: %+v", got)
	}
}

func TestFilterAIHistoryByDatasources_NilPassThrough(t *testing.T) {
	rows := []pkgmetadata.AIQueryHistoryEntry{{ID: "1", DatasourceID: "a"}, {ID: "2", DatasourceID: "b"}}
	got := FilterAIHistoryByDatasources(rows, nil)
	if len(got) != 2 {
		t.Fatalf("nil filter must pass through, got %d", len(got))
	}
}

func TestFilterAIHistoryByDatasources_KeepsAllowedOnly(t *testing.T) {
	rows := []pkgmetadata.AIQueryHistoryEntry{
		{ID: "1", DatasourceID: "a"},
		{ID: "2", DatasourceID: "b"},
		{ID: "3", DatasourceID: "c"},
	}
	got := FilterAIHistoryByDatasources(rows, map[string]struct{}{"a": {}, "c": {}})
	if len(got) != 2 || got[0].ID != "1" || got[1].ID != "3" {
		t.Fatalf("unexpected filtered rows: %+v", got)
	}
}

func TestFilterAIHistoryByDatasources_EmptyFilterDropsAll(t *testing.T) {
	rows := []pkgmetadata.AIQueryHistoryEntry{{ID: "1", DatasourceID: "a"}}
	got := FilterAIHistoryByDatasources(rows, map[string]struct{}{})
	if len(got) != 0 {
		t.Fatalf("empty filter must drop all, got %d", len(got))
	}
}

func TestFilterQueryHistoryByDatasources(t *testing.T) {
	rows := []pkgquery.HistoryEntry{
		{ID: "1", DatasourceID: "a"},
		{ID: "2", DatasourceID: "b"},
	}
	got := FilterQueryHistoryByDatasources(rows, map[string]struct{}{"b": {}})
	if len(got) != 1 || got[0].ID != "2" {
		t.Fatalf("unexpected filtered rows: %+v", got)
	}

	if pass := FilterQueryHistoryByDatasources(rows, nil); len(pass) != 2 {
		t.Fatalf("nil filter must pass through, got %d", len(pass))
	}
}

func TestMaskAIHistoryRow_NilSafe(t *testing.T) {
	MaskAIHistoryRow(nil) // must not panic
}

func TestMaskAIHistoryRow_ZeroesSensitiveFields(t *testing.T) {
	row := pkgmetadata.AIQueryHistoryEntry{
		ID:            "1",
		UserID:        ptrString("alice"),
		Question:      "secret question",
		PromptContext: map[string]any{"k": "v"},
		AIResponse:    map[string]any{"sql": "select 1"},
		LogicalQuery:  map[string]any{"select": []any{"id"}},
		Warnings:      []string{"w1"},
	}
	MaskAIHistoryRow(&row)
	if row.Question != "" {
		t.Fatalf("question not masked: %q", row.Question)
	}
	if row.PromptContext != nil {
		t.Fatalf("prompt_context not masked: %+v", row.PromptContext)
	}
	if row.AIResponse != nil {
		t.Fatalf("ai_response not masked: %+v", row.AIResponse)
	}
	if row.LogicalQuery != nil {
		t.Fatalf("logical_query not masked: %+v", row.LogicalQuery)
	}
	if row.Warnings != nil {
		t.Fatalf("warnings not masked: %+v", row.Warnings)
	}
	if row.ID != "1" || row.UserID == nil || *row.UserID != "alice" {
		t.Fatalf("non-sensitive fields should remain: %+v", row)
	}
}
