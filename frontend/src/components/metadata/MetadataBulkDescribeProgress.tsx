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
      <div className={`border-border min-h-0 flex-1 overflow-auto rounded-md border`}>
        <table
          className="mt-0 w-full min-w-0 table-fixed border-collapse text-[0.8125rem]"
          style={{ margin: 0 }}
        >
          <thead>
            <tr>
              <th className="border-border-strong sticky top-0 z-3 w-9 border-b bg-(--table-header-bg) p-[0.5rem_0.45rem] text-right font-['Plus_Jakarta_Sans',sans-serif] text-[0.7rem] font-bold tracking-wider text-(--table-header-fg) uppercase [font-variant-numeric:tabular-nums] shadow-[0_1px_0_var(--table-header-shadow-line)]">
                {t('metadata.bulk_table_idx')}
              </th>
              <th className="border-border-strong sticky top-0 z-3 border-b bg-(--table-header-bg) p-[0.5rem_0.45rem] text-left align-top font-['Plus_Jakarta_Sans',sans-serif] text-[0.7rem] font-bold tracking-wider text-(--table-header-fg) uppercase shadow-[0_1px_0_var(--table-header-shadow-line)]">
                {t('metadata.bulk_table_name')}
              </th>
              <th className="border-border-strong sticky top-0 z-3 w-26 border-b bg-(--table-header-bg) p-[0.5rem_0.45rem] text-left align-top font-['Plus_Jakarta_Sans',sans-serif] text-[0.7rem] font-bold tracking-wider text-(--table-header-fg) uppercase shadow-[0_1px_0_var(--table-header-shadow-line)]">
                {t('metadata.bulk_table_status')}
              </th>
              <th className="border-border-strong sticky top-0 z-3 border-b bg-(--table-header-bg) p-[0.5rem_0.45rem] text-left align-top font-['Plus_Jakarta_Sans',sans-serif] text-[0.7rem] font-bold tracking-wider text-(--table-header-fg) uppercase shadow-[0_1px_0_var(--table-header-shadow-line)]">
                {t('metadata.bulk_table_detail')}
              </th>
            </tr>
          </thead>
          <tbody>
            {bulkEntriesDisplay.map((e, idx) => (
              <tr
                key={`${e.schema}.${e.table}`}
                className={`border-border border-b last:border-0 odd:bg-(--table-stripe-odd) even:bg-(--table-stripe-even)`}
              >
                <td className="text-foreground-faint w-9 p-[0.3rem_0.45rem] text-right align-top [font-variant-numeric:tabular-nums]">
                  {idx + 1}
                </td>
                <td className="text-foreground-muted p-[0.3rem_0.45rem] align-top">
                  <code className="block text-[0.78rem] leading-[1.35] wrap-anywhere whitespace-normal">
                    {e.schema}.{e.table}
                  </code>
                </td>
                <td className="w-26 p-[0.3rem_0.45rem] align-top">
                  <BulkStatusBadge status={e.status} />
                </td>
                <td
                  className="text-foreground-muted p-[0.3rem_0.45rem] align-top"
                  style={{ color: 'var(--text-secondary)' }}
                >
                  <span
                    className="line-clamp-2 max-w-full overflow-hidden text-[0.78rem] leading-[1.35] wrap-anywhere"
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
