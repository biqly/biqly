package ai

import (
	"testing"

	"github.com/biqly/biqly/internal/i18n"
)

func TestPromptLocaleForQuestionFallsBackToUILocaleWhenQuestionHasNoSignals(t *testing.T) {
	got := PromptLocaleForQuestion("q", i18n.LocaleTR)
	if got != i18n.LocaleTR {
		t.Fatalf("PromptLocaleForQuestion() = %q, want %q", got, i18n.LocaleTR)
	}
}
