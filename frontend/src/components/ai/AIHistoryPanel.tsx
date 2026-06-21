import { Fragment, useCallback, useState } from 'react'

import { getAIHistoryDetail, listAIHistory } from '../../api/admin'
import { useFetch } from '../../hooks/useFetch'
import { usePaginatedList } from '../../hooks/usePaginatedList'
import { useQueryParam } from '../../hooks/useQueryParam'
import { localeLanguageTag, useLocale, useT } from '../../i18n'
import type { AIHistoryEntry } from '../../types/auth'
import type { PageQuery } from '../../types/pagination'
import { formatDateTime } from '../../utils/formatters'
import { DEFAULT_TABLE_PAGE_SIZE_OPTIONS } from '../../utils/paging'
import { useAuth } from '../auth/AuthProvider'
import { ShareButton } from '../sharing/ShareButton'
import { DataState } from '../ui/DataState'
import { EmptyState } from '../ui/EmptyState'
import { LoadingOverlay } from '../ui/LoadingOverlay'
import { Pagination } from '../ui/Pagination'
import {
  aiHistoryActionsClass,
  aiHistoryClass,
  aiHistoryDetailBlockClass,
  aiHistoryDetailBtnClass,
  aiHistoryDetailCellClass,
  aiHistoryDetailCloseBtnClass,
  aiHistoryDetailContentClass,
  aiHistoryDetailHeaderClass,
  aiHistoryDetailRowClass,
  aiHistoryHeaderClass,
  aiHistoryMonoClass,
  aiHistoryQuestionClass,
  aiHistoryRowExpandedClass,
  aiHistoryStatusClass,
  type AiHistoryStatusVariant,
  aiHistoryTableClass,
  aiHistoryTableWrapClass,
  aiHistoryToggleClass,
} from './aiJobsClasses'

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
  const [locale] = useLocale()
  const languageTag = localeLanguageTag(locale)
  const { accessToken, roles } = useAuth()
  const [showAll, setShowAll] = useState(false)
  const [historyIdParam, setHistoryIdParam] = useQueryParam('historyId')
  const expandedId = historyIdParam || null

  const isAdmin = roles.some((r) => r === 'super_admin' || r === 'admin')

  const fetcher = useCallback(
    async (q: PageQuery) => {
      const res = await listAIHistory(accessToken ?? '', {
        page: q.page,
        pageSize: q.pageSize,
        showAll: isAdmin ? showAll : undefined,
      })
      return { items: res.entries, total: res.total }
    },
    [accessToken, showAll, isAdmin],
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
    fetchKey: `${accessToken ?? ''}|${isAdmin}`,
    resetPageKey: showAll,
  })

  const { data: detail, loading: detailLoading } = useFetch(
    () => getAIHistoryDetail(accessToken ?? '', expandedId ?? ''),
    [expandedId, accessToken],
    { enabled: Boolean(expandedId && accessToken) },
  )

  function toggleDetail(id: string) {
    if (expandedId === id) {
      setHistoryIdParam('')
    } else {
      setHistoryIdParam(id)
    }
  }

  function statusBadge(entry: AIHistoryEntry): { label: string; cls: AiHistoryStatusVariant } {
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
    <div className={aiHistoryClass}>
      <div className={aiHistoryHeaderClass}>
        <h2>{t('admin.ai_history.title')}</h2>
        {isAdmin && (
          <label className={aiHistoryToggleClass}>
            <input
              type="checkbox"
              checked={showAll}
              onChange={(e) => setShowAll(e.target.checked)}
            />
            <span>{t('admin.ai_history.show_all_users')}</span>
          </label>
        )}
      </div>

      <div style={containerStyle}>
        <DataState
          loading={loading}
          error={error}
          empty={entries.length === 0}
          emptyState={<EmptyState description={t('admin.ai_history.empty')} />}
        >
          <>
            <div className={aiHistoryTableWrapClass}>
              <table className={aiHistoryTableClass}>
                <colgroup>
                  <col className="w-[26%]" />
                  <col className="w-[10%]" />
                  <col className="w-[8%]" />
                  <col className="w-[12%]" />
                  <col className="w-[8%]" />
                  <col className="w-[10%]" />
                  <col className="w-[16%]" />
                  <col className="w-[10%]" />
                </colgroup>
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
                  {entries.map((entry) => {
                    const badge = statusBadge(entry)
                    const isExpanded = expandedId === entry.id
                    return (
                      <Fragment key={entry.id}>
                        <tr className={isExpanded ? aiHistoryRowExpandedClass : ''} style={trStyle}>
                          <td className={aiHistoryQuestionClass} style={tdStyle}>
                            {entry.question || '—'}
                          </td>
                          <td style={tdStyle}>
                            <span className={aiHistoryStatusClass(badge.cls)}>{badge.label}</span>
                          </td>
                          <td style={tdStyle}>
                            {entry.confidence_score != null
                              ? `${(entry.confidence_score * 100).toFixed(0)}%`
                              : '—'}
                          </td>
                          <td
                            className={aiHistoryMonoClass}
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
                          <td style={tdStyle}>{formatDateTime(entry.created_at, languageTag)}</td>
                          <td style={{ ...tdStyle, textAlign: 'right' }}>
                            <div className={aiHistoryActionsClass}>
                              <ShareButton resourceType="query" resourceID={entry.id} />
                              <button
                                onClick={() => toggleDetail(entry.id)}
                                className={aiHistoryDetailBtnClass}
                                aria-expanded={isExpanded}
                                title={t('admin.ai_history.detail')}
                              >
                                {isExpanded ? '▲' : '▼'}
                              </button>
                            </div>
                          </td>
                        </tr>
                        {isExpanded && (
                          <tr className={aiHistoryDetailRowClass}>
                            <td colSpan={8} className={aiHistoryDetailCellClass}>
                              {detailLoading ? (
                                <div className="relative flex min-h-22 items-center justify-center">
                                  <LoadingOverlay loading={true} />
                                </div>
                              ) : detail ? (
                                <div className={aiHistoryDetailContentClass}>
                                  <div className={aiHistoryDetailHeaderClass}>
                                    <span className="text-foreground-muted text-2xs font-semibold tracking-wide uppercase">
                                      {t('admin.ai_history.detail')}
                                    </span>
                                    <button
                                      type="button"
                                      className={aiHistoryDetailCloseBtnClass}
                                      onClick={() => toggleDetail(entry.id)}
                                    >
                                      {t('common.close')}
                                    </button>
                                  </div>
                                  {detail.prompt_context != null && (
                                    <div className={aiHistoryDetailBlockClass}>
                                      <h4>{t('admin.ai_history.prompt')}</h4>
                                      <pre>{formatDetail(detail.prompt_context)}</pre>
                                    </div>
                                  )}
                                  {detail.ai_response != null && (
                                    <div className={aiHistoryDetailBlockClass}>
                                      <h4>{t('admin.ai_history.generated_sql')}</h4>
                                      <pre>{formatDetail(detail.ai_response)}</pre>
                                    </div>
                                  )}
                                  {detail.logical_query != null && (
                                    <div className={aiHistoryDetailBlockClass}>
                                      <h4>{t('admin.ai_history.logical_query')}</h4>
                                      <pre>{formatDetail(detail.logical_query)}</pre>
                                    </div>
                                  )}
                                </div>
                              ) : (
                                <p className="px-4 py-3">—</p>
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
