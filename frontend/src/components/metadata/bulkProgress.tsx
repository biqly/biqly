/* eslint-disable react-refresh/only-export-components */

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
    <TagBadge tone={tone[status]} ariaLabel={t(STATUS_KEYS[status])}>
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
    <div className="mb-2 shrink-0">
      <div className="flex justify-between text-[0.8rem] text-foreground-muted mb-1 gap-2">
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
      <div className={`h-[6px] bg-card rounded-[4px] overflow-hidden border border-border`}>
        <div
          className="h-full transition-[width] duration-200 ease-in-out"
          style={{
            width: `${pct}%`,
            background: err > 0 ? 'linear-gradient(90deg, #4ade80, #f87171)' : '#4ade80',
          }}
        />
      </div>
      <div className="flex gap-3 mt-[0.3rem] text-[0.75rem] text-foreground-muted">
        <span className="text-emerald-400">{t('metadata.bulk_counts_ok', { ok })}</span>
        <span className="text-red-400">{t('metadata.bulk_counts_err', { err })}</span>
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
    return true
  })
  const preview = progress?.pending_preview?.length
    ? progress.pending_preview
    : pending.slice(0, 6).map((e) => `${e.schema}.${e.table}`)

  if (!preview.length && !progress?.current_schema) {
    return null
  }

  const current =
    progress?.current_schema && progress.current_table
      ? `${progress.current_schema}.${progress.current_table}`
      : entries.find((e) => e.status === 'running')
        ? `${entries.find((e) => e.status === 'running')!.schema}.${entries.find((e) => e.status === 'running')!.table}`
        : null

  const shown = preview.slice(0, 5)
  const more = preview.length > 5 ? preview.length - 5 : 0

  return (
    <div className="mb-3 shrink-0">
      <div className="text-[0.75rem] font-semibold text-foreground-muted mb-[0.35rem]">
        {t('metadata.bulk_queue_heading')}
      </div>
      {current && (
        <p className="m-[0_0_0.35rem] text-[0.8rem] text-foreground">
          {t('metadata.bulk_progress_processing', {
            done: completedSet.size,
            total: entries.length,
            current,
          })}
        </p>
      )}
      {shown.length > 0 && (
        <p className="m-0 text-[0.78rem] text-foreground-muted">
          {t('metadata.bulk_queue_next', { items: shown.join(', ') })}
          {more > 0 ? ` ${t('metadata.bulk_queue_more', { count: more })}` : ''}
        </p>
      )}
    </div>
  )
}
