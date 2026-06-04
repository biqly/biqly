import { useCallback, useEffect, useMemo, useState } from 'react'

import type { PIIColumn, PIIScanSummary } from '../../api/admin'
import { deleteColumnPII, listPIIColumns, scanPII, updateColumnPII } from '../../api/admin'
import { useDatasources } from '../../hooks/useDatasources'
import { useT } from '../../i18n'
import { useAuth } from '../auth/AuthProvider'
import { LoadingOverlay } from '../ui/LoadingOverlay'
import { Select } from '../ui/Select'
import { datasourceSelectOptions } from './adminSelectOptions'
import {
  normalizePIIMaskingStrategy,
  piiMaskingStrategyLabelKey,
  piiStrategyChanged,
  PII_MASKING_STRATEGIES,
  shouldShowPIIConfirmAction,
} from './piiDetectionPanelLogic'
import { ReadOnlyNote } from './ReadOnlyNote'

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
  const reviewer = user?.email || 'admin'

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
    if (datasources && datasources.length > 0 && !selectedDS) {
      const firstDS = datasources[0]
      if (firstDS) {
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
      setError(err instanceof Error ? err.message : String(err))
      setColumns([])
    } finally {
      setLoading(false)
    }
  }, [token, selectedDS])

  useEffect(() => {
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
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setScanning(false)
    }
  }

  const handleConfirm = async (col: PIIColumn) => {
    const piiType = pendingType[col.column_id] || col.pii_type
    const maskingStrategy = normalizePIIMaskingStrategy(
      pendingStrategy[col.column_id] || col.masking_strategy,
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
      setError(err instanceof Error ? err.message : String(err))
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
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  const dsOptions = useMemo(
    () => datasourceSelectOptions(datasources ?? [], loadingDS),
    [datasources, loadingDS],
  )

  const totalDetected = scanSummary
    ? Object.values(scanSummary.detected || {}).reduce((sum, n) => sum + n, 0)
    : 0

  return (
    <div className="pii-panel" style={containerStyle}>
      <h2 style={headerStyle}>{t('admin.pii.title')}</h2>
      <p style={descriptionStyle}>{t('admin.pii.description')}</p>

      {!canEdit && <ReadOnlyNote />}

      <div style={toolbarStyle}>
        <div style={labelStyle} className="admin-form-label">
          <span style={labelTextStyle}>Datasource</span>
          <Select
            value={selectedDS}
            options={dsOptions}
            onChange={setSelectedDS}
            disabled={loadingDS || dsOptions.every((o) => o.disabled)}
          />
        </div>
        <button
          onClick={handleRescan}
          disabled={!canEdit || !selectedDS || scanning}
          style={!canEdit || !selectedDS || scanning ? btnPrimaryDisabled : btnPrimary}
        >
          {scanning ? t('admin.pii.scanning') : t('admin.pii.rescan')}
        </button>
      </div>

      {error && (
        <div style={errStyle}>
          {t('common.error')}: {error}
        </div>
      )}
      {scanSummary && (
        <div style={successStyle}>
          {t('admin.pii.scan_summary', {
            scanned: String(scanSummary.scanned_columns),
            detected: String(totalDetected),
          })}
        </div>
      )}

      <LoadingOverlay loading={loading}>
        <div style={contentLayout}>
          {!selectedDS ? (
            <div style={emptyStyle}>{t('admin.pii.select_datasource')}</div>
          ) : columns.length === 0 && !loading ? (
            <div style={emptyStyle}>{t('admin.pii.empty')}</div>
          ) : (
            <table style={tableStyle}>
              <thead>
                <tr style={theadRow}>
                  <th style={thStyle}>{t('admin.pii.col_column')}</th>
                  <th style={thStyle}>{t('admin.pii.col_type')}</th>
                  <th style={thStyle}>{t('admin.pii.col_confidence')}</th>
                  <th style={thStyle}>{t('admin.pii.col_strategy')}</th>
                  <th style={thStyle}>{t('admin.pii.col_reviewed')}</th>
                  <th style={{ ...thStyle, textAlign: 'right' }}>{t('admin.pii.col_actions')}</th>
                </tr>
              </thead>
              <tbody>
                {columns.map((col) => {
                  const selectedStrategy = normalizePIIMaskingStrategy(
                    pendingStrategy[col.column_id] || col.masking_strategy,
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
                    <tr key={col.column_id} style={trRow}>
                      <td style={tdStyle}>
                        <code style={codeStyle}>
                          {col.schema}.{col.table}.{col.column}
                        </code>
                      </td>
                      <td style={tdStyle}>
                        {canEdit && !col.reviewed_by ? (
                          <select
                            value={pendingType[col.column_id] || col.pii_type}
                            onChange={(e) =>
                              setPendingType((prev) => ({
                                ...prev,
                                [col.column_id]: e.target.value,
                              }))
                            }
                            style={typeSelectStyle}
                            aria-label={t('admin.pii.col_type')}
                          >
                            {PII_TYPES.map((typ) => (
                              <option key={typ} value={typ}>
                                {typ}
                              </option>
                            ))}
                          </select>
                        ) : (
                          <span style={typeBadge}>{col.pii_type}</span>
                        )}
                      </td>
                      <td style={tdStyle}>
                        <ConfidenceBadge confidence={col.confidence} />
                      </td>
                      <td style={tdStyle}>
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
                              style={typeSelectStyle}
                              aria-label={t('admin.pii.col_strategy')}
                            >
                              {PII_MASKING_STRATEGIES.map((strategy) => (
                                <option key={strategy} value={strategy}>
                                  {t(piiMaskingStrategyLabelKey(strategy))}
                                </option>
                              ))}
                            </select>
                            {hasRawPIIAccess && (
                              <div style={strategyHintStyle}>
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
                      <td style={tdStyle}>
                        {col.reviewed_by ? (
                          <span style={reviewedBadge}>
                            {t('admin.pii.reviewed_by', { reviewer: col.reviewed_by })}
                          </span>
                        ) : (
                          <span style={unreviewedBadge}>{t('admin.pii.unreviewed')}</span>
                        )}
                      </td>
                      <td style={{ ...tdStyle, textAlign: 'right', whiteSpace: 'nowrap' }}>
                        {showConfirm && (
                          <button
                            onClick={() => handleConfirm(col)}
                            disabled={!canEdit}
                            style={canEdit ? btnSmall : btnSmallDisabled}
                          >
                            {t('admin.pii.confirm')}
                          </button>
                        )}
                        <button
                          onClick={() => handleDismiss(col)}
                          disabled={!canEdit}
                          style={canEdit ? btnSmallDanger : btnSmallDisabled}
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
    </div>
  )
}

function ConfidenceBadge({ confidence }: { confidence: number | null }) {
  if (confidence == null) {
    return <span>—</span>
  }
  const pct = Math.round(confidence * 100)
  const color = confidence > 0.8 ? '#10b981' : confidence >= 0.6 ? '#f59e0b' : '#ef4444'
  return (
    <span
      style={{
        padding: '2px 8px',
        borderRadius: 999,
        fontSize: 11,
        fontWeight: 600,
        color,
        background: `${color}1a`,
        border: `1px solid ${color}33`,
      }}
    >
      {pct}%
    </span>
  )
}

const containerStyle: React.CSSProperties = { display: 'flex', flexDirection: 'column', gap: 16 }

const headerStyle: React.CSSProperties = {
  margin: 0,
  fontSize: '20px',
  fontWeight: 600,
  color: 'var(--text-primary, #f4f4f5)',
}

const descriptionStyle: React.CSSProperties = {
  margin: 0,
  fontSize: 13,
  color: 'var(--text-secondary, #a1a1aa)',
}

const toolbarStyle: React.CSSProperties = {
  display: 'flex',
  alignItems: 'flex-end',
  gap: 16,
  background: 'var(--bg-card-raised, rgba(255, 255, 255, 0.04))',
  padding: 16,
  borderRadius: 8,
  border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
}

const labelStyle: React.CSSProperties = {
  display: 'flex',
  flexDirection: 'column',
  gap: 6,
  minWidth: 220,
}

const labelTextStyle: React.CSSProperties = {
  fontSize: '12px',
  color: 'var(--text-secondary, #a1a1aa)',
  fontWeight: 500,
  textTransform: 'uppercase',
  letterSpacing: '0.5px',
}

const contentLayout: React.CSSProperties = {
  background: 'var(--bg-card, #ffffff)',
  border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
  borderRadius: 8,
  overflow: 'auto',
}

const emptyStyle: React.CSSProperties = {
  padding: '60px 20px',
  textAlign: 'center',
  color: 'var(--text-secondary, #a1a1aa)',
  fontSize: 14,
}

const tableStyle: React.CSSProperties = {
  width: '100%',
  borderCollapse: 'collapse',
  fontSize: 14,
  textAlign: 'left',
}

const theadRow: React.CSSProperties = {
  background: 'var(--bg-card-raised, #f9fafb)',
  borderBottom: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
}

const thStyle: React.CSSProperties = {
  padding: '12px 16px',
  fontWeight: 600,
  color: 'var(--text-primary, #f4f4f5)',
  fontSize: 13,
}

const trRow: React.CSSProperties = {
  borderBottom: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
}

const tdStyle: React.CSSProperties = {
  padding: '12px 16px',
  color: 'var(--text-primary, #f4f4f5)',
}

const codeStyle: React.CSSProperties = {
  fontFamily: 'var(--font-mono, monospace)',
  fontSize: 12,
  color: 'var(--text-secondary, #a1a1aa)',
  background: 'var(--bg-card-raised, rgba(0, 0, 0, 0.2))',
  padding: '2px 6px',
  borderRadius: 4,
}

const typeBadge: React.CSSProperties = {
  padding: '2px 8px',
  background: 'rgba(99, 102, 241, 0.15)',
  color: 'var(--accent, #6366f1)',
  borderRadius: 4,
  fontSize: 11,
  fontWeight: 600,
}

const typeSelectStyle: React.CSSProperties = {
  padding: '4px 8px',
  borderRadius: 4,
  border: '1px solid var(--border, rgba(255, 255, 255, 0.12))',
  background: 'var(--bg-card, transparent)',
  color: 'var(--text-primary, #f4f4f5)',
  fontSize: 12,
}

const reviewedBadge: React.CSSProperties = {
  fontSize: 11,
  color: '#10b981',
  fontWeight: 500,
}

const unreviewedBadge: React.CSSProperties = {
  fontSize: 11,
  color: '#f59e0b',
  fontWeight: 500,
}

const strategyHintStyle: React.CSSProperties = {
  marginTop: 4,
  color: 'var(--text-secondary, #a1a1aa)',
  fontSize: 11,
  lineHeight: 1.35,
}

const btnPrimary: React.CSSProperties = {
  padding: '8px 16px',
  background: 'var(--accent, #6366f1)',
  color: 'white',
  border: 0,
  borderRadius: 6,
  cursor: 'pointer',
  fontSize: 13,
  fontWeight: 600,
}

const btnPrimaryDisabled: React.CSSProperties = {
  ...btnPrimary,
  background: 'var(--border, rgba(255, 255, 255, 0.06))',
  color: 'var(--text-secondary, #a1a1aa)',
  cursor: 'not-allowed',
  opacity: 0.5,
}

const btnSmall: React.CSSProperties = {
  padding: '4px 10px',
  marginLeft: 8,
  background: 'transparent',
  color: 'var(--accent, #6366f1)',
  border: '1px solid var(--accent, #6366f1)',
  borderRadius: 4,
  cursor: 'pointer',
  fontSize: 12,
  fontWeight: 600,
}

const btnSmallDanger: React.CSSProperties = {
  ...btnSmall,
  color: '#ef4444',
  border: '1px solid #ef4444',
}

const btnSmallDisabled: React.CSSProperties = {
  ...btnSmall,
  color: 'var(--text-secondary, #a1a1aa)',
  border: '1px solid var(--border, rgba(255,255,255,0.1))',
  cursor: 'not-allowed',
  opacity: 0.5,
}

const errStyle: React.CSSProperties = {
  color: 'var(--error, #ef4444)',
  padding: '10px 16px',
  background: 'rgba(239, 68, 68, 0.1)',
  borderRadius: 6,
  border: '1px solid rgba(239, 68, 68, 0.2)',
  fontSize: 13,
  fontWeight: 500,
}

const successStyle: React.CSSProperties = {
  color: 'var(--success, #10b981)',
  padding: '10px 16px',
  background: 'rgba(16, 185, 129, 0.1)',
  borderRadius: 6,
  border: '1px solid rgba(16, 185, 129, 0.2)',
  fontSize: 13,
  fontWeight: 500,
}
