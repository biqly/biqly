import { useCallback, useEffect, useMemo, useState } from 'react'

import type { PIIColumn, PIIScanSummary } from '../../api/admin'
import { deleteColumnPII, listPIIColumns, scanPII, updateColumnPII } from '../../api/admin'
import { useDatasources } from '../../hooks/useDatasources'
import { useT } from '../../i18n'
import { cn } from '../../lib/cn'
import { errorMessage } from '../../utils/error'
import { useAuth } from '../auth/AuthProvider'
import { ErrorAlert } from '../ui/ErrorAlert'
import { LoadingOverlay } from '../ui/LoadingOverlay'
import { Select } from '../ui/Select'
import { adminBtnPrimaryClass, adminFormLabelClass } from './adminClasses'
import { AdminPanelShell } from './AdminPanelShell'
import { datasourceSelectOptions } from './adminSelectOptions'
import {
  normalizePIIMaskingStrategy,
  PII_MASKING_STRATEGIES,
  piiMaskingStrategyLabelKey,
  piiStrategyChanged,
  shouldShowPIIConfirmAction,
} from './piiDetectionPanelLogic'

const PII_TYPES = [
  'email',
  'phone',
  'iban',
  'tc_kimlik_no',
  'address',
  'ip_address',
  'credit_card_like',
] as const

export function PIIDetectionPanel({ token }: { token: string }) {
  const t = useT()
  const { hasPermission, isSuperAdmin, roles, user } = useAuth()
  const canEdit = hasPermission('admin:roles')
  const hasRawPIIAccess =
    isSuperAdmin || roles.some((role) => ['admin', 'super_admin'].includes(role.toLowerCase()))
  const reviewer = user?.email ?? 'admin'

  const { datasources, loading: loadingDS } = useDatasources()
  const [selectedDS, setSelectedDS] = useState<string>('')

  const [columns, setColumns] = useState<PIIColumn[]>([])
  const [loading, setLoading] = useState(false)
  const [scanning, setScanning] = useState(false)
  const [scanSummary, setScanSummary] = useState<PIIScanSummary | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [pendingType, setPendingType] = useState<Record<string, string>>({})
  const [pendingStrategy, setPendingStrategy] = useState<Record<string, string>>({})

  useEffect(() => {
    if (datasources.length > 0 && !selectedDS) {
      const firstDS = datasources[0]
      if (firstDS) {
        // eslint-disable-next-line react-hooks/set-state-in-effect
        setSelectedDS(firstDS.id)
      }
    }
  }, [datasources, selectedDS])

  const loadColumns = useCallback(async () => {
    if (!selectedDS) {
      setColumns([])
      return
    }
    setLoading(true)
    setError(null)
    try {
      const cols = await listPIIColumns(token, selectedDS)
      setColumns(cols)
    } catch (err) {
      setError(errorMessage(err))
      setColumns([])
    } finally {
      setLoading(false)
    }
  }, [token, selectedDS])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setScanSummary(null)
    setPendingType({})
    setPendingStrategy({})
    void loadColumns()
  }, [loadColumns])

  const handleRescan = async () => {
    if (!selectedDS || scanning) {
      return
    }
    setScanning(true)
    setError(null)
    try {
      const summary = await scanPII(token, selectedDS)
      setScanSummary(summary)
      await loadColumns()
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setScanning(false)
    }
  }

  const handleConfirm = async (col: PIIColumn) => {
    const piiType = pendingType[col.column_id] ?? col.pii_type
    const maskingStrategy = normalizePIIMaskingStrategy(
      pendingStrategy[col.column_id] ?? col.masking_strategy,
    )
    setError(null)
    try {
      await updateColumnPII(token, col.column_id, {
        pii_type: piiType,
        pii_masking_strategy: maskingStrategy,
        pii_reviewed_by: reviewer,
      })
      await loadColumns()
    } catch (err) {
      setError(errorMessage(err))
    }
  }

  const handleDismiss = async (col: PIIColumn) => {
    if (!window.confirm(t('admin.pii.dismiss_confirm'))) {
      return
    }
    setError(null)
    try {
      await deleteColumnPII(token, col.column_id, reviewer)
      await loadColumns()
    } catch (err) {
      setError(errorMessage(err))
    }
  }

  const dsOptions = useMemo(
    () => datasourceSelectOptions(datasources, loadingDS),
    [datasources, loadingDS],
  )

  const totalDetected = scanSummary
    ? Object.values(scanSummary.detected).reduce((sum, n) => sum + n, 0)
    : 0

  return (
    <AdminPanelShell
      title={t('admin.pii.title')}
      description={t('admin.pii.description')}
      readOnly={!canEdit}
      maxWidth="100%"
    >
      <div className="bg-card-raised border-border flex flex-wrap items-end gap-4 rounded-lg border p-4">
        <label className={cn(adminFormLabelClass, 'min-w-55')}>
          <span className="text-foreground-muted text-xs font-medium tracking-wider uppercase">
            {t('admin.pii.datasource')}
          </span>
          <Select
            value={selectedDS}
            options={dsOptions}
            onChange={setSelectedDS}
            disabled={loadingDS || dsOptions.every((o) => o.disabled)}
          />
        </label>
        <button
          onClick={() => {
            void handleRescan()
          }}
          disabled={!canEdit || !selectedDS || scanning}
          className={cn(
            adminBtnPrimaryClass,
            (!canEdit || !selectedDS || scanning) && 'cursor-not-allowed opacity-50',
          )}
        >
          {scanning ? t('admin.pii.scanning') : t('admin.pii.rescan')}
        </button>
      </div>

      {error && <ErrorAlert error={`${t('common.error')}: ${error}`} />}
      {scanSummary && (
        <div className="text-success rounded-lg border border-emerald-500/20 bg-emerald-500/12 p-[10px_16px] text-sm font-medium">
          {t('admin.pii.scan_summary', {
            scanned: String(scanSummary.scanned_columns),
            detected: String(totalDetected),
          })}
        </div>
      )}

      <LoadingOverlay loading={loading}>
        <div className="border-border bg-card overflow-x-auto rounded-lg border shadow-sm">
          {!selectedDS ? (
            <div className="text-foreground-muted p-[60px_20px] text-center text-sm">
              {t('admin.pii.select_datasource')}
            </div>
          ) : columns.length === 0 && !loading ? (
            <div className="text-foreground-muted p-[60px_20px] text-center text-sm">
              {t('admin.pii.empty')}
            </div>
          ) : (
            <table className="w-full border-collapse text-left text-sm">
              <thead>
                <tr className="border-border bg-card-raised border-b">
                  <th className="text-foreground p-[12px_16px] text-xs font-semibold tracking-wider uppercase">
                    {t('admin.pii.col_column')}
                  </th>
                  <th className="text-foreground p-[12px_16px] text-xs font-semibold tracking-wider uppercase">
                    {t('admin.pii.col_type')}
                  </th>
                  <th className="text-foreground p-[12px_16px] text-xs font-semibold tracking-wider uppercase">
                    {t('admin.pii.col_confidence')}
                  </th>
                  <th className="text-foreground p-[12px_16px] text-xs font-semibold tracking-wider uppercase">
                    {t('admin.pii.col_strategy')}
                  </th>
                  <th className="text-foreground p-[12px_16px] text-xs font-semibold tracking-wider uppercase">
                    {t('admin.pii.col_reviewed')}
                  </th>
                  <th className="text-foreground p-[12px_16px] text-right text-xs font-semibold tracking-wider uppercase">
                    {t('admin.pii.col_actions')}
                  </th>
                </tr>
              </thead>
              <tbody>
                {columns.map((col) => {
                  const selectedStrategy = normalizePIIMaskingStrategy(
                    pendingStrategy[col.column_id] ?? col.masking_strategy,
                  )
                  const pendingPIIType = pendingType[col.column_id]
                  const typeChanged = pendingPIIType != null && pendingPIIType !== col.pii_type
                  const strategyChanged = piiStrategyChanged(
                    col.masking_strategy,
                    pendingStrategy[col.column_id],
                  )
                  const showConfirm = shouldShowPIIConfirmAction({
                    canEdit,
                    reviewedBy: col.reviewed_by,
                    typeChanged,
                    strategyChanged,
                  })

                  return (
                    <tr key={col.column_id} className="border-border border-b">
                      <td className="text-foreground p-[12px_16px]">
                        <code className="text-foreground-muted bg-card-raised rounded px-1.5 py-0.5 font-mono text-xs">
                          {col.schema}.{col.table}.{col.column}
                        </code>
                      </td>
                      <td className="text-foreground p-[12px_16px]">
                        {canEdit && !col.reviewed_by ? (
                          <select
                            value={pendingType[col.column_id] ?? col.pii_type}
                            onChange={(e) =>
                              setPendingType((prev) => ({
                                ...prev,
                                [col.column_id]: e.target.value,
                              }))
                            }
                            className="border-border bg-card text-foreground rounded border p-1 px-2 text-xs focus-visible:outline-none"
                            aria-label={t('admin.pii.col_type')}
                          >
                            {PII_TYPES.map((typ) => (
                              <option key={typ} value={typ}>
                                {typ}
                              </option>
                            ))}
                          </select>
                        ) : (
                          <span className="bg-accent/15 text-accent inline-block rounded px-2 py-0.5 text-xs font-semibold">
                            {col.pii_type}
                          </span>
                        )}
                      </td>
                      <td className="text-foreground p-[12px_16px]">
                        <ConfidenceBadge confidence={col.confidence} />
                      </td>
                      <td className="text-foreground p-[12px_16px]">
                        {canEdit ? (
                          <>
                            <select
                              value={selectedStrategy}
                              onChange={(e) =>
                                setPendingStrategy((prev) => ({
                                  ...prev,
                                  [col.column_id]: e.target.value,
                                }))
                              }
                              className="border-border bg-card text-foreground rounded border p-1 px-2 text-xs focus-visible:outline-none"
                              aria-label={t('admin.pii.col_strategy')}
                            >
                              {PII_MASKING_STRATEGIES.map((strategy) => (
                                <option key={strategy} value={strategy}>
                                  {t(piiMaskingStrategyLabelKey(strategy))}
                                </option>
                              ))}
                            </select>
                            {hasRawPIIAccess && (
                              <div className="text-foreground-muted text-2xs mt-1 leading-normal">
                                {t('admin.pii.strategy_raw_access_note')}
                              </div>
                            )}
                          </>
                        ) : (
                          <span>
                            {col.masking_strategy
                              ? t(piiMaskingStrategyLabelKey(col.masking_strategy))
                              : '—'}
                          </span>
                        )}
                      </td>
                      <td className="text-foreground p-[12px_16px]">
                        {col.reviewed_by ? (
                          <span className="text-success text-xs font-medium">
                            {t('admin.pii.reviewed_by', { reviewer: col.reviewed_by })}
                          </span>
                        ) : (
                          <span className="text-warning text-xs font-medium">
                            {t('admin.pii.unreviewed')}
                          </span>
                        )}
                      </td>
                      <td className="text-foreground p-[12px_16px] text-right whitespace-nowrap">
                        {showConfirm && (
                          <button
                            onClick={() => {
                              void handleConfirm(col)
                            }}
                            disabled={!canEdit}
                            className="border-accent bg-accent/10 text-accent hover:bg-accent/20 mr-2 cursor-pointer rounded border px-2.5 py-1 text-xs font-semibold transition-all duration-200 disabled:cursor-not-allowed disabled:opacity-50"
                          >
                            {t('admin.pii.confirm')}
                          </button>
                        )}
                        <button
                          onClick={() => {
                            void handleDismiss(col)
                          }}
                          disabled={!canEdit}
                          className="border-error bg-error/10 text-error hover:bg-error/20 cursor-pointer rounded border px-2.5 py-1 text-xs font-semibold transition-all duration-200 disabled:cursor-not-allowed disabled:opacity-50"
                        >
                          {t('admin.pii.dismiss')}
                        </button>
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          )}
        </div>
      </LoadingOverlay>
    </AdminPanelShell>
  )
}

function ConfidenceBadge({ confidence }: { confidence: number | null }) {
  if (confidence == null) {
    return <span>—</span>
  }
  const pct = Math.round(confidence * 100)
  const colorClass =
    confidence > 0.8
      ? 'text-success bg-emerald-500/10 border-emerald-500/20'
      : confidence >= 0.6
        ? 'text-warning bg-amber-500/10 border-amber-500/20'
        : 'text-error bg-red-500/10 border-red-500/20'
  return (
    <span
      className={cn(
        'inline-block rounded-full border px-2 py-0.5 text-xs font-semibold',
        colorClass,
      )}
    >
      {pct}%
    </span>
  )
}
