import type { TFunction, TranslationKey } from '../../i18n/locale'

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

export const STATUS_KEYS: Record<BulkStatus, TranslationKey> = {
  pending: 'metadata.bulk_status_pending',
  running: 'metadata.bulk_status_running',
  ok: 'metadata.bulk_status_ok',
  error: 'metadata.bulk_status_error',
  skipped: 'metadata.bulk_status_skipped',
}

export function objectTypeLabel(tableType: string, t: TFunction): string {
  const u = tableType.toUpperCase()
  if (u === 'VIEW') {
    return t('metadata.type_view')
  }
  if (u === 'BASE TABLE') {
    return t('metadata.type_base_table')
  }
  return tableType
}
