package handlers

import (
	"context"
	"testing"

	"github.com/biqly/biqly/internal/ai"
	"github.com/biqly/biqly/internal/config"
	"github.com/biqly/biqly/internal/semantic"
)

func ambiguityTestModel() *semantic.SemanticModel {
	return &semantic.SemanticModel{
		Metrics: []semantic.Metric{
			{Name: "gross_revenue", Synonyms: []string{"ciro"}},
			{Name: "net_revenue", Synonyms: []string{"ciro"}},
		},
	}
}

func TestProcessContextResolveSetsFlag(t *testing.T) {
	pc := &ProcessContext{
		Question:            "Ciro göster",
		ClarificationChoice: "ambiguity:0:1",
	}
	model := ambiguityTestModel()

	if err := pc.Resolve(context.Background(), model, nil); err != nil {
		t.Fatalf("Resolve() error = %v, want nil", err)
	}
	if got, want := pc.Question, "net_revenue göster"; got != want {
		t.Errorf("Question = %q, want %q", got, want)
	}
	if pc.ClarificationChoice != "" {
		t.Errorf("ClarificationChoice = %q, want empty", pc.ClarificationChoice)
	}
	if !pc.ClarificationResolved {
		t.Error("ClarificationResolved = false, want true after resolving a choice")
	}
}

func TestProcessContextResolveNoChoiceKeepsFlagUnset(t *testing.T) {
	pc := &ProcessContext{Question: "Ciro göster"}

	if err := pc.Resolve(context.Background(), ambiguityTestModel(), nil); err != nil {
		t.Fatalf("Resolve() error = %v, want nil", err)
	}
	if pc.ClarificationResolved {
		t.Error("ClarificationResolved = true, want false when no choice provided")
	}
}

func TestProcessContextShouldCheckAmbiguity(t *testing.T) {
	cfg := config.AmbiguityConfig{CheckEnabled: true}

	unresolved := &ProcessContext{Question: "Ciro göster"}
	if !unresolved.ShouldCheckAmbiguity(cfg) {
		t.Error("ShouldCheckAmbiguity() = false, want true before resolve")
	}

	resolved := &ProcessContext{Question: "net_revenue göster", ClarificationResolved: true}
	if resolved.ShouldCheckAmbiguity(cfg) {
		t.Error("ShouldCheckAmbiguity() = true, want false after resolve")
	}

	disabled := config.AmbiguityConfig{CheckEnabled: false}
	if unresolved.ShouldCheckAmbiguity(disabled) {
		t.Error("ShouldCheckAmbiguity() = true, want false when check disabled")
	}
}

// TestProcessContextSyncAsyncIdenticalBehavior ensures sync and async paths use the
// same buildProcessContext + Resolve + ApplyToRequest sequence (resolveProcessContext
// wraps Resolve with glossary load and metrics only).
func TestProcessContextSyncAsyncIdenticalBehavior(t *testing.T) {
	model := ambiguityTestModel()
	ctx := context.Background()

	syncReq := aiQueryRequest{
		Question:            "Ciro göster",
		ClarificationChoice: "ambiguity:0:1",
		DatasourceID:        "ds-1",
	}
	syncPC := buildProcessContext(syncReq)
	if err := syncPC.Resolve(ctx, model, nil); err != nil {
		t.Fatalf("sync Resolve() error = %v", err)
	}
	syncPC.ApplyToRequest(&syncReq)

	asyncReq := aiQueryRequest{
		Question:            "Ciro göster",
		ClarificationChoice: "ambiguity:0:1",
		DatasourceID:        "ds-1",
	}
	asyncPC := buildProcessContext(asyncReq)
	if err := asyncPC.Resolve(ctx, model, nil); err != nil {
		t.Fatalf("async Resolve() error = %v", err)
	}
	asyncPC.ApplyToRequest(&asyncReq)

	if syncPC.Question != asyncPC.Question {
		t.Errorf("Question mismatch: sync %q async %q", syncPC.Question, asyncPC.Question)
	}
	if syncPC.ClarificationResolved != asyncPC.ClarificationResolved {
		t.Errorf("ClarificationResolved mismatch: sync %v async %v", syncPC.ClarificationResolved, asyncPC.ClarificationResolved)
	}
	if syncReq.Question != asyncReq.Question {
		t.Errorf("applied Question mismatch: sync %q async %q", syncReq.Question, asyncReq.Question)
	}
	if syncReq.ClarificationChoice != asyncReq.ClarificationChoice {
		t.Errorf("applied ClarificationChoice mismatch: sync %q async %q", syncReq.ClarificationChoice, asyncReq.ClarificationChoice)
	}

	cfg := config.AmbiguityConfig{CheckEnabled: true}
	if syncPC.ShouldCheckAmbiguity(cfg) != asyncPC.ShouldCheckAmbiguity(cfg) {
		t.Error("ShouldCheckAmbiguity diverged between sync and async paths")
	}
	if syncPC.ShouldCheckAmbiguity(cfg) {
		t.Error("ShouldCheckAmbiguity = true after resolve, want false")
	}
}

func TestAmbiguityHardCapStopsAfterMaxRounds(t *testing.T) {
	cfg := config.AmbiguityConfig{CheckEnabled: true}

	for round := range maxClarificationRounds {
		pc := &ProcessContext{clarificationRound: round}
		if !pc.ShouldCheckAmbiguity(cfg) {
			t.Fatalf("round %d: ShouldCheckAmbiguity() = false, want true", round)
		}
		if pc.AmbiguityCapReached(cfg) {
			t.Fatalf("round %d: AmbiguityCapReached() = true, want false", round)
		}
	}

	capped := &ProcessContext{clarificationRound: maxClarificationRounds}
	if capped.ShouldCheckAmbiguity(cfg) {
		t.Error("at cap: ShouldCheckAmbiguity() = true, want false")
	}
	if !capped.AmbiguityCapReached(cfg) {
		t.Error("at cap: AmbiguityCapReached() = false, want true")
	}
}

func TestAttachAmbiguityClarificationRoundIncrementsWireRound(t *testing.T) {
	pc := &ProcessContext{clarificationRound: 1}
	resp := &ai.Response{
		Clarification: &ai.ClarificationResponse{
			NeedsClarification: true,
			Clarification: &ai.Clarification{
				Source: "ambiguity_analyzer",
			},
		},
	}

	attachAmbiguityClarificationRound(pc, resp)

	if got, want := resp.Clarification.ClarificationRound, 2; got != want {
		t.Errorf("ClarificationRound = %d, want %d", got, want)
	}
}

func TestAttachAmbiguityClarificationRoundSkipsRouterClarifications(t *testing.T) {
	pc := &ProcessContext{clarificationRound: 0}
	resp := &ai.Response{
		Clarification: &ai.ClarificationResponse{
			NeedsClarification: true,
			Clarification: &ai.Clarification{
				Source: "router",
			},
		},
	}

	attachAmbiguityClarificationRound(pc, resp)

	if resp.Clarification.ClarificationRound != 0 {
		t.Errorf("ClarificationRound = %d, want 0 for router clarifications", resp.Clarification.ClarificationRound)
	}
}
