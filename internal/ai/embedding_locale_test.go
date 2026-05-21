package ai

import (
	"testing"

	"github.com/biqly/biqly/internal/i18n"
)

func TestEmbeddingModelForLocale(t *testing.T) {
	base := "embeddinggemma:300m"
	if got := EmbeddingModelForLocale(base, i18n.LocaleTR); got != base+"@tr" {
		t.Fatalf("got %q", got)
	}
	if got := EmbeddingModelForLocale(base, i18n.LocaleEN); got != base+"@en" {
		t.Fatalf("got %q", got)
	}
}

func TestEmbeddingModelMatches(t *testing.T) {
	base := "embeddinggemma:300m"
	if !EmbeddingModelMatches(base+"@tr", base, i18n.LocaleTR) {
		t.Fatal("expected tr tag match")
	}
	if EmbeddingModelMatches(base, base, i18n.LocaleTR) {
		t.Fatal("legacy bare model must not match Turkish questions")
	}
	if !EmbeddingModelMatches(base, base, i18n.LocaleEN) {
		t.Fatal("legacy bare model should match English")
	}
}
