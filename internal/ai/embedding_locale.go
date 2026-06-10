package ai

import (
	"github.com/biqly/biqly/internal/i18n"
)

// embeddingLocalesWritten are persisted on each metadata embed refresh.
// Uses the effective locale set so registry-added languages get embedding
// vectors without a release.
func embeddingLocalesWritten() []i18n.Locale {
	return i18n.ActiveLocales()
}
