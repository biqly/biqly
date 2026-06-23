import type { CatalogEntry } from '../../hooks/useSemanticCatalog'

export interface ActiveMention {
  /** Index in the value string where the triggering `@` sits. */
  at: number
  /** Text typed after the `@` (may be empty). */
  query: string
}

/**
 * Finds an active `@`-mention at the given cursor, or null when none is open.
 * The trigger must sit at the start of the input or follow whitespace /
 * punctuation so that email-style strings like "a@b" do not fire, and the text
 * between the `@` and the cursor must contain no whitespace.
 */
export function findActiveMention(value: string, cursor: number): ActiveMention | null {
  const before = value.slice(0, cursor)
  const atIdx = before.lastIndexOf('@')
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
