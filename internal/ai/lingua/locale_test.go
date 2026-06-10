package lingua

import (
	"context"
	"testing"

	"github.com/biqly/biqly/internal/i18n"
)

type stubLocaleProvider struct {
	locales []i18n.RuntimeLocale
}

func (s stubLocaleProvider) Locales(context.Context) ([]i18n.RuntimeLocale, error) {
	return s.locales, nil
}

func (stubLocaleProvider) Bundles(context.Context) (map[i18n.Locale]map[string]any, error) {
	return map[i18n.Locale]map[string]any{}, nil
}

// Question-language detection reads the dynamic locale registry: a new
// locale's letters/signals work without code changes (DİL-3 acceptance).
func TestDetectQuestionLocaleUsesRegistrySignals(t *testing.T) {
	i18n.SetRuntimeProvider(stubLocaleProvider{locales: []i18n.RuntimeLocale{{
		Profile: i18n.LocaleProfile{
			Locale:          i18n.Locale("de"),
			Label:           "Deutsch",
			ShortLabel:      "DE",
			QuestionLetters: "äöüß",
			QuestionSignals: []string{" wie viele ", " zeige ", " letzten "},
		},
		Enabled: true,
	}}})
	t.Cleanup(func() { i18n.SetRuntimeProvider(nil) })

	if got := DetectQuestionLocale("wie viele tweets im letzten monat?"); got != i18n.Locale("de") {
		t.Fatalf("DetectQuestionLocale(de question) = %q, want de", got)
	}
	if got := DetectQuestionLocale("dün kaç adet tweet atılmış?"); got != i18n.LocaleTR {
		t.Fatalf("embedded TR detection must keep working, got %q", got)
	}
}

func TestDetectQuestionLocale(t *testing.T) {
	tests := []struct {
		q    string
		want i18n.Locale
	}{
		{"dün kaç adet tweet atılmış?", i18n.LocaleTR},
		{"show total sales by customer", i18n.LocaleEN},
		{"", i18n.DefaultLocale},
		{"İstanbul sales", i18n.LocaleTR},
		{"silinen tweetler", i18n.LocaleTR},
	}
	for _, tc := range tests {
		if got := DetectQuestionLocale(tc.q); got != tc.want {
			t.Errorf("DetectQuestionLocale(%q) = %q, want %q", tc.q, got, tc.want)
		}
	}
}
