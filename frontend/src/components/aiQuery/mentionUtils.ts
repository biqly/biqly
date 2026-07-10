import type { CatalogEntry } from '../../hooks/useSemanticCatalog'

export interface ActiveMention {
  /** Index in the value string where the triggering `@` sits. */
  at: number
  /** Text typed after the `@` (may be empty). */
  query: string
}

/**
 * Finds an active `@` field or `#` business-term token at the given cursor.
 * The trigger must sit at the start of the input or follow whitespace /
 * punctuation so that email-style strings like "a@b" do not fire, and the text
 * between the `@` and the cursor must contain no whitespace.
 */
export function findActiveMention(value: string, cursor: number): ActiveMention | null {
  const before = value.slice(0, cursor)
  const atIdx = Math.max(before.lastIndexOf('@'), before.lastIndexOf('#'))
  if (atIdx === -1) {
    return null
  }
  if (atIdx > 0) {
    const prev = value[atIdx - 1] ?? ''
    if (!/\s|[()[\],;:.]/.test(prev)) {
      return null
    }
  }
  const query = before.slice(atIdx + 1)
  if (/\s/.test(query)) {
    return null
  }
  return { at: atIdx, query }
}

export interface MentionSegment {
  text: string
  /** Catalog name when this run is a recognized `@mention` token, else undefined. */
  name?: string
}

/**
 * Splits composer text into plain runs and recognized `@name` mention tokens so
 * the highlight overlay can color the tokens. A token is `@` followed by a run
 * of non-whitespace whose remainder exactly matches a known catalog name; an
 * unrecognized `@foo` stays plain text.
 */
export function splitMentionTokens(value: string, names: Set<string>): MentionSegment[] {
  const segments: MentionSegment[] = []
  const re = /[@#]\S+/g
  let last = 0
  let m: RegExpExecArray | null
  while ((m = re.exec(value)) !== null) {
    const tokenText = m[0]
    const name = tokenText.slice(1)
    if (!names.has(name)) {
      continue
    }
    if (m.index > last) {
      segments.push({ text: value.slice(last, m.index) })
    }
    segments.push({ text: tokenText, name })
    last = m.index + tokenText.length
  }
  if (last < value.length) {
    segments.push({ text: value.slice(last) })
  }
  return segments
}

/** Ranks a catalog entry against a typed mention query. 0 means no match. */
export function score(entry: CatalogEntry, q: string): number {
  if (!q) {
    return 1
  }
  const ql = q.toLowerCase()
  const name = entry.name.toLowerCase()
  const label = entry.label.toLowerCase()
  if (name === ql || label === ql) {
    return 100
  }
  if (name.startsWith(ql)) {
    return 80
  }
  if (label.startsWith(ql)) {
    return 70
  }
  if (name.includes(ql)) {
    return 50
  }
  if (label.includes(ql)) {
    return 40
  }
  return 0
}
