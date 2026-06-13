import type { useT } from '../../i18n'
import { legacyButtonClass } from '../../lib/buttonClasses'
import { modalActionsClass } from '../../lib/modalClasses'
import {
  type BulkEntry,
  BulkProgressHeader,
  BulkQueuePreview,
  BulkStatusBadge,
} from './bulkProgress'

export function MetadataBulkDescribeProgress({
  t,
  bulkEntries,
  bulkEntriesDisplay,
  bulkRunning,
  bulkSummary,
  activeDescribeBatchJob,
  onClose,
  onCancelBulk,
}: {
  t: ReturnType<typeof useT>
  bulkEntries: BulkEntry[]
  bulkEntriesDisplay: BulkEntry[]
  bulkRunning: boolean
  bulkSummary: { ok: number; error: number; skipped: number } | null
  activeDescribeBatchJob: { progress_json?: unknown } | null | undefined
  onClose: () => void
  onCancelBulk: () => void
}) {
  return (
    <>
      <BulkProgressHeader entries={bulkEntries} running={bulkRunning} summary={bulkSummary} />
      {bulkRunning && (
        <BulkQueuePreview
          entries={bulkEntries}
          progress={activeDescribeBatchJob?.progress_json ?? null}
        />
      )}
      <div className={`flex-1 min-h-0 overflow-auto border border-border rounded-md`}>
        <table
          className="min-w-0 w-full table-fixed mt-0 text-[0.8125rem] border-collapse"
          style={{ margin: 0 }}
        >
          <thead>
            <tr>
              <th className="sticky top-0 z-[3] p-[0.5rem_0.45rem] bg-[var(--table-header-bg)] text-[var(--table-header-fg)] font-['Plus_Jakarta_Sans',sans-serif] text-[0.7rem] font-bold tracking-wider uppercase border-b border-[var(--border-strong)] shadow-[0_1px_0_var(--table-header-shadow-line)] w-[2.25rem] text-right text-foreground-faint [font-variant-numeric:tabular-nums]">
                {t('metadata.bulk_table_idx')}
              </th>
              <th className="sticky top-0 z-[3] p-[0.5rem_0.45rem] text-left align-top bg-[var(--table-header-bg)] text-[var(--table-header-fg)] font-['Plus_Jakarta_Sans',sans-serif] text-[0.7rem] font-bold tracking-wider uppercase border-b border-[var(--border-strong)] shadow-[0_1px_0_var(--table-header-shadow-line)]">
                {t('metadata.bulk_table_name')}
              </th>
              <th className="sticky top-0 z-[3] p-[0.5rem_0.45rem] text-left align-top bg-[var(--table-header-bg)] text-[var(--table-header-fg)] font-['Plus_Jakarta_Sans',sans-serif] text-[0.7rem] font-bold tracking-wider uppercase border-b border-[var(--border-strong)] shadow-[0_1px_0_var(--table-header-shadow-line)] w-[6.5rem]">
                {t('metadata.bulk_table_status')}
              </th>
              <th className="sticky top-0 z-[3] p-[0.5rem_0.45rem] text-left align-top bg-[var(--table-header-bg)] text-[var(--table-header-fg)] font-['Plus_Jakarta_Sans',sans-serif] text-[0.7rem] font-bold tracking-wider uppercase border-b border-[var(--border-strong)] shadow-[0_1px_0_var(--table-header-shadow-line)]">
                {t('metadata.bulk_table_detail')}
              </th>
            </tr>
          </thead>
          <tbody>
            {bulkEntriesDisplay.map((e, idx) => (
              <tr
                key={`${e.schema}.${e.table}`}
                className={`border-b border-border last:border-0 odd:bg-[var(--table-stripe-odd)] even:bg-[var(--table-stripe-even)]`}
              >
                <td className="p-[0.3rem_0.45rem] align-top text-right text-foreground-faint [font-variant-numeric:tabular-nums] w-[2.25rem]">
                  {idx + 1}
                </td>
                <td className="p-[0.3rem_0.45rem] align-top text-foreground-muted">
                  <code className="block whitespace-normal break-words [overflow-wrap:anywhere] text-[0.78rem] leading-[1.35]">
                    {e.schema}.{e.table}
                  </code>
                </td>
                <td className="p-[0.3rem_0.45rem] align-top w-[6.5rem]">
                  <BulkStatusBadge status={e.status} />
                </td>
                <td
                  className="p-[0.3rem_0.45rem] align-top text-foreground-muted"
                  style={{ color: 'var(--text-secondary)' }}
                >
                  <span
                    className="line-clamp-2 overflow-hidden break-words [overflow-wrap:anywhere] leading-[1.35] text-[0.78rem] max-w-full"
                    title={e.message}
                  >
                    {e.message ?? (e.status === 'pending' ? t('common.em_dash') : '')}
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <div className={modalActionsClass()}>
        {bulkRunning ? (
          <>
            <button
              type="button"
              className={legacyButtonClass('btn btn-ghost btn-sm')}
              onClick={onClose}
            >
              {t('metadata.bulk_run_background')}
            </button>
            <button
              type="button"
              className={legacyButtonClass('btn btn-ghost btn-sm')}
              onClick={onCancelBulk}
            >
              {t('metadata.bulk_stop_after')}
            </button>
          </>
        ) : (
          <button type="button" className={legacyButtonClass('btn btn-sm')} onClick={onClose}>
            {t('metadata.bulk_close_btn')}
          </button>
        )}
      </div>
    </>
  )
}
