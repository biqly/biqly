// Client-side pinned-conversation store. Pins are a per-browser preference
// (like the locale/theme toggles), not server state — so they need no schema
// change and never cross-contaminate between users on a shared machine beyond
// the existing local conversation cache.

const STORAGE_KEY = 'biqly.aiQuery.pinnedConversations'

export function loadPinnedIds(): Set<string> {
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    if (!raw) {
      return new Set()
    }
    const parsed: unknown = JSON.parse(raw)
    return Array.isArray(parsed)
      ? new Set(parsed.filter((v): v is string => typeof v === 'string'))
      : new Set()
  } catch {
    return new Set()
  }
}

export function savePinnedIds(ids: Set<string>): void {
  try {
    window.localStorage.setItem(STORAGE_KEY, JSON.stringify([...ids]))
  } catch {
    // Non-fatal: pins just won't persist across reloads.
  }
}

/** Returns a NEW set with `id` toggled — callers replace state immutably. */
export function togglePinnedId(ids: Set<string>, id: string): Set<string> {
  const next = new Set(ids)
  if (next.has(id)) {
    next.delete(id)
  } else {
    next.add(id)
  }
  return next
}
