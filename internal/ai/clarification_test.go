package ai

import (
	"testing"

	"github.com/biqly/biqly/internal/ai/ambiguity"
	"github.com/biqly/biqly/internal/ai/routing"
	"github.com/biqly/biqly/internal/i18n"
)

func TestBuildClarificationReturnsNilOnEmptyQuestion(t *testing.T) {
	if got := buildClarification("", "", "ai"); got != nil {
		t.Errorf("expected nil for empty question, got %+v", got)
	}
}

func TestBuildClarificationPopulatesEnvelope(t *testing.T) {
	got := buildClarification("Did you mean gross or net revenue?", "ambiguous metric", "ai")
	if got == nil {
		t.Fatal("expected envelope, got nil")
		return
	}
	if got.Status != ClarificationStatusNeeded {
		t.Errorf("status = %q, want %q", got.Status, ClarificationStatusNeeded)
	}
	if got.Question != "Did you mean gross or net revenue?" {
		t.Errorf("question = %q", got.Question)
	}
	if got.Reason != "ambiguous metric" {
		t.Errorf("reason = %q", got.Reason)
	}
	if got.Source != "ai" {
		t.Errorf("source = %q", got.Source)
	}
}

func TestClarificationFromRoutingNilWhenNotNeeded(t *testing.T) {
	if got := ClarificationFromRouting(nil, ""); got != nil {
		t.Errorf("nil routing should yield nil envelope, got %+v", got)
	}
	result := &routing.TableRoutingResult{NeedsClarification: false, Candidates: []routing.TableCandidate{{Table: "x"}}}
	if got := ClarificationFromRouting(result, ""); got != nil {
		t.Errorf("clear routing should yield nil envelope, got %+v", got)
	}
	empty := &routing.TableRoutingResult{NeedsClarification: true}
	if got := ClarificationFromRouting(empty, ""); got != nil {
		t.Errorf("no candidates should yield nil envelope, got %+v", got)
	}
}

func TestClarificationFromRoutingBuildsOptionsAndCandidates(t *testing.T) {
	result := &routing.TableRoutingResult{
		NeedsClarification: true,
		Candidates: []routing.TableCandidate{
			{Table: "public.orders", Score: 0.62, Description: "Customer orders"},
			{Table: "public.sales", Score: 0.58, Description: "Sales transactions"},
		},
	}
	c := ClarificationFromRouting(result, "")
	if c == nil {
		t.Fatal("expected envelope, got nil")
		return
	}
	if c.Source != "router" {
		t.Errorf("source = %q, want router", c.Source)
	}
	if len(c.Options) != 2 || c.Options[0].Key != "public.orders" {
		t.Errorf("options not built: %+v", c.Options)
	}
	if len(c.Candidates) != 2 || c.Candidates[0].Score != 0.62 {
		t.Errorf("candidates not built: %+v", c.Candidates)
	}
}

func TestClarificationFromAmbiguityBuildsOptionsAndDetail(t *testing.T) {
	result := ambiguity.Result{
		IsAmbiguous: true,
		Ambiguities: []ambiguity.Item{
			{
				Term: "ciro",
				Type: "semantic",
				Interpretations: []ambiguity.Interpretation{
					{
						Label:       "Brüt gelir",
						Description: "İndirim öncesi gelir",
						SemanticMapping: ambiguity.SemanticMapping{
							Type: "metric",
							Name: "gross_revenue",
						},
						Confidence: 0.95,
					},
					{
						Label: "Net gelir",
						SemanticMapping: ambiguity.SemanticMapping{
							Type: "metric",
							Name: "net_revenue",
						},
						Confidence: 0.91,
					},
				},
			},
		},
	}

	got := ClarificationFromAmbiguity(result)
	if got == nil {
		t.Fatal("ClarificationFromAmbiguity() = nil, want envelope")
	}
	if got.Source != "ambiguity_analyzer" {
		t.Errorf("ClarificationFromAmbiguity().Source = %q, want %q", got.Source, "ambiguity_analyzer")
	}
	if len(got.Options) != 2 || got.Options[0].Key != "ambiguity:0:0" || got.Options[0].Hint != "İndirim öncesi gelir" {
		t.Errorf("ClarificationFromAmbiguity().Options = %+v, want analyzer options", got.Options)
	}
	if got.AmbiguityDetail == nil || len(got.AmbiguityDetail.Ambiguities) != 1 {
		t.Errorf("ClarificationFromAmbiguity().AmbiguityDetail = %+v, want one ambiguity", got.AmbiguityDetail)
	}
}

func TestClarificationFromAmbiguityWithMaxOptionsCapsChoices(t *testing.T) {
	result := ambiguity.Result{
		IsAmbiguous: true,
		Ambiguities: []ambiguity.Item{
			{
				Term: "ciro",
				Type: "semantic",
				Interpretations: []ambiguity.Interpretation{
					{Label: "Brüt gelir"},
					{Label: "Net gelir"},
				},
			},
		},
	}

	got := ClarificationFromAmbiguityWithMaxOptions(i18n.LocaleTR, result, 1)
	if got == nil || len(got.Options) != 1 {
		t.Fatalf("ClarificationFromAmbiguityWithMaxOptions() = %+v, want one option", got)
	}
}
