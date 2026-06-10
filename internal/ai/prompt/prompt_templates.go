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
	if body := promptTemplateFromEmbedExact(loc, name); body != "" {
		return body
	}
	if loc != i18n.DefaultLocale {
		return promptTemplateFromEmbedExact(i18n.DefaultLocale, name)
	}
	return ""
}

// promptTemplateFromEmbedExact reads one embedded template without locale
// fallback, so callers can tell a locale-specific hit from an EN substitute.
func promptTemplateFromEmbedExact(loc i18n.Locale, name string) string {
	data, err := promptFS.ReadFile(fmt.Sprintf("prompts/%s/%s.tmpl", loc, name))
	if err != nil {
		return ""
	}
	return string(data)
}

// languageBridgeTemplates are the sections that carry user-facing wording and
// therefore get the bridge note when served from the EN fallback.
var languageBridgeTemplates = map[string]bool{
	"system_rules":  true,
	"clarification": true,
}

// languageBridgeNote returns the instruction appended to EN-fallback templates
// for a locale without its own template (ADR-0001 / DİL-4 bridge): the LLM
// keeps the EN instructions but writes user-facing text in the user's
// language. The permanent path is the admin authoring locale templates in the
// DB, which then take precedence and drop this note.
func languageBridgeNote(loc i18n.Locale, name string) string {
	if loc == "" || loc == i18n.DefaultLocale || !languageBridgeTemplates[name] {
		return ""
	}
	label := string(loc)
	if profile, ok := i18n.LocaleProfileFor(loc); ok && profile.Label != "" {
		label = profile.Label
	}
	return fmt.Sprintf("\n\n## User Language\nThe user's language is %s (%s). Write every user-facing text — clarification questions, explanations, warnings — in %s. Keep LogicalQuery JSON keys, catalog field names, and values exactly as listed in the catalog.", label, loc, label)
}
