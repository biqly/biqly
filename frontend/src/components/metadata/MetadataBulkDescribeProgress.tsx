import type { useT } from '../../i18n'
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
      <div className="bulk-describe-scroll">
        <table className="results-table results-table--dense" style={{ margin: 0 }}>
          <thead>
            <tr>
              <th className="bulk-col-idx">{t('metadata.bulk_table_idx')}</th>
              <th>{t('metadata.bulk_table_name')}</th>
              <th className="bulk-col-status">{t('metadata.bulk_table_status')}</th>
              <th>{t('metadata.bulk_table_detail')}</th>
            </tr>
          </thead>
          <tbody>
            {bulkEntriesDisplay.map((e, idx) => (
              <tr key={`${e.schema}.${e.table}`}>
                <td className="bulk-col-idx">{idx + 1}</td>
                <td className="bulk-col-name">
                  <code>
                    {e.schema}.{e.table}
                  </code>
                </td>
                <td className="bulk-col-status">
                  <BulkStatusBadge status={e.status} />
                </td>
                <td className="bulk-col-detail" style={{ color: 'var(--text-secondary)' }}>
                  <span className="bulk-col-detail-inner" title={e.message}>
                    {e.message ?? (e.status === 'pending' ? t('common.em_dash') : '')}
                  </span>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <div className="modal-actions">
        {bulkRunning ? (
          <>
            <button type="button" className="btn btn-ghost btn-sm" onClick={onClose}>
              {t('metadata.bulk_run_background')}
            </button>
            <button type="button" className="btn btn-ghost btn-sm" onClick={onCancelBulk}>
              {t('metadata.bulk_stop_after')}
            </button>
          </>
        ) : (
          <button type="button" className="btn btn-sm" onClick={onClose}>
            {t('metadata.bulk_close_btn')}
          </button>
        )}
      </div>
    </>
  )
}
