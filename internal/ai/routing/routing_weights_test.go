package routing

import "testing"

func TestMergePositiveWeightOnlyOverridesWithPositiveValues(t *testing.T) {
	base := 1.5
	mergePositiveWeight(&base, 0)
	if base != 1.5 {
		t.Fatalf("zero override changed base to %v", base)
	}
	mergePositiveWeight(&base, -2)
	if base != 1.5 {
		t.Fatalf("negative override changed base to %v", base)
	}
	mergePositiveWeight(&base, 3)
	if base != 3 {
		t.Fatalf("positive override = %v, want 3", base)
	}
}

func TestWeightedTokenScore(t *testing.T) {
	tokens := tokenSet("show total revenue by country")
	base := weightedTokenScore(tokens, "country revenue report", 1)
	if base <= 0 {
		t.Fatalf("weightedTokenScore matched score = %v, want > 0", base)
	}
	if got := weightedTokenScore(tokens, "country revenue report", 2.5); got != base*2.5 {
		t.Fatalf("weightedTokenScore weighted score = %v, want %v", got, base*2.5)
	}
	if got := weightedTokenScore(tokens, "unrelated table", 2.5); got != 0 {
		t.Fatalf("weightedTokenScore unrelated score = %v, want 0", got)
	}
	if got := WeightedTokenScore(tokens, "country revenue report", 1); got != base {
		t.Fatalf("WeightedTokenScore exported wrapper score = %v, want %v", got, base)
	}
}
