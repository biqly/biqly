package handlers

import (
	"context"
	"testing"

	"github.com/biqly/biqly/internal/semantic"
)

func TestResolveClarificationChoice(t *testing.T) {
	req := &aiQueryRequest{
		Question:            "Ciro göster",
		ClarificationChoice: "ambiguity:0:1",
	}
	model := &semantic.SemanticModel{
		Metrics: []semantic.Metric{
			{Name: "gross_revenue", Synonyms: []string{"ciro"}},
			{Name: "net_revenue", Synonyms: []string{"ciro"}},
		},
	}

	err := resolveClarificationChoice(context.Background(), req, model, nil)
	if err != nil {
		t.Fatalf("resolveClarificationChoice() error = %v, want nil", err)
	}
	if got, want := req.Question, "net_revenue göster"; got != want {
		t.Errorf("resolveClarificationChoice().Question = %q, want %q", got, want)
	}
	if req.ClarificationChoice != "" {
		t.Errorf("resolveClarificationChoice().ClarificationChoice = %q, want empty", req.ClarificationChoice)
	}
}

// TestHandlerResolveClarificationChoiceSetsResolvedFlag guards the async job path
// (executeAIQueryPhase) regression: resolving a clarification choice through the
// shared handler method MUST mark the turn as resolved so standardProcessOptions
// skips the pre-LLM ambiguity check and the flow does not loop re-asking.
func TestHandlerResolveClarificationChoiceSetsResolvedFlag(t *testing.T) {
	h := &AIHandler{}
	req := &aiQueryRequest{
		Question:            "Ciro göster",
		ClarificationChoice: "ambiguity:0:1",
	}
	model := &semantic.SemanticModel{
		Metrics: []semantic.Metric{
			{Name: "gross_revenue", Synonyms: []string{"ciro"}},
			{Name: "net_revenue", Synonyms: []string{"ciro"}},
		},
	}

	if err := h.resolveClarificationChoice(context.Background(), req, model, nil); err != nil {
		t.Fatalf("resolveClarificationChoice() error = %v, want nil", err)
	}
	if !req.clarificationResolved {
		t.Error("clarificationResolved = false, want true after resolving a choice")
	}
}

// TestHandlerResolveClarificationChoiceNoChoiceKeepsFlagUnset ensures turns
// without a clarification choice are not falsely marked resolved.
func TestHandlerResolveClarificationChoiceNoChoiceKeepsFlagUnset(t *testing.T) {
	h := &AIHandler{}
	req := &aiQueryRequest{Question: "Ciro göster"}

	if err := h.resolveClarificationChoice(context.Background(), req, &semantic.SemanticModel{}, nil); err != nil {
		t.Fatalf("resolveClarificationChoice() error = %v, want nil", err)
	}
	if req.clarificationResolved {
		t.Error("clarificationResolved = true, want false when no choice provided")
	}
}
