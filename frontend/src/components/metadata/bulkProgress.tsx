export type BulkStatus = 'pending' | 'running' | 'ok' | 'error' | 'skipped'

export interface BulkEntry {
  schema: string
  table: string
  status: BulkStatus
  message?: string
}

const BULK_DISPLAY_ORDER: Record<BulkStatus, number> = {
  running: 0,
  pending: 1,
  error: 2,
  ok: 3,
  skipped: 4,
}

export function sortBulkEntriesForDisplay(entries: BulkEntry[]): BulkEntry[] {
  return [...entries]
    .map((entry, queueIndex) => ({ entry, queueIndex }))
    .sort((a, b) => {
      const da = BULK_DISPLAY_ORDER[a.entry.status]
      const db = BULK_DISPLAY_ORDER[b.entry.status]
      if (da !== db) return da - db
      return a.queueIndex - b.queueIndex
    })
    .map(({ entry }) => entry)
}

export function BulkStatusBadge({ status }: { status: BulkStatus }) {
  const map: Record<BulkStatus, { label: string; color: string }> = {
    pending: { label: 'bekliyor', color: 'var(--text-secondary)' },
    running: { label: 'çalışıyor', color: '#60a5fa' },
    ok: { label: 'tamam', color: '#4ade80' },
    error: { label: 'hata', color: '#f87171' },
    skipped: { label: 'atlandı', color: 'var(--text-secondary)' },
  }
  const s = map[status]
  return <span style={{ color: s.color, fontSize: '0.85rem', whiteSpace: 'nowrap' }}>{s.label}</span>
}

export function objectTypeLabel(tableType: string): string {
  const u = tableType.toUpperCase()
  if (u === 'VIEW') return 'Görünümler'
  if (u === 'BASE TABLE') return 'Temel tablolar'
  return tableType
}

export function BulkProgressHeader({
  entries,
  running,
  summary,
}: {
  entries: BulkEntry[]
  running: boolean
  summary: { ok: number; error: number; skipped: number } | null
}) {
  const total = entries.length
  const done = entries.filter((e) => e.status === 'ok' || e.status === 'error' || e.status === 'skipped').length
  const ok = entries.filter((e) => e.status === 'ok').length
  const err = entries.filter((e) => e.status === 'error').length
  const skipped = entries.filter((e) => e.status === 'skipped').length
  const current = entries.find((e) => e.status === 'running')
  const pct = total === 0 ? 0 : Math.round((done / total) * 100)

  return (
    <div style={{ marginBottom: '0.5rem', flexShrink: 0 }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.8rem', color: 'var(--text-secondary)', marginBottom: '0.25rem', gap: '0.5rem' }}>
        <span>
          {running
            ? <>İşleniyor {done} / {total} — şu an: <code>{current ? `${current.schema}.${current.table}` : '…'}</code></>
            : summary
              ? <>Tamamlandı — {summary.ok} başarılı, {summary.error} hata, {summary.skipped} atlandı</>
              : <>{done} / {total}</>}
        </span>
        <span>{pct}%</span>
      </div>
      <div style={{ height: '6px', background: 'var(--bg-card)', borderRadius: '4px', overflow: 'hidden', border: '1px solid var(--border)' }}>
        <div
          style={{
            width: `${pct}%`,
            height: '100%',
            background: err > 0 ? 'linear-gradient(90deg, #4ade80, #f87171)' : '#4ade80',
            transition: 'width 0.2s ease',
          }}
        />
      </div>
      <div style={{ display: 'flex', gap: '0.75rem', marginTop: '0.3rem', fontSize: '0.75rem', color: 'var(--text-secondary)' }}>
        <span style={{ color: '#4ade80' }}>OK {ok}</span>
        <span style={{ color: '#f87171' }}>ERR {err}</span>
        <span>SKIP {skipped}</span>
      </div>
    </div>
  )
}
