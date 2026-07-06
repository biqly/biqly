package ambiguity

import "testing"

func tieResult() Result {
	return Result{
		IsAmbiguous: true,
		Ambiguities: []Item{
			{
				Term: "ciro",
				Type: "semantic",
				Interpretations: []Interpretation{
					{Label: "gross_revenue", SemanticMapping: SemanticMapping{Type: "metric", Name: "gross_revenue"}, Confidence: 1},
					{Label: "net_revenue", SemanticMapping: SemanticMapping{Type: "metric", Name: "net_revenue"}, Confidence: 1},
				},
			},
		},
	}
}

func clearBestResult() Result {
	return Result{
		IsAmbiguous: true,
		Ambiguities: []Item{
			{
				Term: "revenue",
				Type: "semantic",
				Interpretations: []Interpretation{
					{Label: "net_revenue", SemanticMapping: SemanticMapping{Type: "metric", Name: "net_revenue"}, Confidence: 1},
					{Label: "gross_revenue", SemanticMapping: SemanticMapping{Type: "metric", Name: "gross_revenue"}, Confidence: 0.75},
				},
			},
		},
	}
}

func TestDecideClarifiesGenuineTie(t *testing.T) {
	decision := Decide(tieResult(), ClarificationTieEpsilon, false)
	if !decision.Clarify {
		t.Fatalf("Decide() Clarify = false, want true for equal-confidence tie")
	}
	if len(decision.Chosen) != 0 {
		t.Errorf("Decide() Chosen = %v, want empty when clarifying", decision.Chosen)
	}
}

func TestDecideProceedsWhenClearBest(t *testing.T) {
	decision := Decide(clearBestResult(), ClarificationTieEpsilon, false)
	if decision.Clarify {
		t.Fatalf("Decide() Clarify = true, want false when one interpretation is materially ahead")
	}
	if len(decision.Chosen) != 1 || decision.Chosen[0].Interpretation.SemanticMapping.Name != "net_revenue" {
		t.Errorf("Decide() Chosen = %+v, want the top-confidence net_revenue", decision.Chosen)
	}
}

func TestDecideForceProceedsOnTie(t *testing.T) {
	decision := Decide(tieResult(), ClarificationTieEpsilon, true)
	if decision.Clarify {
		t.Fatalf("Decide() Clarify = true under force, want false (skip never dead-ends)")
	}
	if len(decision.Chosen) != 1 {
		t.Fatalf("Decide() Chosen = %v, want one default under force", decision.Chosen)
	}
}

func TestDecideNotAmbiguous(t *testing.T) {
	if Decide(Result{}, ClarificationTieEpsilon, true).Clarify {
		t.Error("Decide() on non-ambiguous result should not clarify")
	}
}

func TestApplyDefaultsRewritesQuestion(t *testing.T) {
	decision := Decide(clearBestResult(), ClarificationTieEpsilon, false)
	got := ApplyDefaults("show me revenue", decision.Chosen)
	want := "show me net_revenue"
	if got != want {
		t.Errorf("ApplyDefaults() = %q, want %q", got, want)
	}
}

func TestApplyDefaultsSkipsAbsentTerm(t *testing.T) {
	chosen := []ChosenDefault{{Term: "revenue", Interpretation: Interpretation{Label: "net_revenue"}}}
	got := ApplyDefaults("show me sales", chosen)
	if got != "show me sales" {
		t.Errorf("ApplyDefaults() = %q, want the question unchanged when the term is absent", got)
	}
}
