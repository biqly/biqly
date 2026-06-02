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
