package ai

import (
	"strings"
	"unicode/utf8"

	"github.com/biqly/biqly/internal/i18n"
)

// DetectQuestionLocale picks a supported locale from the natural-language question so
// table routing uses the matching embedding vectors and localized metadata.
func DetectQuestionLocale(question string) i18n.Locale {
	if loc, ok := detectQuestionLocale(question); ok {
		return loc
	}
	return i18n.DefaultLocale
}

func detectQuestionLocale(question string) (i18n.Locale, bool) {
	q := strings.TrimSpace(question)
	if q == "" {
		return i18n.DefaultLocale, false
	}
	profiles := i18n.SupportedLocaleProfiles()
	scores := make(map[i18n.Locale]int, len(profiles))
	for _, profile := range i18n.SupportedLocaleProfiles() {
		if containsAnyRune(q, profile.QuestionLetters) {
			scores[profile.Locale] += 100
		}
	}
	lower := " " + strings.ToLower(q) + " "
	for _, profile := range profiles {
		for _, signal := range profile.QuestionSignals {
			if strings.Contains(lower, normalizeQuestionSignal(signal)) {
				scores[profile.Locale] += 10
			}
		}
	}
	bestLocale := i18n.DefaultLocale
	bestScore := 0
	for _, profile := range profiles {
		score := scores[profile.Locale]
		if score > bestScore {
			bestLocale = profile.Locale
			bestScore = score
		}
	}
	if bestScore == 0 {
		return i18n.DefaultLocale, false
	}
	return bestLocale, true
}

func containsAnyRune(s, chars string) bool {
	if chars == "" {
		return false
	}
	for _, r := range chars {
		if strings.ContainsRune(s, r) {
			return true
		}
	}
	return false
}

func normalizeQuestionSignal(signal string) string {
	signal = strings.ToLower(strings.TrimSpace(signal))
	if signal == "" {
		return ""
	}
	if utf8.RuneCountInString(signal) == 1 && strings.ContainsAny(signal, "?.!,;:") {
		return signal
	}
	if strings.HasSuffix(signal, "?") {
		return " " + strings.TrimSpace(signal)
	}
	return " " + signal + " "
}
