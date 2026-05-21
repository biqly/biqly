package ai

import (
	"testing"

	"github.com/biqly/biqly/internal/i18n"
)

func TestDetectQuestionLocale(t *testing.T) {
	tests := []struct {
		q    string
		want i18n.Locale
	}{
		{"dün kaç adet tweet atılmış?", i18n.LocaleTR},
		{"show total sales by customer", i18n.LocaleEN},
		{"", i18n.DefaultLocale},
		{"İstanbul sales", i18n.LocaleTR},
	}
	for _, tc := range tests {
		if got := DetectQuestionLocale(tc.q); got != tc.want {
			t.Errorf("DetectQuestionLocale(%q) = %q, want %q", tc.q, got, tc.want)
		}
	}
}
