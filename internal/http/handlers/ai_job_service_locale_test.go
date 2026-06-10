package handlers

import (
	"context"
	"testing"

	"github.com/biqly/biqly/internal/ai"
	ambiguitypkg "github.com/biqly/biqly/internal/ai/ambiguity"
	"github.com/biqly/biqly/internal/i18n"
	"github.com/biqly/biqly/internal/metadata"
)

func TestConsumerContextForAIJobRestoresLocale(t *testing.T) {
	t.Parallel()

	job := &metadata.AIJob{Locale: "tr"}
	ctx := consumerContextForAIJob(context.Background(), job)
	if got := i18n.FromContext(ctx); got != i18n.LocaleTR {
		t.Fatalf("FromContext() = %q, want tr", got)
	}
}

func TestConsumerContextForAIJobClarificationUsesTRCatalog(t *testing.T) {
	t.Parallel()

	job := &metadata.AIJob{Locale: "tr"}
	ctx := consumerContextForAIJob(context.Background(), job)
	locale := i18n.FromContext(ctx)

	result := ambiguitypkg.Result{
		IsAmbiguous: true,
		Ambiguities: []ambiguitypkg.Item{
			{
				Term: "ay",
				Type: "semantic",
				Interpretations: []ambiguitypkg.Interpretation{
					{Label: "A"},
					{Label: "B"},
				},
			},
		},
	}
	clar := ai.ClarificationFromAmbiguityWithMaxOptions(locale, result, 8)
	if clar == nil {
		t.Fatal("expected clarification")
	}
	wantReason := i18n.T(i18n.LocaleTR, "clarification.ambiguity_reason")
	if clar.Reason != wantReason {
		t.Fatalf("Reason = %q, want %q", clar.Reason, wantReason)
	}
	wantQuestion := i18n.Tf(i18n.LocaleTR, "clarification.ambiguity_question_single", map[string]any{"Term": "ay"})
	if clar.Question != wantQuestion {
		t.Fatalf("Question = %q, want %q", clar.Question, wantQuestion)
	}
}
