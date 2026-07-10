import { parse as parseYAML } from 'yaml'

export interface FrontmatterResult {
  /** Parsed YAML frontmatter; null when the document has none (or it is invalid). */
  frontmatter: Record<string, unknown> | null
  /** Raw frontmatter text between the --- fences (for the meta panel). */
  raw: string
  /** Markdown body without the frontmatter block. */
  body: string
}

/**
 * parseFrontmatter splits a markdown document into its YAML frontmatter and
 * body. Mirrors the backend's parseKnowledgeMarkdown semantics: the document
 * must START with a `---` line; an unparsable block is treated as no
 * frontmatter (the whole document stays the body).
 */
export function parseFrontmatter(content: string): FrontmatterResult {
  const none: FrontmatterResult = { frontmatter: null, raw: '', body: content }
  if (!content.startsWith('---\n') && !content.startsWith('---\r\n')) {
    return none
  }
  const rest = content.slice(content.indexOf('\n') + 1)
  const endMatch = /\n---(\r?\n|$)/.exec(rest)
  if (!endMatch) {
    return none
  }
  const raw = rest.slice(0, endMatch.index)
  const body = rest.slice(endMatch.index + endMatch[0].length).replace(/^[\r\n]+/, '')
  try {
    const parsed: unknown = parseYAML(raw)
    if (!parsed || typeof parsed !== 'object' || Array.isArray(parsed)) {
      return none
    }
    return { frontmatter: parsed as Record<string, unknown>, raw, body }
  } catch {
    return none
  }
}
