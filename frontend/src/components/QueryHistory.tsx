import { Fragment, useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import { getAIHistoryDetail, listAIHistory } from '../api/admin'
import { useDatasources } from '../hooks/useDatasources'
import { useDebouncedValue } from '../hooks/useDebouncedValue'
import { usePaginatedList } from '../hooks/usePaginatedList'
import { useSemanticModels } from '../hooks/useSemanticModels'
import { useToast } from '../hooks/useToast'
import { localeLanguageTag, useLocale, useT } from '../i18n'
import { buttonClass } from '../lib/buttonClasses'
import { cardClass } from '../lib/cardClasses'
import { cn } from '../lib/cn'
import { formRowClass, legacyFormClass } from '../lib/formClasses'
import { legacyLayoutClass } from '../lib/layoutClasses'
import type { AIHistoryEntry } from '../types/auth'
import type { PageQuery } from '../types/pagination'
import { pickValidId } from '../utils/effectiveSelection'
import { formatDateTime } from '../utils/formatters'
import { DEFAULT_TABLE_PAGE_SIZE_OPTIONS } from '../utils/paging'
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
  const [locale] = useLocale()
  const toast = useToast()
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
    setPageSize,
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
    const controller = new AbortController()
    void Promise.resolve().then(() => {
      if (!controller.signal.aborted) {
        setInFlightDetailId(expandedId)
      }
    })
    getAIHistoryDetail(accessToken, expandedId)
      .then((data) => {
        if (!controller.signal.aborted) {
          setDetailCache((prev) => ({ ...prev, [expandedId]: data }))
        }
      })
      .catch(() => {
        if (!controller.signal.aborted) {
          setDetailCache((prev) => ({ ...prev, [expandedId]: null }))
          toast.error(t('query_history.detail_load_error'))
        }
      })
      .finally(() => {
        if (!controller.signal.aborted) {
          setInFlightDetailId((prev) => (prev === expandedId ? null : prev))
        }
      })
    return () => {
      controller.abort()
    }
  }, [expandedId, accessToken, detailCache, t, toast])

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
      <div className={cardClass()}>
        <div className={cn(formRowClass, 'mb-5')}>
          <label className={cn(legacyFormClass('form-field'), 'min-w-52')}>
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
          <label className={cn(legacyFormClass('form-field'), 'min-w-52')}>
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
          <label className={cn(legacyFormClass('form-field'), 'min-w-40')}>
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
          <div className={cn(legacyFormClass('form-field'), 'min-w-60 flex-1')}>
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
              <table className={`${aiHistoryTableClass} w-full border-collapse`}>
                <thead>
                  <tr
                    style={{
                      background: 'var(--table-header-bg)',
                      borderBottom: '1px solid var(--border)',
                    }}
                    className="text-left"
                  >
                    <th className="p-3 font-semibold text-(--table-header-fg)">
                      {t('query_history.col_question')}
                    </th>
                    <th className="p-3 font-semibold text-(--table-header-fg)">
                      {t('query_history.col_status')}
                    </th>
                    <th className="p-3 font-semibold text-(--table-header-fg)">
                      {t('query_history.col_confidence')}
                    </th>
                    <th className="p-3 font-semibold text-(--table-header-fg)">
                      {t('query_history.col_model')}
                    </th>
                    <th className="p-3 font-semibold text-(--table-header-fg)">
                      {t('query_history.col_latency')}
                    </th>
                    <th className="p-3 font-semibold text-(--table-header-fg)">
                      {t('query_history.col_tokens')}
                    </th>
                    <th className="p-3 font-semibold text-(--table-header-fg)">
                      {t('query_history.col_created_at')}
                    </th>
                    <th className="p-3 font-semibold text-(--table-header-fg)">
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
                          className={cn(
                            isExpanded ? aiHistoryRowExpandedClass : '',
                            'border-border border-b',
                          )}
                        >
                          <td className={cn(aiHistoryQuestionClass, 'text-foreground p-3')}>
                            <div className="font-medium">{entry.question || '—'}</div>
                            <div className="text-foreground-muted mt-0.5 text-xs">
                              {datasourceName}
                            </div>
                          </td>
                          <td className="p-3">
                            <span className={aiHistoryStatusClass(badge.cls)}>{badge.label}</span>
                          </td>
                          <td className="text-foreground p-3">
                            {entry.confidence_score != null
                              ? `${(entry.confidence_score * 100).toFixed(0)}%`
                              : '—'}
                          </td>
                          <td className={cn(aiHistoryMonoClass, 'text-foreground p-3')}>
                            {modelName}
                          </td>
                          <td className="text-foreground p-3">
                            {entry.latency_ms != null ? `${entry.latency_ms}ms` : '—'}
                          </td>
                          <td className="text-foreground p-3">{entry.token_count ?? '—'}</td>
                          <td className="text-foreground p-3 text-[0.8rem]">
                            {formatDateTime(entry.created_at, localeLanguageTag(locale))}
                          </td>
                          <td className="p-3 text-right">
                            <div className="flex items-center justify-end gap-2">
                              <button
                                type="button"
                                onClick={() => handleRerun(entry.question)}
                                className={cn(
                                  buttonClass('ghost', { size: 'sm' }),
                                  'px-2 py-1 text-xs',
                                )}
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
                                <div className="relative flex min-h-21.25 items-center justify-center">
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
                                <p className="text-foreground-faint p-4">—</p>
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
              pageSizeOptions={DEFAULT_TABLE_PAGE_SIZE_OPTIONS}
              onPageSizeChange={(size) => {
                setPageSize(size)
                setCurrentPage(1)
              }}
            />
          </>
        </DataState>
      </div>
    </div>
  )
}
