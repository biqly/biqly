/**
 * Module-level registry that dedupes the fixed bottom-right loading pill.
 * Route-level Suspense fallbacks and screen-level LoadingScreens can be
 * mounted at the same time; without this every instance rendered its own
 * fixed pill at the same corner (stacked rings/borders) and announced a
 * duplicate role="status" to screen readers. Only the first-mounted active
 * instance ("owner") renders the pill; ownership hands over seamlessly when
 * it unmounts.
 */

type Listener = () => void

/** Pills mounted within this window of the previous one skip the entry delay. */
export const HANDOFF_WINDOW_MS = 400

let nextId = 1
const active: number[] = []
const listeners = new Set<Listener>()
let lastHiddenAt = 0

function emit(): void {
  for (const listener of listeners) {
    listener()
  }
}

export function acquireCornerId(): number {
  return nextId++
}

export function registerCorner(id: number): void {
  active.push(id)
  emit()
}

export function unregisterCorner(id: number): void {
  const idx = active.indexOf(id)
  if (idx !== -1) {
    active.splice(idx, 1)
  }
  if (active.length === 0) {
    lastHiddenAt = Date.now()
  }
  emit()
}

export function subscribeCorner(listener: Listener): () => void {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

/** useSyncExternalStore snapshot — primitive so Object.is comparison works. */
export function cornerOwnerId(): number {
  return active[0] ?? 0
}

/**
 * True when another pill is (or was just) visible, so the incoming pill
 * should appear without the entry delay — consecutive route-load → data-load
 * phases read as one continuous indicator instead of blinking.
 */
export function shouldShowImmediately(now: number = Date.now()): boolean {
  return active.length > 0 || now - lastHiddenAt < HANDOFF_WINDOW_MS
}
