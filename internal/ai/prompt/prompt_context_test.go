package prompt

import (
	"strings"
	"testing"

	"github.com/biqly/biqly/internal/semantic"
)

func TestContextTierForAttempt(t *testing.T) {
	t.Parallel()
	tests := []struct {
		attempt int
		want    int
	}{
		{0, 0},
		{1, 1},
		{2, 2},
		{3, 2},
	}
	for _, tc := range tests {
		t.Run("", func(t *testing.T) {
			if got := ContextTierForAttempt(tc.attempt); got != tc.want {
				t.Errorf("ContextTierForAttempt(%d) = %d, want %d", tc.attempt, got, tc.want)
			}
		})
	}
}

func TestFewShotCap(t *testing.T) {
	t.Parallel()
	if got := FewShotCap(0); got != 2 {
		t.Fatalf("compact = %d, want 2", got)
	}
	if got := FewShotCap(1); got != 5 {
		t.Fatalf("standard = %d, want 5", got)
	}
	if got := FewShotCap(2); got != 12 {
		t.Fatalf("expanded = %d, want 12", got)
	}
	if got := FewShotCap(99); got != 12 {
		t.Fatalf("default = %d, want 12", got)
	}
}

func TestPriorTurnsCap(t *testing.T) {
	t.Parallel()
	if got := PriorTurnsCap(0); got != 2 {
		t.Fatalf("compact = %d, want 2", got)
	}
	if got := PriorTurnsCap(1); got != 5 {
		t.Fatalf("standard = %d, want 5", got)
	}
	if got := PriorTurnsCap(2); got != 8 {
		t.Fatalf("expanded = %d, want 8", got)
	}
}

func TestPriorTurnsTokenBudget(t *testing.T) {
	t.Parallel()
	if got := PriorTurnsTokenBudget(0); got != 150 {
		t.Fatalf("compact = %d, want 150", got)
	}
	if got := PriorTurnsTokenBudget(1); got != 250 {
		t.Fatalf("standard = %d, want 250", got)
	}
	if got := PriorTurnsTokenBudget(2); got != 400 {
		t.Fatalf("expanded = %d, want 400", got)
	}
}

func TestGlossaryCap(t *testing.T) {
	t.Parallel()
	if got := GlossaryCap(0); got != 12 {
		t.Fatalf("compact = %d, want 12", got)
	}
	if got := GlossaryCap(1); got != 25 {
		t.Fatalf("standard = %d, want 25", got)
	}
	if got := GlossaryCap(2); got != 40 {
		t.Fatalf("expanded = %d, want 40", got)
	}
	if got := GlossaryCap(99); got != 40 {
		t.Fatalf("default = %d, want 40", got)
	}
}

func TestTailSliceEmpty(t *testing.T) {
	t.Parallel()
	if got := TailSlice([]string{}, 5); got != nil {
		t.Fatalf("expected nil, got %v", got)
	}
	if got := TailSlice[string](nil, 5); got != nil {
		t.Fatalf("expected nil for nil slice")
	}
}

func TestTailSliceZeroMax(t *testing.T) {
	t.Parallel()
	if got := TailSlice([]string{"a", "b"}, 0); got != nil {
		t.Fatalf("expected nil for maxItems=0")
	}
}

func TestTailSliceFewerThanMax(t *testing.T) {
	t.Parallel()
	items := []int{1, 2, 3}
	got := TailSlice(items, 10)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[0] != 1 || got[2] != 3 {
		t.Fatalf("got %v, want [1 2 3]", got)
	}
}

func TestTailSliceMoreThanMax(t *testing.T) {
	t.Parallel()
	items := []int{1, 2, 3, 4, 5, 6}
	got := TailSlice(items, 3)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[0] != 4 || got[2] != 6 {
		t.Fatalf("got %v, want [4 5 6]", got)
	}
}

func TestTailSliceCopies(t *testing.T) {
	t.Parallel()
	original := []int{1, 2, 3, 4, 5}
	got := TailSlice(original, 3)
	// Modify original to ensure got is a copy
	original[2] = 999
	if got[0] != 3 {
		t.Fatalf("expected copy, got[0]=%d (original was mutated)", got[0])
	}
}

func TestTailGlossaryEmpty(t *testing.T) {
	t.Parallel()
	if got := TailGlossary(nil, 5); got != nil {
		t.Fatal("expected nil")
	}
	if got := TailGlossary([]GlossaryEntry{}, 5); got != nil {
		t.Fatal("expected nil")
	}
}

func TestTailGlossaryZeroMax(t *testing.T) {
	t.Parallel()
	if got := TailGlossary([]GlossaryEntry{{Term: "a"}}, 0); got != nil {
		t.Fatal("expected nil")
	}
}

func TestTailGlossaryFewerThanMax(t *testing.T) {
	t.Parallel()
	entries := []GlossaryEntry{{Term: "a"}, {Term: "b"}}
	got := TailGlossary(entries, 10)
	if len(got) != 2 {
		t.Fatalf("len = %d, want 2", len(got))
	}
}

func TestTailGlossaryMoreThanMax(t *testing.T) {
	t.Parallel()
	entries := []GlossaryEntry{
		{Term: "a"}, {Term: "b"}, {Term: "c"},
		{Term: "d"}, {Term: "e"},
	}
	got := TailGlossary(entries, 3)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[0].Term != "a" || got[2].Term != "c" {
		t.Fatalf("got %v, want first 3 [a b c]", got)
	}
}

func TestTruncateRunesUnderLimit(t *testing.T) {
	t.Parallel()
	if got := TruncateRunes("hello", 100); got != "hello" {
		t.Fatalf("got %q", got)
	}
}

func TestTruncateRunesOverLimit(t *testing.T) {
	t.Parallel()
	got := TruncateRunes("hello world", 5)
	if got != "hello…" {
		t.Fatalf("got %q, want %q", got, "hello…")
	}
}

func TestTruncateRunesUnicode(t *testing.T) {
	t.Parallel()
	got := TruncateRunes("héllo wörld", 6)
	if got != "héllo …" {
		t.Fatalf("got %q", got)
	}
}

func TestTruncateRunesEmpty(t *testing.T) {
	t.Parallel()
	if got := TruncateRunes("", 10); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestDefaultGlossarySliceWithModel(t *testing.T) {
	t.Parallel()
	entries := []GlossaryEntry{
		{Term: "revenue", MapsToName: "revenue", MapsToType: "metric", Source: "catalog"},
		{Term: "customer_name", MapsToName: "customer_name", MapsToType: "dimension", Source: "catalog"},
	}
	model := &semantic.SemanticModel{
		Dimensions: []semantic.Dimension{
			{Name: "customer_name", IsDisplay: true, ColumnRef: "c.name"},
		},
		Metrics: []semantic.Metric{
			{Name: "row_count", Aggregation: "count", Expression: "*"},
		},
	}
	got := defaultGlossarySlice(entries, model)
	if len(got) == 0 {
		t.Fatal("expected at least some entries")
	}
}

func TestDefaultGlossarySliceNilModel(t *testing.T) {
	t.Parallel()
	entries := []GlossaryEntry{
		{Term: "revenue", MapsToName: "revenue", MapsToType: "metric", Source: "catalog"},
	}
	got := defaultGlossarySlice(entries, nil)
	if len(got) == 0 {
		t.Fatal("expected at least some entries")
	}
}

func TestTruncatePriorTurnResultExceeds(t *testing.T) {
	t.Parallel()
	got := truncatePriorTurnResult("hello world this is a test", 10)
	if got != "hello w..." {
		t.Fatalf("got %q", got)
	}
}

func TestTruncatePriorTurnResultZero(t *testing.T) {
	t.Parallel()
	if got := truncatePriorTurnResult("hello", 0); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestTruncatePriorTurnResultNegative(t *testing.T) {
	t.Parallel()
	if got := truncatePriorTurnResult("hello", -1); got != "" {
		t.Fatalf("got %q", got)
	}
}

func TestTruncatePriorTurnResultUnderLimit(t *testing.T) {
	t.Parallel()
	if got := truncatePriorTurnResult("short", 100); got != "short" {
		t.Fatalf("got %q", got)
	}
}

func TestJoinSynonymsCap(t *testing.T) {
	t.Parallel()
	syns := []string{"a", "b", "c", "d", "e", "f", "g"}
	got := joinSynonymsCap(syns, 3)
	if !strings.Contains(got, "a") {
		t.Fatalf("got %q, expected to contain 'a'", got)
	}
	if strings.Contains(got, "d") {
		t.Fatalf("got %q, expected NOT to contain 'd' (capped at 3)", got)
	}
}

func TestJoinSynonymsCapUnder(t *testing.T) {
	t.Parallel()
	syns := []string{"x", "y"}
	got := joinSynonymsCap(syns, 10)
	if got != "x, y" {
		t.Fatalf("got %q", got)
	}
}
