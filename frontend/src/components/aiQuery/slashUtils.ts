import type { SavedQueryOption } from '../../api/aiSkills'

export interface ActiveSlash {
  /** Index in the value string where the triggering `/` sits. */
  at: number
  /** Text typed after the `/` (may be empty). */
  query: string
}

/**
 * Finds an active `/`-trigger at the given cursor, or null when none is open.
 * The trigger must sit at the start of the input or follow whitespace so that
 * mid-word slashes (URLs, dates like "2024/01", "and/or") do not fire, and the
 * text between the `/` and the cursor must contain no whitespace.
 */
export function findActiveSlash(value: string, cursor: number): ActiveSlash | null {
  const before = value.slice(0, cursor)
  const slashIdx = before.lastIndexOf('/')
  if (slashIdx === -1) {
    return null
  }
  if (slashIdx > 0) {
    const prev = value[slashIdx - 1] ?? ''
    if (!/\s/.test(prev)) {
      return null
    }
  }
  const query = before.slice(slashIdx + 1)
  if (/\s/.test(query)) {
    return null
  }
  return { at: slashIdx, query }
}

/** Ranks a saved query against a typed slash query. 0 means no match. */
export function scoreSavedQuery(option: SavedQueryOption, q: string): number {
  if (!q) {
    return 1
  }
  const ql = q.toLowerCase()
  const name = option.name.toLowerCase()
  const question = option.question.toLowerCase()
  if (name === ql) {
    return 100
  }
  if (name.startsWith(ql)) {
    return 80
  }
  if (name.includes(ql)) {
    return 50
  }
  if (question.includes(ql)) {
    return 30
  }
  return 0
}
