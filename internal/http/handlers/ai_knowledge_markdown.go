package handlers

import (
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"
)

// knowledgeFolder* are the well-known knowledge-base folders whose published
// files extract into structured stores. Anything else (metrics/, ad-hoc
// folders, root files) stays markdown-only and reaches the AI through the
// agent knowledge tools.
const (
	knowledgeFolderInstructions = "instructions"
	knowledgeFolderGlossary     = "glossary"
	knowledgeFolderSQLPairs     = "sql-pairs"
	knowledgeFolderMetrics      = "metrics"
)

// parseKnowledgeMarkdown splits an .md document into its YAML frontmatter (as
// a generic map; nil when absent or invalid) and the markdown body.
func parseKnowledgeMarkdown(content string) (map[string]any, string) {
	trimmed := strings.TrimPrefix(content, "\ufeff")
	if !strings.HasPrefix(trimmed, "---\n") && trimmed != "---" && !strings.HasPrefix(trimmed, "---\r\n") {
		return nil, content
	}
	rest := trimmed[strings.Index(trimmed, "\n")+1:]
	endIdx := -1
	for _, marker := range []string{"\n---\n", "\n---\r\n"} {
		if i := strings.Index(rest, marker); i >= 0 {
			endIdx = i
			break
		}
	}
	var raw, body string
	switch {
	case endIdx >= 0:
		raw = rest[:endIdx]
		body = rest[endIdx+len("\n---\n"):]
	case strings.HasSuffix(strings.TrimRight(rest, "\n\r"), "\n---"),
		strings.TrimRight(rest, "\n\r") == "---":
		raw = strings.TrimSuffix(strings.TrimRight(rest, "\n\r"), "---")
	default:
		return nil, content
	}
	var fm map[string]any
	if err := yaml.Unmarshal([]byte(raw), &fm); err != nil || fm == nil {
		return nil, content
	}
	return fm, strings.TrimLeft(body, "\n\r")
}

var mdHeadingRe = regexp.MustCompile(`(?m)^#\s+(.+)$`)

// knowledgeTitle derives a display title: frontmatter title/term, else the
// first H1 heading, else the file name without extension.
func knowledgeTitle(fm map[string]any, body, path string) string {
	for _, key := range []string{"title", "term", "name"} {
		if v, ok := fm[key].(string); ok && strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	if m := mdHeadingRe.FindStringSubmatch(body); m != nil {
		return strings.TrimSpace(m[1])
	}
	base := path
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	return strings.TrimSuffix(base, ".md")
}

// fmString reads a string frontmatter field, empty when missing.
func fmString(fm map[string]any, key string) string {
	if v, ok := fm[key].(string); ok {
		return strings.TrimSpace(v)
	}
	return ""
}

// fmStrings reads a string-list frontmatter field ([a, b] or single string).
func fmStrings(fm map[string]any, key string) []string {
	switch v := fm[key].(type) {
	case string:
		if s := strings.TrimSpace(v); s != "" {
			return []string{s}
		}
	case []any:
		out := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && strings.TrimSpace(s) != "" {
				out = append(out, strings.TrimSpace(s))
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

var fencedSQLRe = regexp.MustCompile("(?s)```sql\\s*\\n(.*?)```")

// knowledgeSQLFromBody extracts the first fenced ```sql block from a
// sql-pairs body when the frontmatter doesn't carry the query explicitly.
func knowledgeSQLFromBody(body string) string {
	if m := fencedSQLRe.FindStringSubmatch(body); m != nil {
		return strings.TrimSpace(m[1])
	}
	return ""
}

var knowledgePathRe = regexp.MustCompile(`^[a-zA-Z0-9._/-]+\.md$`)

// validKnowledgePath accepts simple relative markdown paths (optionally one
// folder deep) and rejects traversal or absolute paths.
func validKnowledgePath(path string) bool {
	if !knowledgePathRe.MatchString(path) {
		return false
	}
	if strings.Contains(path, "..") || strings.HasPrefix(path, "/") || strings.Contains(path, "//") {
		return false
	}
	return strings.Count(path, "/") <= 1
}

var slugCleanupRe = regexp.MustCompile(`[^a-z0-9]+`)

// knowledgeSlug turns a free-form title into a file-name slug.
func knowledgeSlug(title string) string {
	lower := strings.ToLower(strings.TrimSpace(title))
	replacer := strings.NewReplacer("ç", "c", "ğ", "g", "ı", "i", "ö", "o", "ş", "s", "ü", "u")
	lower = replacer.Replace(lower)
	slug := strings.Trim(slugCleanupRe.ReplaceAllString(lower, "-"), "-")
	if slug == "" {
		return "note"
	}
	const maxSlugLen = 60
	if len(slug) > maxSlugLen {
		slug = strings.Trim(slug[:maxSlugLen], "-")
	}
	return slug
}
