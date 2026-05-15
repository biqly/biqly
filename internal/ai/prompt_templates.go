package ai

import (
	"embed"
	"fmt"

	"github.com/biqly/biqly/internal/i18n"
)

//go:embed prompts/en/*.tmpl prompts/tr/*.tmpl
var promptFS embed.FS

// promptTemplate returns the raw text for a named prompt section in the given
// locale. Falls back to English when the locale-specific file is missing — the
// LLM-facing rules are intentionally English-stable so model accuracy doesn't
// regress when the UI flips to a localized chrome.
func promptTemplate(loc i18n.Locale, name string) string {
	if loc == "" {
		loc = i18n.DefaultLocale
	}
	if data, err := promptFS.ReadFile(fmt.Sprintf("prompts/%s/%s.tmpl", loc, name)); err == nil {
		return string(data)
	}
	if loc != i18n.DefaultLocale {
		if data, err := promptFS.ReadFile(fmt.Sprintf("prompts/%s/%s.tmpl", i18n.DefaultLocale, name)); err == nil {
			return string(data)
		}
	}
	return ""
}
