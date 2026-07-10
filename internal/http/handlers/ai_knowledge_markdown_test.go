package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseKnowledgeMarkdownWithFrontmatter(t *testing.T) {
	content := "---\ntype: glossary\nterm: money transfer\naliases: [\"transfers\", \"wire\"]\n---\n\n# money transfer\n\nBody text.\n"
	fm, body := parseKnowledgeMarkdown(content)
	require.NotNil(t, fm)
	assert.Equal(t, "glossary", fmString(fm, "type"))
	assert.Equal(t, "money transfer", fmString(fm, "term"))
	assert.Equal(t, []string{"transfers", "wire"}, fmStrings(fm, "aliases"))
	assert.Contains(t, body, "# money transfer")
	assert.NotContains(t, body, "term:")
}

func TestParseKnowledgeMarkdownWithoutFrontmatter(t *testing.T) {
	fm, body := parseKnowledgeMarkdown("# Just a doc\n\nHello.")
	assert.Nil(t, fm)
	assert.Equal(t, "# Just a doc\n\nHello.", body)
}

func TestParseKnowledgeMarkdownInvalidYAMLFallsBack(t *testing.T) {
	content := "---\n: : not yaml [\n---\n\nBody."
	fm, body := parseKnowledgeMarkdown(content)
	assert.Nil(t, fm)
	assert.Equal(t, content, body)
}

func TestKnowledgeTitlePrefersFrontmatterThenHeadingThenFilename(t *testing.T) {
	assert.Equal(t, "Fiscal year", knowledgeTitle(map[string]any{"title": "Fiscal year"}, "# Other", "instructions/fy.md"))
	assert.Equal(t, "Heading", knowledgeTitle(nil, "intro\n# Heading\ntext", "a/b.md"))
	assert.Equal(t, "fraud-rate", knowledgeTitle(nil, "no heading", "metrics/fraud-rate.md"))
}

func TestKnowledgeSQLFromBody(t *testing.T) {
	body := "Some text.\n\n```sql\nSELECT 1\nFROM t\n```\nMore."
	assert.Equal(t, "SELECT 1\nFROM t", knowledgeSQLFromBody(body))
	assert.Equal(t, "", knowledgeSQLFromBody("no sql here"))
}

func TestValidKnowledgePath(t *testing.T) {
	assert.True(t, validKnowledgePath("instructions/my-rule.md"))
	assert.True(t, validKnowledgePath("README.md"))
	assert.False(t, validKnowledgePath("a/b/c.md"), "max one folder deep")
	assert.False(t, validKnowledgePath("../etc/passwd.md"))
	assert.False(t, validKnowledgePath("/abs/path.md"))
	assert.False(t, validKnowledgePath("notes.txt"))
	assert.False(t, validKnowledgePath(""))
}

func TestKnowledgeSlug(t *testing.T) {
	assert.Equal(t, "ciro-hesabi-nasil-yapilir", knowledgeSlug("Ciro hesabı nasıl yapılır?"))
	assert.Equal(t, "money-transfer", knowledgeSlug("Money   Transfer"))
	assert.Equal(t, "note", knowledgeSlug("!!!"))
}

func TestStripMarkdownFence(t *testing.T) {
	wrapped := "```markdown\n---\ntitle: x\n---\n\n# X\n```"
	assert.Equal(t, "---\ntitle: x\n---\n\n# X", stripMarkdownFence(wrapped))
	plain := "---\ntitle: x\n---\n\n# X"
	assert.Equal(t, plain, stripMarkdownFence(plain))
}

func TestSuggestKnowledgePath(t *testing.T) {
	assert.Equal(t, "metrics/fraud-rate.md", suggestKnowledgePath("metrics", "Fraud Rate"))
	assert.Equal(t, "fraud-rate.md", suggestKnowledgePath("", "Fraud Rate"))
}
