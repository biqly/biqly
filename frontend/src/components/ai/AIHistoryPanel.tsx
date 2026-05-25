import { Fragment, useEffect, useState, useCallback } from 'react'
import { listAIHistory, getAIHistoryDetail } from '../../api/admin'
import { useAuth } from '../auth/AuthProvider'
import { useT } from '../../i18n'
import type { AIHistoryEntry } from '../../types/auth'
import { ShareButton } from '../sharing/ShareButton'

export function AIHistoryPanel() {
  const t = useT()
  const { accessToken, roles } = useAuth()
  const [entries, setEntries] = useState<AIHistoryEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [showAll, setShowAll] = useState(false)
  const [limit, setLimit] = useState(50)
  const [expandedID, setExpandedID] = useState<string | null>(null)
  const [detail, setDetail] = useState<AIHistoryEntry | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)

  const isAdmin = roles.some((r) => r === 'super_admin' || r === 'admin')

  const load = useCallback(async () => {
    if (!accessToken) return
    setLoading(true)
    try {
      const rows = await listAIHistory(accessToken, { limit, showAll: isAdmin && showAll })
      setEntries(rows)
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setLoading(false)
    }
  }, [accessToken, limit, showAll, isAdmin])

  useEffect(() => {
    load()
  }, [load])

  async function toggleDetail(id: string) {
    if (expandedID === id) {
      setExpandedID(null)
      setDetail(null)
      return
    }
    setExpandedID(id)
    setDetailLoading(true)
    try {
      const d = await getAIHistoryDetail(accessToken!, id)
      setDetail(d)
    } catch {
      setDetail(null)
    } finally {
      setDetailLoading(false)
    }
  }

  function statusBadge(entry: AIHistoryEntry) {
    if (entry.needs_clarification) return { label: t('admin.ai_history.status_clarification'), cls: 'clarification' }
    if (entry.outcome_status === 'success') return { label: t('admin.ai_history.status_success'), cls: 'success' }
    return { label: t('admin.ai_history.status_error'), cls: 'error' }
  }

  if (!accessToken) return null

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
      {loading && <div className="ai-history__loading">{t('common.loading')}</div>}

      {!loading && entries.length === 0 && (
        <p className="ai-history__empty">{t('admin.ai_history.empty')}</p>
      )}

      {entries.length > 0 && (
        <div className="ai-history__table-wrap">
          <table className="ai-history__table">
            <thead>
              <tr>
                <th>{t('admin.ai_history.question')}</th>
                <th>{t('admin.ai_history.status')}</th>
                <th>{t('admin.ai_history.confidence')}</th>
                <th>{t('admin.ai_history.model')}</th>
                <th>{t('admin.ai_history.latency')}</th>
                <th>{t('admin.ai_history.tokens')}</th>
                <th>{t('admin.ai_history.created_at')}</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {entries.map((entry) => {
                const badge = statusBadge(entry)
                const isExpanded = expandedID === entry.id
                return (
                  <Fragment key={entry.id}>
                    <tr key={entry.id} className={isExpanded ? 'ai-history__row--expanded' : ''}>
                      <td className="ai-history__question">{entry.question || '—'}</td>
                      <td>
                        <span className={`ai-history__status ai-history__status--${badge.cls}`}>
                          {badge.label}
                        </span>
                      </td>
                      <td>{entry.confidence_score != null ? `${(entry.confidence_score * 100).toFixed(0)}%` : '—'}</td>
                      <td className="ai-history__mono">{entry.model_used || '—'}</td>
                      <td>{entry.latency_ms != null ? `${entry.latency_ms}ms` : '—'}</td>
                      <td>{entry.token_count ?? '—'}</td>
                      <td>{new Date(entry.created_at).toLocaleString()}</td>
                      <td>
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
                      <tr key={`${entry.id}-detail`} className="ai-history__detail-row">
                        <td colSpan={8}>
                          {detailLoading ? (
                            <div className="ai-history__loading">{t('common.loading')}</div>
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
                            <p>—</p>
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
      )}

      {entries.length >= limit && (
        <button
          onClick={() => setLimit((prev) => prev + 50)}
          className="ai-history__load-more"
        >
          {t('admin.ai_history.load_more')}
        </button>
      )}
    </div>
  )
}

function formatDetail(value: unknown): string {
  if (typeof value === 'string') return value
  return JSON.stringify(value, null, 2)
}
