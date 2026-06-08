import { unknownToDisplayString } from './formatters'

const UPDATE_CHECK_INTERVAL_MS = 60_000
const MIN_RELOAD_INTERVAL_MS = 15_000
const STORAGE_LAST_RELOAD = 'biqly:last_update_reload_ts'

export type UpdateTrigger = 'chunk-error' | 'new-version'

function now() {
  return Date.now()
}

function lastReloadTs(): number {
  try {
    const raw = window.sessionStorage.getItem(STORAGE_LAST_RELOAD)
    const n = raw ? Number(raw) : 0
    return Number.isFinite(n) ? n : 0
  } catch {
    return 0
  }
}

export function canReloadForUpdate(): boolean {
  const last = lastReloadTs()
  return last === 0 || now() - last > MIN_RELOAD_INTERVAL_MS
}

export function markReloadForUpdate() {
  try {
    window.sessionStorage.setItem(STORAGE_LAST_RELOAD, String(now()))
  } catch {
    /* ignore */
  }
}

function normalizeAssetPath(p: string): string {
  const q = p.indexOf('?')
  return q >= 0 ? p.slice(0, q) : p
}

export function currentIndexEntryScript(): string | null {
  const script = document.querySelector('script[type="module"][src]')
  const src = script?.getAttribute('src')
  return src ? normalizeAssetPath(src) : null
}

export async function fetchIndexEntryScript(): Promise<string | null> {
  const res = await fetch(`/?__biqly_update=${now()}`, {
    cache: 'no-store',
    credentials: 'same-origin',
    headers: { 'cache-control': 'no-cache' },
  })
  if (!res.ok) {
    return null
  }
  const html = await res.text()
  const match = /<script[^>]+type=["']module["'][^>]+src=["']([^"']+)["']/i.exec(html)
  if (!match?.[1]) {
    return null
  }
  return normalizeAssetPath(match[1])
}

export function startIndexPoll(onNewVersion: () => void): () => void {
  let cancelled = false
  let lastSeen = currentIndexEntryScript()

  async function tick() {
    if (cancelled) {
      return
    }
    try {
      const next = await fetchIndexEntryScript()
      if (next && lastSeen && next !== lastSeen) {
        onNewVersion()
        // avoid spamming; keep the old value until reload happens.
      } else if (next) {
        lastSeen = next
      }
    } catch {
      /* ignore */
    }
  }

  const id = window.setInterval(() => void tick(), UPDATE_CHECK_INTERVAL_MS)
  // Do a delayed first check so we don't compete with initial boot.
  const boot = window.setTimeout(() => void tick(), 10_000)

  return () => {
    cancelled = true
    window.clearInterval(id)
    window.clearTimeout(boot)
  }
}

function errorString(err: unknown): string {
  if (err instanceof Error) {
    return `${err.name}: ${err.message}`
  }
  return unknownToDisplayString(err)
}

export function isLikelyAssetLoadFailure(err: unknown): boolean {
  const msg = errorString(err).toLowerCase()
  return (
    msg.includes('loading chunk') ||
    msg.includes('chunkloaderror') ||
    msg.includes('failed to fetch dynamically imported module') ||
    msg.includes('importing a module script failed') ||
    (msg.includes('css') && msg.includes('failed')) ||
    msg.includes('net::err') ||
    msg.includes('unexpected token') // sometimes served HTML instead of JS after deploy
  )
}
