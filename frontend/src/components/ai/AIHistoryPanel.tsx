import { Fragment, useCallback, useEffect, useState } from 'react'

import { getAIHistoryDetail, listAIHistory } from '../../api/admin'
import { useQueryParam } from '../../hooks/useQueryParam'
import { useT } from '../../i18n'
import type { AIHistoryEntry } from '../../types/auth'
import { useAuth } from '../auth/AuthProvider'
import { ShareButton } from '../sharing/ShareButton'
import { LoadingOverlay } from '../ui/LoadingOverlay'
import { Pagination } from '../ui/Pagination'

function formatHistoryTokens(entry: AIHistoryEntry): string {
  const prompt = entry.prompt_tokens ?? 0
  const completion = entry.completion_tokens ?? 0
  const total = entry.token_count ?? 0
  if (prompt > 0 || completion > 0) {
    return `${prompt.toLocaleString()} + ${completion.toLocaleString()}`
  }
  if (total > 0) {
    return total.toLocaleString()
  }
  return '—'
}

export function AIHistoryPanel() {
  const t = useT()
  const { accessToken, roles } = useAuth()
  const [entries, setEntries] = useState<AIHistoryEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [showAll, setShowAll] = useState(false)
  const [historyIdParam, setHistoryIdParam] = useQueryParam('historyId')
  const expandedID = historyIdParam || null
  const [detail, setDetail] = useState<AIHistoryEntry | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)

  // Pagination
  const [currentPage, setCurrentPage] = useState(1)
  const pageSize = 10
  const [totalItems, setTotalItems] = useState(0)
  const totalPages = Math.ceil(totalItems / pageSize)
  const displayedEntries = entries

  const isAdmin = roles.some((r) => r === 'super_admin' || r === 'admin')

  const load = useCallback(async () => {
    if (!accessToken) {
      return
    }
    setLoading(true)
    try {
      const res = await listAIHistory(accessToken, {
        page: currentPage,
        pageSize,
        showAll: isAdmin ? showAll : undefined,
      })
      setEntries(res.entries || [])
      setTotalItems(res.total || 0)
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setLoading(false)
    }
  }, [accessToken, currentPage, showAll, isAdmin])

  useEffect(() => {
    void load()
  }, [load])

  useEffect(() => {
    setCurrentPage(1)
  }, [showAll])

  useEffect(() => {
    if (!expandedID || !accessToken) {
      setDetail(null)
      return
    }
    let cancelled = false
    setDetailLoading(true)
    getAIHistoryDetail(accessToken, expandedID)
      .then((d) => {
        if (!cancelled) {
          setDetail(d)
        }
      })
      .catch(() => {
        if (!cancelled) {
          setDetail(null)
        }
      })
      .finally(() => {
        if (!cancelled) {
          setDetailLoading(false)
        }
      })
    return () => {
      cancelled = true
    }
  }, [expandedID, accessToken])

  function toggleDetail(id: string) {
    if (expandedID === id) {
      setHistoryIdParam('')
    } else {
      setHistoryIdParam(id)
    }
  }

  function statusBadge(entry: AIHistoryEntry) {
    if (entry.needs_clarification) {
      return { label: t('admin.ai_history.status_clarification'), cls: 'clarification' }
    }
    if (entry.outcome_status === 'success') {
      return { label: t('admin.ai_history.status_success'), cls: 'success' }
    }
    return { label: t('admin.ai_history.status_error'), cls: 'error' }
  }

  if (!accessToken) {
    return null
  }

  return (
    <div className="ai-history">
      <div className="ai-history__header">
        <h2>{t('admin.ai_history.title')}</h2>
        {isAdmin && (
          <label className="ai-history__toggle">
            <input
              type="checkbox"
              checked={showAll}
              onChange={(e) => setShowAll(e.target.checked)}
            />
            <span>{t('admin.ai_history.show_all_users')}</span>
          </label>
        )}
      </div>

      {error && <div className="ai-history__error">{error}</div>}

      <div style={containerStyle}>
        <LoadingOverlay loading={loading}>
          <div
            style={{
              minHeight: entries.length === 0 && loading ? 120 : 'auto',
              display: 'flex',
              flexDirection: 'column',
            }}
          >
            {entries.length === 0 ? (
              <p
                className="ai-history__empty"
                style={{ margin: 0, padding: '48px 24px', textAlign: 'center' }}
              >
                {loading ? '' : t('admin.ai_history.empty')}
              </p>
            ) : (
              <>
                <div className="ai-history__table-wrap">
                  <table
                    className="ai-history__table"
                    style={{ borderCollapse: 'collapse', width: '100%' }}
                  >
                    <thead>
                      <tr style={theadRow}>
                        <th style={thStyle}>{t('admin.ai_history.question')}</th>
                        <th style={thStyle}>{t('admin.ai_history.status')}</th>
                        <th style={thStyle}>{t('admin.ai_history.confidence')}</th>
                        <th style={thStyle}>{t('admin.ai_history.model')}</th>
                        <th style={thStyle}>{t('admin.ai_history.latency')}</th>
                        <th style={thStyle}>{t('admin.ai_history.tokens')}</th>
                        <th style={thStyle}>{t('admin.ai_history.created_at')}</th>
                        <th style={thStyle}></th>
                      </tr>
                    </thead>
                    <tbody>
                      {displayedEntries.map((entry) => {
                        const badge = statusBadge(entry)
                        const isExpanded = expandedID === entry.id
                        return (
                          <Fragment key={entry.id}>
                            <tr
                              className={isExpanded ? 'ai-history__row--expanded' : ''}
                              style={trStyle}
                            >
                              <td className="ai-history__question" style={tdStyle}>
                                {entry.question || '—'}
                              </td>
                              <td style={tdStyle}>
                                <span
                                  className={`ai-history__status ai-history__status--${badge.cls}`}
                                >
                                  {badge.label}
                                </span>
                              </td>
                              <td style={tdStyle}>
                                {entry.confidence_score != null
                                  ? `${(entry.confidence_score * 100).toFixed(0)}%`
                                  : '—'}
                              </td>
                              <td
                                className="ai-history__mono"
                                style={{ ...tdStyle, fontFamily: 'var(--font-mono, monospace)' }}
                              >
                                {entry.model_used ?? '—'}
                              </td>
                              <td style={tdStyle}>
                                {entry.latency_ms != null ? `${entry.latency_ms}ms` : '—'}
                              </td>
                              <td style={tdStyle} title={t('admin.ai_history.tokens_breakdown')}>
                                {formatHistoryTokens(entry)}
                              </td>
                              <td style={tdStyle}>{new Date(entry.created_at).toLocaleString()}</td>
                              <td style={{ ...tdStyle, textAlign: 'right' }}>
                                <div className="ai-history__actions">
                                  <ShareButton resourceType="query" resourceID={entry.id} />
                                  <button
                                    onClick={() => toggleDetail(entry.id)}
                                    className="ai-history__detail-btn"
                                    aria-expanded={isExpanded}
                                    title={t('admin.ai_history.detail')}
                                  >
                                    {isExpanded ? '▲' : '▼'}
                                  </button>
                                </div>
                              </td>
                            </tr>
                            {isExpanded && (
                              <tr className="ai-history__detail-row">
                                <td colSpan={8}>
                                  {detailLoading ? (
                                    <div
                                      style={{
                                        position: 'relative',
                                        minHeight: 85,
                                        display: 'flex',
                                        alignItems: 'center',
                                        justifyContent: 'center',
                                      }}
                                    >
                                      <LoadingOverlay loading={true} />
                                    </div>
                                  ) : detail ? (
                                    <div className="ai-history__detail-content">
                                      {detail.prompt_context != null && (
                                        <div className="ai-history__detail-block">
                                          <h4>{t('admin.ai_history.prompt')}</h4>
                                          <pre>{formatDetail(detail.prompt_context)}</pre>
                                        </div>
                                      )}
                                      {detail.ai_response != null && (
                                        <div className="ai-history__detail-block">
                                          <h4>{t('admin.ai_history.generated_sql')}</h4>
                                          <pre>{formatDetail(detail.ai_response)}</pre>
                                        </div>
                                      )}
                                      {detail.logical_query != null && (
                                        <div className="ai-history__detail-block">
                                          <h4>{t('admin.ai_history.logical_query')}</h4>
                                          <pre>{formatDetail(detail.logical_query)}</pre>
                                        </div>
                                      )}
                                    </div>
                                  ) : (
                                    <p style={{ padding: 16 }}>—</p>
                                  )}
                                </td>
                              </tr>
                            )}
                          </Fragment>
                        )
                      })}
                    </tbody>
                  </table>
                </div>
                <Pagination
                  currentPage={currentPage}
                  totalPages={totalPages}
                  onPageChange={setCurrentPage}
                  totalItems={totalItems}
                  itemsPerPage={pageSize}
                />
              </>
            )}
          </div>
        </LoadingOverlay>
      </div>
    </div>
  )
}

function formatDetail(value: unknown): string {
  if (typeof value === 'string') {
    return value
  }
  return JSON.stringify(value, null, 2)
}

const containerStyle: React.CSSProperties = {
  background: 'var(--bg-card, #ffffff)',
  border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
  borderRadius: 8,
  overflow: 'hidden',
  boxShadow: 'var(--shadow-sm, 0 1px 3px rgba(0,0,0,0.05))',
}

const theadRow: React.CSSProperties = {
  background: 'var(--table-header-bg, #f9fafb)',
  borderBottom: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
  textAlign: 'left',
}

const thStyle: React.CSSProperties = {
  padding: '12px 16px',
  fontWeight: 600,
  color: 'var(--table-header-fg, #4b5563)',
}

const trStyle: React.CSSProperties = {
  borderBottom: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
}

const tdStyle: React.CSSProperties = {
  padding: '12px 16px',
  color: 'var(--text-primary, #f4f4f5)',
}

const textMuted: React.CSSProperties = {
  color: 'var(--text-secondary, #8a8a92)',
  fontSize: 14,
  padding: 16,
}

const errStyle: React.CSSProperties = {
  color: 'var(--error, crimson)',
  padding: 16,
  fontWeight: 600,
}
