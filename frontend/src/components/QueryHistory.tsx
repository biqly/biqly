import '../styles/ai-jobs.css'

import { Fragment, useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import { getAIHistoryDetail, listAIHistory } from '../api/admin'
import { useDatasources } from '../hooks/useDatasources'
import { useSemanticModels } from '../hooks/useSemanticModels'
import { useT } from '../i18n'
import type { AIHistoryEntry } from '../types/auth'
import { pickValidId } from '../utils/effectiveSelection'
import { useAuth } from './auth/AuthProvider'
import { EmptyState } from './ui/EmptyState'
import { LoadingOverlay } from './ui/LoadingOverlay'
import { Pagination } from './ui/Pagination'
import { Select } from './ui/Select'

export default function QueryHistory() {
  const t = useT()
  const navigate = useNavigate()
  const { accessToken } = useAuth()

  const [entries, setEntries] = useState<AIHistoryEntry[]>([])
  const [totalItems, setTotalItems] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const { datasources } = useDatasources()
  const { models } = useSemanticModels(null, { all: true })

  const [selectedDatasourceId, setSelectedDatasourceId] = useState('')
  const [selectedModelId, setSelectedModelId] = useState('')
  const [statusFilter, setStatusFilter] = useState('')
  const [searchInput, setSearchInput] = useState('')
  const [debouncedSearch, setDebouncedSearch] = useState('')

  const [expandedId, setExpandedId] = useState<string | null>(null)

  const pageSize = 10
  const totalPages = Math.max(1, Math.ceil(totalItems / pageSize))

  const filteredModels = useMemo(() => {
    if (!selectedDatasourceId) {
      return models
    }
    return models.filter((m) => m.datasource_id === selectedDatasourceId)
  }, [models, selectedDatasourceId])

  const effectiveModelId = useMemo(
    () => pickValidId(selectedModelId, filteredModels),
    [selectedModelId, filteredModels],
  )

  const filterKey = `${selectedDatasourceId}|${effectiveModelId}|${statusFilter}|${debouncedSearch}`
  const [pageState, setPageState] = useState({ key: filterKey, page: 1 })
  const currentPage = pageState.key === filterKey ? pageState.page : 1
  const setCurrentPage = useCallback(
    (page: number) => setPageState({ key: filterKey, page }),
    [filterKey],
  )

  const [detailCache, setDetailCache] = useState<Record<string, AIHistoryEntry | null>>({})
  const [inFlightDetailId, setInFlightDetailId] = useState<string | null>(null)

  useEffect(() => {
    const timer = window.setTimeout(() => setDebouncedSearch(searchInput), 300)
    return () => window.clearTimeout(timer)
  }, [searchInput])

  const loadHistory = useCallback(async () => {
    if (!accessToken) {
      return
    }
    setLoading(true)
    try {
      const res = await listAIHistory(accessToken, {
        page: currentPage,
        pageSize,
        datasourceId: selectedDatasourceId || undefined,
        modelId: effectiveModelId || undefined,
        status: statusFilter || undefined,
        search: debouncedSearch || undefined,
      })
      setEntries(res.entries)
      setTotalItems(res.total)
      setError(null)
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setLoading(false)
    }
  }, [
    accessToken,
    currentPage,
    pageSize,
    selectedDatasourceId,
    effectiveModelId,
    statusFilter,
    debouncedSearch,
  ])

  useEffect(() => {
    void loadHistory()
  }, [loadHistory])

  useEffect(() => {
    if (!expandedId || !accessToken || expandedId in detailCache) {
      return
    }
    let cancelled = false
    void Promise.resolve().then(() => {
      if (!cancelled) {
        setInFlightDetailId(expandedId)
      }
    })
    getAIHistoryDetail(accessToken, expandedId)
      .then((data) => {
        if (!cancelled) {
          setDetailCache((prev) => ({ ...prev, [expandedId]: data }))
        }
      })
      .catch(() => {
        if (!cancelled) {
          setDetailCache((prev) => ({ ...prev, [expandedId]: null }))
        }
      })
      .finally(() => {
        if (!cancelled) {
          setInFlightDetailId((prev) => (prev === expandedId ? null : prev))
        }
      })
    return () => {
      cancelled = true
    }
  }, [expandedId, accessToken, detailCache])

  const detail = expandedId ? (detailCache[expandedId] ?? null) : null
  const detailLoading =
    expandedId !== null && !(expandedId in detailCache) && inFlightDetailId === expandedId

  const datasourceMap = useMemo(() => {
    return new Map(datasources.map((d) => [d.id, d.name]))
  }, [datasources])

  const modelMap = useMemo(() => {
    return new Map(models.map((m) => [m.id, m.label ?? m.name]))
  }, [models])

  const toggleDetail = (id: string) => {
    setExpandedId((prev) => (prev === id ? null : id))
  }

  const handleRerun = (question: string) => {
    void navigate('/ai-query', { state: { question } })
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
    if (typeof value === 'string') {
      return value
    }
    return JSON.stringify(value, null, 2)
  }

  if (!accessToken) {
    return null
  }

  return (
    <div className="page-stack">
      <div className="card">
        <div className="card-intro">
          <h2>{t('query_history.title')}</h2>
          <p className="card-lead card-lead--single-line">{t('app.nav.query_history_desc')}</p>
        </div>

        <div
          className="form-row"
          style={{ gap: '1rem', flexWrap: 'wrap', alignItems: 'flex-end', marginBottom: '1.25rem' }}
        >
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
                ...filteredModels.map((m) => ({ value: m.id, label: m.label ?? m.name })),
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
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
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
          <div
            style={{
              minHeight: totalItems === 0 && loading ? 120 : 'auto',
              display: 'flex',
              flexDirection: 'column',
            }}
          >
            {totalItems === 0 && !loading ? (
              <EmptyState description={t('query_history.empty')} />
            ) : (
              <>
                <div className="ai-history__table-wrap">
                  <table
                    className="ai-history__table"
                    style={{ borderCollapse: 'collapse', width: '100%' }}
                  >
                    <thead>
                      <tr
                        style={{
                          background: 'var(--table-header-bg, #f9fafb)',
                          borderBottom: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
                          textAlign: 'left',
                        }}
                      >
                        <th
                          style={{
                            padding: '12px 16px',
                            fontWeight: 600,
                            color: 'var(--table-header-fg, #4b5563)',
                          }}
                        >
                          {t('query_history.col_question')}
                        </th>
                        <th
                          style={{
                            padding: '12px 16px',
                            fontWeight: 600,
                            color: 'var(--table-header-fg, #4b5563)',
                          }}
                        >
                          {t('query_history.col_status')}
                        </th>
                        <th
                          style={{
                            padding: '12px 16px',
                            fontWeight: 600,
                            color: 'var(--table-header-fg, #4b5563)',
                          }}
                        >
                          {t('query_history.col_confidence')}
                        </th>
                        <th
                          style={{
                            padding: '12px 16px',
                            fontWeight: 600,
                            color: 'var(--table-header-fg, #4b5563)',
                          }}
                        >
                          {t('query_history.col_model')}
                        </th>
                        <th
                          style={{
                            padding: '12px 16px',
                            fontWeight: 600,
                            color: 'var(--table-header-fg, #4b5563)',
                          }}
                        >
                          {t('query_history.col_latency')}
                        </th>
                        <th
                          style={{
                            padding: '12px 16px',
                            fontWeight: 600,
                            color: 'var(--table-header-fg, #4b5563)',
                          }}
                        >
                          {t('query_history.col_tokens')}
                        </th>
                        <th
                          style={{
                            padding: '12px 16px',
                            fontWeight: 600,
                            color: 'var(--table-header-fg, #4b5563)',
                          }}
                        >
                          {t('query_history.col_created_at')}
                        </th>
                        <th
                          style={{
                            padding: '12px 16px',
                            fontWeight: 600,
                            color: 'var(--table-header-fg, #4b5563)',
                          }}
                        >
                          {t('query_history.col_actions')}
                        </th>
                      </tr>
                    </thead>
                    <tbody>
                      {entries.map((entry) => {
                        const badge = getStatusBadge(entry)
                        const isExpanded = expandedId === entry.id
                        const datasourceName =
                          datasourceMap.get(entry.datasource_id) ?? entry.datasource_id
                        const modelName = entry.model_id
                          ? (modelMap.get(entry.model_id) ?? entry.model_id)
                          : '—'

                        return (
                          <Fragment key={entry.id}>
                            <tr
                              className={isExpanded ? 'ai-history__row--expanded' : ''}
                              style={{
                                borderBottom: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
                              }}
                            >
                              <td
                                className="ai-history__question"
                                style={{ padding: '12px 16px', color: 'var(--text-primary)' }}
                              >
                                <div style={{ fontWeight: '500' }}>{entry.question || '—'}</div>
                                <div
                                  style={{
                                    fontSize: '0.75rem',
                                    color: 'var(--text-secondary)',
                                    marginTop: '2px',
                                  }}
                                >
                                  {datasourceName}
                                </div>
                              </td>
                              <td style={{ padding: '12px 16px' }}>
                                <span
                                  className={`ai-history__status ai-history__status--${badge.cls}`}
                                >
                                  {badge.label}
                                </span>
                              </td>
                              <td style={{ padding: '12px 16px', color: 'var(--text-primary)' }}>
                                {entry.confidence_score != null
                                  ? `${(entry.confidence_score * 100).toFixed(0)}%`
                                  : '—'}
                              </td>
                              <td
                                className="ai-history__mono"
                                style={{
                                  padding: '12px 16px',
                                  fontFamily: 'var(--font-mono, monospace)',
                                  color: 'var(--text-primary)',
                                }}
                              >
                                {modelName}
                              </td>
                              <td style={{ padding: '12px 16px', color: 'var(--text-primary)' }}>
                                {entry.latency_ms != null ? `${entry.latency_ms}ms` : '—'}
                              </td>
                              <td style={{ padding: '12px 16px', color: 'var(--text-primary)' }}>
                                {entry.token_count ?? '—'}
                              </td>
                              <td
                                style={{
                                  padding: '12px 16px',
                                  color: 'var(--text-primary)',
                                  fontSize: '0.8rem',
                                }}
                              >
                                {new Date(entry.created_at).toLocaleString()}
                              </td>
                              <td style={{ padding: '12px 16px', textAlign: 'right' }}>
                                <div
                                  style={{
                                    display: 'flex',
                                    gap: '8px',
                                    justifyContent: 'flex-end',
                                    alignItems: 'center',
                                  }}
                                >
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
