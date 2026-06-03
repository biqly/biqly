import type { TranslationKey } from '../../i18n'
import { useT } from '../../i18n'
import { TagBadge } from '../ui/TagBadge'

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
      if (da !== db) {
        return da - db
      }
      return a.queueIndex - b.queueIndex
    })
    .map(({ entry }) => entry)
}

const STATUS_KEYS: Record<BulkStatus, TranslationKey> = {
  pending: 'metadata.bulk_status_pending',
  running: 'metadata.bulk_status_running',
  ok: 'metadata.bulk_status_ok',
  error: 'metadata.bulk_status_error',
  skipped: 'metadata.bulk_status_skipped',
}

export function BulkStatusBadge({ status }: { status: BulkStatus }) {
  const t = useT()
  const tone: Record<BulkStatus, 'default' | 'success' | 'warning' | 'error'> = {
    pending: 'default',
    running: 'warning',
    ok: 'success',
    error: 'error',
    skipped: 'default',
  }
  return (
    <TagBadge tone={tone[status]} className="bulk-status-badge" ariaLabel={t(STATUS_KEYS[status])}>
      {t(STATUS_KEYS[status])}
    </TagBadge>
  )
}

type TFn = ReturnType<typeof useT>

export function objectTypeLabel(tableType: string, t: TFn): string {
  const u = tableType.toUpperCase()
  if (u === 'VIEW') {
    return t('metadata.type_view')
  }
  if (u === 'BASE TABLE') {
    return t('metadata.type_base_table')
  }
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
  const t = useT()
  const total = entries.length
  const done = entries.filter(
    (e) => e.status === 'ok' || e.status === 'error' || e.status === 'skipped',
  ).length
  const ok = entries.filter((e) => e.status === 'ok').length
  const err = entries.filter((e) => e.status === 'error').length
  const skipped = entries.filter((e) => e.status === 'skipped').length
  const current = entries.find((e) => e.status === 'running')
  const pct = total === 0 ? 0 : Math.round((done / total) * 100)

  const currentDisplay = current
    ? `${current.schema}.${current.table}`
    : t('metadata.bulk_progress_placeholder')

  return (
    <div style={{ marginBottom: '0.5rem', flexShrink: 0 }}>
      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          fontSize: '0.8rem',
          color: 'var(--text-secondary)',
          marginBottom: '0.25rem',
          gap: '0.5rem',
        }}
      >
        <span>
          {running ? (
            <>{t('metadata.bulk_progress_processing', { done, total, current: currentDisplay })}</>
          ) : summary ? (
            <>
              {t('metadata.bulk_progress_done', {
                ok: summary.ok,
                err: summary.error,
                skipped: summary.skipped,
              })}
            </>
          ) : (
            <>{t('metadata.bulk_progress_count', { done, total })}</>
          )}
        </span>
        <span>{pct}%</span>
      </div>
      <div
        style={{
          height: '6px',
          background: 'var(--bg-card)',
          borderRadius: '4px',
          overflow: 'hidden',
          border: '1px solid var(--border)',
        }}
      >
        <div
          style={{
            width: `${pct}%`,
            height: '100%',
            background: err > 0 ? 'linear-gradient(90deg, #4ade80, #f87171)' : '#4ade80',
            transition: 'width 0.2s ease',
          }}
        />
      </div>
      <div
        style={{
          display: 'flex',
          gap: '0.75rem',
          marginTop: '0.3rem',
          fontSize: '0.75rem',
          color: 'var(--text-secondary)',
        }}
      >
        <span style={{ color: '#4ade80' }}>{t('metadata.bulk_counts_ok', { ok })}</span>
        <span style={{ color: '#f87171' }}>{t('metadata.bulk_counts_err', { err })}</span>
        <span>{t('metadata.bulk_counts_skip', { skipped })}</span>
      </div>
    </div>
  )
}

export function BulkQueuePreview({
  entries,
  progress,
}: {
  entries: BulkEntry[]
  progress?: {
    pending_preview?: string[]
    completed?: string[]
    current_schema?: string
    current_table?: string
  } | null
}) {
  const t = useT()
  const completedSet = new Set(progress?.completed ?? [])
  const pending = entries.filter((e) => {
    const key = `${e.schema}.${e.table}`
    if (completedSet.has(key)) {
      return false
    }
    if (e.status === 'ok' || e.status === 'error' || e.status === 'skipped') {
      return false
    }
    return e.status === 'pending' || e.status === 'running'
  })
  const preview = progress?.pending_preview?.length
    ? progress.pending_preview
    : pending.slice(0, 6).map((e) => `${e.schema}.${e.table}`)

  if (!preview.length && !progress?.current_schema) {
    return null
  }

  const current =
    progress?.current_schema && progress?.current_table
      ? `${progress.current_schema}.${progress.current_table}`
      : entries.find((e) => e.status === 'running')
        ? `${entries.find((e) => e.status === 'running')!.schema}.${entries.find((e) => e.status === 'running')!.table}`
        : null

  const shown = preview.slice(0, 5)
  const more = preview.length > 5 ? preview.length - 5 : 0

  return (
    <div className="bulk-queue-preview" style={{ marginBottom: '0.75rem', flexShrink: 0 }}>
      <div
        style={{
          fontSize: '0.75rem',
          fontWeight: 600,
          color: 'var(--text-secondary)',
          marginBottom: '0.35rem',
        }}
      >
        {t('metadata.bulk_queue_heading')}
      </div>
      {current && (
        <p style={{ margin: '0 0 0.35rem', fontSize: '0.8rem', color: 'var(--text-primary)' }}>
          {t('metadata.bulk_progress_processing', {
            done: completedSet.size,
            total: entries.length,
            current,
          })}
        </p>
      )}
      {shown.length > 0 && (
        <p style={{ margin: 0, fontSize: '0.78rem', color: 'var(--text-secondary)' }}>
          {t('metadata.bulk_queue_next', { items: shown.join(', ') })}
          {more > 0 ? ` ${t('metadata.bulk_queue_more', { count: more })}` : ''}
        </p>
      )}
    </div>
  )
}
