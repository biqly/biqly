import { Fragment, useEffect, useState, useMemo, useCallback } from 'react'
import { useNavigate } from 'react-router-dom'
import { listAIHistory, getAIHistoryDetail } from '../api/admin'
import { useAuth } from './auth/AuthProvider'
import { useT } from '../i18n'
import type { AIHistoryEntry } from '../types/auth'
import { Pagination } from './ui/Pagination'
import { LoadingOverlay } from './ui/LoadingOverlay'
import { useDatasources } from '../hooks/useDatasources'
import { useSemanticModels } from '../hooks/useSemanticModels'
import { Select } from './ui/Select'
import { EmptyState } from './ui/EmptyState'
import '../styles/ai-jobs.css'

export default function QueryHistory() {
  const t = useT()
  const navigate = useNavigate()
  const { accessToken } = useAuth()

  const [entries, setEntries] = useState<AIHistoryEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // Filters & Search
  const { datasources } = useDatasources()
  const { models } = useSemanticModels(null, { all: true })

  const [selectedDatasourceId, setSelectedDatasourceId] = useState('')
  const [selectedModelId, setSelectedModelId] = useState('')
  const [statusFilter, setStatusFilter] = useState('') // '', 'success', 'error', 'clarification'
  const [searchQuery, setSearchQuery] = useState('')

  // Detail panel expansion
  const [expandedId, setExpandedId] = useState<string | null>(null)
  const [detail, setDetail] = useState<AIHistoryEntry | null>(null)
  const [detailLoading, setDetailLoading] = useState(false)

  // Pagination
  const [currentPage, setCurrentPage] = useState(1)
  const pageSize = 10

  // Load all recent query history (up to 1000) for fast client-side filtering
  const loadHistory = useCallback(async () => {
    if (!accessToken) return
    setLoading(true)
    try {
      const res = await listAIHistory(accessToken, { page: 1, pageSize: 1000 })
      setEntries(res.entries || [])
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setLoading(false)
    }
  }, [accessToken])

  useEffect(() => {
    loadHistory()
  }, [loadHistory])

  // Reset selected model if it is not valid for the current datasource filter
  const filteredModels = useMemo(() => {
    if (!selectedDatasourceId) return models
    return models.filter((m) => m.datasource_id === selectedDatasourceId)
  }, [models, selectedDatasourceId])

  useEffect(() => {
    if (selectedModelId) {
      const exists = filteredModels.some((m) => m.id === selectedModelId)
      if (!exists) {
        setSelectedModelId('')
      }
    }
  }, [selectedDatasourceId, filteredModels, selectedModelId])

  // Reset current page when filters change
  useEffect(() => {
    setCurrentPage(1)
  }, [selectedDatasourceId, selectedModelId, statusFilter, searchQuery])

  // Load detail for expanded row
  useEffect(() => {
    if (!expandedId || !accessToken) {
      setDetail(null)
      return
    }
    let cancelled = false
    setDetailLoading(true)
    getAIHistoryDetail(accessToken, expandedId)
      .then((data) => {
        if (!cancelled) setDetail(data)
      })
      .catch(() => {
        if (!cancelled) setDetail(null)
      })
      .finally(() => {
        if (!cancelled) setDetailLoading(false)
      })
    return () => {
      cancelled = true
    }
  }, [expandedId, accessToken])

  // Filter entries client-side
  const filteredEntries = useMemo(() => {
    return entries.filter((entry) => {
      if (selectedDatasourceId && entry.datasource_id !== selectedDatasourceId) {
        return false
      }
      if (selectedModelId && entry.model_id !== selectedModelId) {
        return false
      }
      if (statusFilter) {
        if (statusFilter === 'clarification') {
          if (!entry.needs_clarification) return false
        } else if (statusFilter === 'success') {
          if (entry.needs_clarification || entry.outcome_status !== 'success') return false
        } else if (statusFilter === 'error') {
          if (entry.needs_clarification || entry.outcome_status === 'success') return false
        }
      }
      if (searchQuery) {
        const q = searchQuery.toLowerCase()
        const questionText = (entry.question || '').toLowerCase()
        if (!questionText.includes(q)) {
          return false
        }
      }
      return true
    })
  }, [entries, selectedDatasourceId, selectedModelId, statusFilter, searchQuery])

  // Paginate filtered entries
  const paginatedEntries = useMemo(() => {
    const start = (currentPage - 1) * pageSize
    const end = start + pageSize
    return filteredEntries.slice(start, end)
  }, [filteredEntries, currentPage, pageSize])

  const totalPages = Math.ceil(filteredEntries.length / pageSize)

  // Map datasource & model IDs to user-friendly labels
  const datasourceMap = useMemo(() => {
    return new Map(datasources.map((d) => [d.id, d.name]))
  }, [datasources])

  const modelMap = useMemo(() => {
    return new Map(models.map((m) => [m.id, m.label || m.name]))
  }, [models])

  const toggleDetail = (id: string) => {
    setExpandedId((prev) => (prev === id ? null : id))
  };

  const handleRerun = (question: string) => {
    navigate('/ai-query', { state: { question } })
  }

  const getStatusBadge = (entry: AIHistoryEntry) => {
    if (entry.needs_clarification) {
      return { label: t('query_history.status_clarification'), cls: 'clarification' }
    }
    if (entry.outcome_status === 'success') {
      return { label: t('query_history.status_success'), cls: 'success' }
    }
    return { label: t('query_history.status_error'), cls: 'error' }
  }

  function formatDetail(value: unknown): string {
    if (typeof value === 'string') return value
    return JSON.stringify(value, null, 2)
  }

  if (!accessToken) return null

  return (
    <div className="page-stack">
      <div className="card">
        <div className="card-intro">
          <h2>{t('query_history.title')}</h2>
          <p className="card-lead card-lead--single-line">
            {t('app.nav.query_history_desc')}
          </p>
        </div>

        {/* Filters Panel */}
        <div className="form-row" style={{ gap: '1rem', flexWrap: 'wrap', alignItems: 'flex-end', marginBottom: '1.25rem' }}>
          <label className="form-field" style={{ minWidth: '13rem' }}>
            <span className="form-label">{t('glossary.label_datasource')}</span>
            <Select
              value={selectedDatasourceId}
              options={[
                { value: '', label: t('glossary.option_all_datasources') },
                ...datasources.map((d) => ({ value: d.id, label: d.name })),
              ]}
              onChange={(v) => {
                setSelectedDatasourceId(v)
                setSelectedModelId('')
              }}
            />
          </label>
          <label className="form-field" style={{ minWidth: '13rem' }}>
            <span className="form-label">{t('glossary.label_model')}</span>
            <Select
              value={selectedModelId}
              options={[
                { value: '', label: t('glossary.option_all_models') },
                ...filteredModels.map((m) => ({ value: m.id, label: m.label || m.name })),
              ]}
              onChange={setSelectedModelId}
            />
          </label>
          <label className="form-field" style={{ minWidth: '10rem' }}>
            <span className="form-label">{t('query_history.label_status')}</span>
            <Select
              value={statusFilter}
              options={[
                { value: '', label: t('admin.filters.all') },
                { value: 'success', label: t('query_history.status_success') },
                { value: 'error', label: t('query_history.status_error') },
                { value: 'clarification', label: t('query_history.status_clarification') },
              ]}
              onChange={setStatusFilter}
            />
          </label>
          <div className="form-field" style={{ flex: 1, minWidth: '15rem' }}>
            <span className="form-label">{t('common.search')}</span>
            <input
              type="text"
              placeholder={t('query_history.search_placeholder')}
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="input"
            />
          </div>
        </div>

        {error && (
          <div style={{ color: 'var(--error, #ef4444)', marginBottom: '1rem', fontWeight: 600 }}>
            {t('common.error')}: {error}
          </div>
        )}

        <LoadingOverlay loading={loading}>
          <div style={{ minHeight: filteredEntries.length === 0 && loading ? 120 : 'auto', display: 'flex', flexDirection: 'column' }}>
            {filteredEntries.length === 0 ? (
              <EmptyState description={t('query_history.empty')} />
            ) : (
              <>
                <div className="ai-history__table-wrap">
                  <table className="ai-history__table" style={{ borderCollapse: 'collapse', width: '100%' }}>
                    <thead>
                      <tr style={{ background: 'var(--table-header-bg, #f9fafb)', borderBottom: '1px solid var(--border, rgba(255, 255, 255, 0.06))', textAlign: 'left' }}>
                        <th style={{ padding: '12px 16px', fontWeight: 600, color: 'var(--table-header-fg, #4b5563)' }}>{t('query_history.col_question')}</th>
                        <th style={{ padding: '12px 16px', fontWeight: 600, color: 'var(--table-header-fg, #4b5563)' }}>{t('query_history.col_status')}</th>
                        <th style={{ padding: '12px 16px', fontWeight: 600, color: 'var(--table-header-fg, #4b5563)' }}>{t('query_history.col_confidence')}</th>
                        <th style={{ padding: '12px 16px', fontWeight: 600, color: 'var(--table-header-fg, #4b5563)' }}>{t('query_history.col_model')}</th>
                        <th style={{ padding: '12px 16px', fontWeight: 600, color: 'var(--table-header-fg, #4b5563)' }}>{t('query_history.col_latency')}</th>
                        <th style={{ padding: '12px 16px', fontWeight: 600, color: 'var(--table-header-fg, #4b5563)' }}>{t('query_history.col_tokens')}</th>
                        <th style={{ padding: '12px 16px', fontWeight: 600, color: 'var(--table-header-fg, #4b5563)' }}>{t('query_history.col_created_at')}</th>
                        <th style={{ padding: '12px 16px', fontWeight: 600, color: 'var(--table-header-fg, #4b5563)' }}>{t('query_history.col_actions')}</th>
                      </tr>
                    </thead>
                    <tbody>
                      {paginatedEntries.map((entry) => {
                        const badge = getStatusBadge(entry)
                        const isExpanded = expandedId === entry.id
                        const datasourceName = datasourceMap.get(entry.datasource_id) || entry.datasource_id
                        const modelName = entry.model_id ? (modelMap.get(entry.model_id) || entry.model_id) : '—'

                        return (
                          <Fragment key={entry.id}>
                            <tr className={isExpanded ? 'ai-history__row--expanded' : ''} style={{ borderBottom: '1px solid var(--border, rgba(255, 255, 255, 0.06))' }}>
                              <td className="ai-history__question" style={{ padding: '12px 16px', color: 'var(--text-primary)' }}>
                                <div style={{ fontWeight: '500' }}>{entry.question || '—'}</div>
                                <div style={{ fontSize: '0.75rem', color: 'var(--text-secondary)', marginTop: '2px' }}>
                                  {datasourceName}
                                </div>
                              </td>
                              <td style={{ padding: '12px 16px' }}>
                                <span className={`ai-history__status ai-history__status--${badge.cls}`}>
                                  {badge.label}
                                </span>
                              </td>
                              <td style={{ padding: '12px 16px', color: 'var(--text-primary)' }}>
                                {entry.confidence_score != null ? `${(entry.confidence_score * 100).toFixed(0)}%` : '—'}
                              </td>
                              <td className="ai-history__mono" style={{ padding: '12px 16px', fontFamily: 'var(--font-mono, monospace)', color: 'var(--text-primary)' }}>
                                {modelName}
                              </td>
                              <td style={{ padding: '12px 16px', color: 'var(--text-primary)' }}>
                                {entry.latency_ms != null ? `${entry.latency_ms}ms` : '—'}
                              </td>
                              <td style={{ padding: '12px 16px', color: 'var(--text-primary)' }}>
                                {entry.token_count ?? '—'}
                              </td>
                              <td style={{ padding: '12px 16px', color: 'var(--text-primary)', fontSize: '0.8rem' }}>
                                {new Date(entry.created_at).toLocaleString()}
                              </td>
                              <td style={{ padding: '12px 16px', textAlign: 'right' }}>
                                <div style={{ display: 'flex', gap: '8px', justifyContent: 'flex-end', alignItems: 'center' }}>
                                  <button
                                    type="button"
                                    onClick={() => handleRerun(entry.question)}
                                    className="btn btn-sm btn-ghost"
                                    style={{ padding: '4px 8px', fontSize: '0.75rem' }}
                                  >
                                    {t('query_history.action_rerun')}
                                  </button>
                                  <button
                                    type="button"
                                    onClick={() => toggleDetail(entry.id)}
                                    className="ai-history__detail-btn"
                                    aria-expanded={isExpanded}
                                    title={t('query_history.action_preview')}
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
                                    <div style={{ position: 'relative', minHeight: 85, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                                      <LoadingOverlay loading={true} />
                                    </div>
                                  ) : detail ? (
                                    <div className="ai-history__detail-content">
                                      {detail.prompt_context != null && (
                                        <div className="ai-history__detail-block">
                                          <h4>{t('query_history.prompt')}</h4>
                                          <pre>{formatDetail(detail.prompt_context)}</pre>
                                        </div>
                                      )}
                                      {detail.ai_response != null && (
                                        <div className="ai-history__detail-block">
                                          <h4>{t('query_history.generated_sql')}</h4>
                                          <pre>{formatDetail(detail.ai_response)}</pre>
                                        </div>
                                      )}
                                      {detail.logical_query != null && (
                                        <div className="ai-history__detail-block">
                                          <h4>{t('query_history.logical_query')}</h4>
                                          <pre>{formatDetail(detail.logical_query)}</pre>
                                        </div>
                                      )}
                                    </div>
                                  ) : (
                                    <p style={{ padding: 16, color: 'var(--text-muted)' }}>—</p>
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
                  totalItems={filteredEntries.length}
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
