import { Fragment, useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import { getAIHistoryDetail, listAIHistory } from '../api/admin'
import { useDatasources } from '../hooks/useDatasources'
import { useDebouncedValue } from '../hooks/useDebouncedValue'
import { usePaginatedList } from '../hooks/usePaginatedList'
import { useSemanticModels } from '../hooks/useSemanticModels'
import { useT } from '../i18n'
import { legacyButtonClass } from '../lib/buttonClasses'
import { legacyCardClass } from '../lib/cardClasses'
import { cn } from '../lib/cn'
import { formRowClass, legacyFormClass } from '../lib/formClasses'
import { legacyLayoutClass } from '../lib/layoutClasses'
import type { AIHistoryEntry } from '../types/auth'
import type { PageQuery } from '../types/pagination'
import { pickValidId } from '../utils/effectiveSelection'
import {
  aiHistoryDetailBlockClass,
  aiHistoryDetailBtnClass,
  aiHistoryDetailContentClass,
  aiHistoryDetailRowClass,
  aiHistoryMonoClass,
  aiHistoryQuestionClass,
  aiHistoryRowExpandedClass,
  aiHistoryStatusClass,
  type AiHistoryStatusVariant,
  aiHistoryTableClass,
  aiHistoryTableWrapClass,
} from './ai/aiJobsClasses'
import { useAuth } from './auth/AuthProvider'
import { DataState } from './ui/DataState'
import { EmptyState } from './ui/EmptyState'
import { LoadingOverlay } from './ui/LoadingOverlay'
import { Pagination } from './ui/Pagination'
import { Select } from './ui/Select'

export default function QueryHistory() {
  const t = useT()
  const navigate = useNavigate()
  const { accessToken } = useAuth()

  const { datasources } = useDatasources()
  const { models } = useSemanticModels(null, { all: true })

  const [selectedDatasourceId, setSelectedDatasourceId] = useState('')
  const [selectedModelId, setSelectedModelId] = useState('')
  const [statusFilter, setStatusFilter] = useState('')
  const [searchInput, setSearchInput] = useState('')
  const debouncedSearch = useDebouncedValue(searchInput, 300)

  const [expandedId, setExpandedId] = useState<string | null>(null)

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

  const fetcher = useCallback(
    async (q: PageQuery) => {
      const res = await listAIHistory(accessToken ?? '', {
        page: q.page,
        pageSize: q.pageSize,
        datasourceId: selectedDatasourceId || undefined,
        modelId: effectiveModelId || undefined,
        status: statusFilter || undefined,
        search: debouncedSearch || undefined,
      })
      return { items: res.entries, total: res.total }
    },
    [accessToken, selectedDatasourceId, effectiveModelId, statusFilter, debouncedSearch],
  )
  const {
    items: entries,
    loading,
    error,
    page: currentPage,
    setPage: setCurrentPage,
    pageSize,
    totalPages,
    total: totalItems,
  } = usePaginatedList<AIHistoryEntry>({
    fetcher,
    initialPageSize: 10,
    enabled: Boolean(accessToken),
    fetchKey: accessToken ?? '',
    resetPageKey: filterKey,
    syncToUrl: 'page',
  })

  const [detailCache, setDetailCache] = useState<Record<string, AIHistoryEntry | null>>({})
  const [inFlightDetailId, setInFlightDetailId] = useState<string | null>(null)

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

  const getStatusBadge = (
    entry: AIHistoryEntry,
  ): { label: string; cls: AiHistoryStatusVariant } => {
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
    <div className={legacyLayoutClass('page-stack')}>
      <div className={legacyCardClass('card')}>
        <div className={legacyCardClass('card-intro')}>
          <h2>{t('query_history.title')}</h2>
          <p className={legacyCardClass('card-lead card-lead--single-line')}>
            {t('app.nav.query_history_desc')}
          </p>
        </div>

        <div className={cn(formRowClass, 'mb-5')}>
          <label className={legacyFormClass('form-field')} style={{ minWidth: '13rem' }}>
            <span className={legacyFormClass('form-label')}>{t('glossary.label_datasource')}</span>
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
          <label className={legacyFormClass('form-field')} style={{ minWidth: '13rem' }}>
            <span className={legacyFormClass('form-label')}>{t('glossary.label_model')}</span>
            <Select
              value={selectedModelId}
              options={[
                { value: '', label: t('glossary.option_all_models') },
                ...filteredModels.map((m) => ({ value: m.id, label: m.label ?? m.name })),
              ]}
              onChange={setSelectedModelId}
            />
          </label>
          <label className={legacyFormClass('form-field')} style={{ minWidth: '10rem' }}>
            <span className={legacyFormClass('form-label')}>{t('query_history.label_status')}</span>
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
          <div className={legacyFormClass('form-field')} style={{ flex: 1, minWidth: '15rem' }}>
            <span className={legacyFormClass('form-label')}>{t('common.search')}</span>
            <input
              type="text"
              placeholder={t('query_history.search_placeholder')}
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
              className={legacyFormClass('input')}
            />
          </div>
        </div>

        <DataState
          loading={loading}
          error={error}
          errorPrefix={t('common.error')}
          empty={totalItems === 0}
          emptyState={<EmptyState description={t('query_history.empty')} />}
        >
          <>
            <div className={aiHistoryTableWrapClass}>
              <table
                className={aiHistoryTableClass}
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
                          className={isExpanded ? aiHistoryRowExpandedClass : ''}
                          style={{
                            borderBottom: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
                          }}
                        >
                          <td
                            className={aiHistoryQuestionClass}
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
                            <span className={aiHistoryStatusClass(badge.cls)}>{badge.label}</span>
                          </td>
                          <td style={{ padding: '12px 16px', color: 'var(--text-primary)' }}>
                            {entry.confidence_score != null
                              ? `${(entry.confidence_score * 100).toFixed(0)}%`
                              : '—'}
                          </td>
                          <td
                            className={aiHistoryMonoClass}
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
                                className={legacyButtonClass('btn btn-sm btn-ghost')}
                                style={{ padding: '4px 8px', fontSize: '0.75rem' }}
                              >
                                {t('query_history.action_rerun')}
                              </button>
                              <button
                                type="button"
                                onClick={() => toggleDetail(entry.id)}
                                className={aiHistoryDetailBtnClass}
                                aria-expanded={isExpanded}
                                title={t('query_history.action_preview')}
                              >
                                {isExpanded ? '▲' : '▼'}
                              </button>
                            </div>
                          </td>
                        </tr>
                        {isExpanded && (
                          <tr className={aiHistoryDetailRowClass}>
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
                                <div className={aiHistoryDetailContentClass}>
                                  {detail.prompt_context != null && (
                                    <div className={aiHistoryDetailBlockClass}>
                                      <h4>{t('query_history.prompt')}</h4>
                                      <pre>{formatDetail(detail.prompt_context)}</pre>
                                    </div>
                                  )}
                                  {detail.ai_response != null && (
                                    <div className={aiHistoryDetailBlockClass}>
                                      <h4>{t('query_history.generated_sql')}</h4>
                                      <pre>{formatDetail(detail.ai_response)}</pre>
                                    </div>
                                  )}
                                  {detail.logical_query != null && (
                                    <div className={aiHistoryDetailBlockClass}>
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
        </DataState>
      </div>
    </div>
  )
}
