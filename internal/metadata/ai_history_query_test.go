package metadata

import "testing"

func TestBuildAIHistoryWhere(t *testing.T) {
	where, args := buildAIHistoryWhere(AIHistoryListFilter{
		UserID:       "u-1",
		DatasourceID: "ds-1",
		ModelID:      "m-1",
		Status:       "success",
		Search:       "tweet",
	})
	if where == "" {
		t.Fatal("expected non-empty where clause")
	}
	if len(args) != 4 {
		t.Fatalf("expected 4 args, got %d", len(args))
	}
}

func TestAIHistoryListFilter_offset(t *testing.T) {
	f := AIHistoryListFilter{Page: 3, PageSize: 10}
	if got := f.offset(); got != 20 {
		t.Fatalf("offset = %d, want 20", got)
	}
}
