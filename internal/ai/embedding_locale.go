package ai

import (
	"github.com/biqly/biqly/internal/i18n"
)

// embeddingLocalesWritten are persisted on each metadata embed refresh.
func embeddingLocalesWritten() []i18n.Locale {
	return append([]i18n.Locale(nil), i18n.SupportedLocales...)
}
