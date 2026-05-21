package ai

import (
	"strings"

	"github.com/biqly/biqly/internal/i18n"
)

const embeddingLocaleSep = "@"

// EmbeddingModelForLocale tags stored vectors by language (e.g. embeddinggemma:300m@tr).
func EmbeddingModelForLocale(baseModel string, loc i18n.Locale) string {
	baseModel = strings.TrimSpace(baseModel)
	if baseModel == "" {
		return ""
	}
	if loc == "" || loc == i18n.DefaultLocale {
		return baseModel + embeddingLocaleSep + string(i18n.LocaleEN)
	}
	return baseModel + embeddingLocaleSep + string(loc)
}

// EmbeddingModelMatches returns true when a stored embedding_model row applies
// to the active question locale. Legacy rows without @locale count as English only.
func EmbeddingModelMatches(storedModel, baseModel string, loc i18n.Locale) bool {
	storedModel = strings.TrimSpace(storedModel)
	baseModel = strings.TrimSpace(baseModel)
	if storedModel == "" || baseModel == "" {
		return false
	}
	want := EmbeddingModelForLocale(baseModel, loc)
	if storedModel == want {
		return true
	}
	if loc == i18n.LocaleEN && storedModel == baseModel {
		return true
	}
	return false
}

// embeddingLocalesWritten are persisted on each metadata embed refresh.
func embeddingLocalesWritten() []i18n.Locale {
	return append([]i18n.Locale(nil), i18n.SupportedLocales...)
}
