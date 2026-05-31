package prompt

import (
	"embed"
	"fmt"

	"github.com/biqly/biqly/internal/i18n"
)

//go:embed prompts/*/*.tmpl
var promptFS embed.FS

// promptTemplateFromEmbed reads embedded defaults (seed + fallback).
func promptTemplateFromEmbed(loc i18n.Locale, name string) string {
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
