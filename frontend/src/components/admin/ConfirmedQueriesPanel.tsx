import { useCallback, useEffect, useMemo, useState } from 'react'

import {
  type ConfirmedQuery,
  deactivateConfirmedQuery,
  listConfirmedQueries,
} from '../../api/aiAdmin'
import { useConfirmedMutation } from '../../hooks/useConfirmedMutation'
import { useDatasources } from '../../hooks/useDatasources'
import { useModal } from '../../hooks/useModal'
import { usePaginatedList } from '../../hooks/usePaginatedList'
import { useSortState } from '../../hooks/useSortState'
import { localeLanguageTag, useLocale, useT } from '../../i18n'
import type { PageQuery } from '../../types/pagination'
import { formatDateTime } from '../../utils/formatters'
import { DEFAULT_TABLE_PAGE_SIZE_OPTIONS } from '../../utils/paging'
import { Button } from '../ui/Button'
import type { ColumnDef } from '../ui/DataTable'
import { DataTable } from '../ui/DataTable'
import { EmptyState } from '../ui/EmptyState'
import { LoadingScreen } from '../ui/LoadingScreen'
import { Pagination } from '../ui/Pagination'
import { Select } from '../ui/Select'
import {
  adminActiveBadgeClass,
  adminFormLabelClass,
  adminLabelTextClass,
  adminTableContainerClass,
  adminTdMonoClass,
} from './adminClasses'
import { AdminPanelShell } from './AdminPanelShell'
import { datasourceSelectOptions } from './adminSelectOptions'

function ConfirmedQueryEmptyIcon() {
  return (
    <svg
      xmlns="http://www.w3.org/2000/svg"
      width="40"
      height="40"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d="M9 11l3 3L22 4" />
      <path d="M21 12v7a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V5a2 2 0 0 1 2-2h11" />
    </svg>
  )
}

// ConfirmedQueriesPanel lists the NL→SQL pairs learned from thumbs-up feedback
// (the AI memory store) and lets admins pull a pair out of few-shot recall.
export function ConfirmedQueriesPanel() {
  const t = useT()
  const [locale] = useLocale()
  const confirmMutation = useConfirmedMutation()
  const _modal = useModal()
  const { datasources, loading: loadingDS } = useDatasources()

  const [selectedDS, setSelectedDS] = useState('')

  const { sort, toggle: toggleSortKey } = useSortState()

  const fetcher = useCallback(
    async (q: PageQuery) => {
      const res = await listConfirmedQueries(selectedDS, {
        page: q.page,
        pageSize: q.pageSize,
        sort: sort?.key,
        order: sort?.dir,
      })
      return { items: res.queries, total: res.total }
    },
    [selectedDS, sort],
  )
  const {
    items: rows,
    loading,
    page: currentPage,
    setPage: setCurrentPage,
    pageSize,
    setPageSize,
    totalPages,
    total: totalItems,
    reload,
  } = usePaginatedList<ConfirmedQuery>({
    fetcher,
    initialPageSize: 10,
    enabled: Boolean(selectedDS),
    resetPageKey: `${selectedDS}|${sort?.key ?? ''}|${sort?.dir ?? ''}`,
    syncToUrl: 'confirmedQueriesPage',
  })

  const handleSortToggle = useCallback(
    (key: string) => {
      toggleSortKey(key)
      setCurrentPage(1)
    },
    [toggleSortKey, setCurrentPage],
  )

  const handleDatasourceChange = useCallback(
    (datasourceId: string) => {
      setSelectedDS(datasourceId)
      setCurrentPage(1)
    },
    [setCurrentPage],
  )

  useEffect(() => {
    const firstDS = datasources[0]
    if (firstDS && !selectedDS) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setSelectedDS(firstDS.id)
    }
  }, [datasources, selectedDS])

  const handleDeactivate = async (id: string) => {
    const ok = await confirmMutation(() => deactivateConfirmedQuery(id), {
      title: t('admin.confirmed_queries.deactivate_confirm_title'),
      message: t('admin.confirmed_queries.deactivate_confirm_message'),
      successMessage: t('admin.confirmed_queries.deactivated'),
      variant: 'warning',
    })
    if (ok) {
      reload()
    }
  }

  const dsOptions = useMemo(
    () => datasourceSelectOptions(datasources, loadingDS),
    [datasources, loadingDS],
  )

  const queryColumns: ColumnDef<ConfirmedQuery>[] = [
    {
      key: 'question',
      header: t('admin.confirmed_queries.col_question'),
      sortable: true,
      cell: (row) => row.nl_query,
    },
    {
      key: 'query',
      header: t('admin.confirmed_queries.col_query'),
      className: adminTdMonoClass,
      cell: (row) => (
        <code
          style={{
            display: 'inline-block',
            maxWidth: 320,
            overflow: 'hidden',
            textOverflow: 'ellipsis',
            whiteSpace: 'nowrap',
            verticalAlign: 'bottom',
          }}
          title={row.sql_query}
        >
          {row.sql_query}
        </code>
      ),
    },
    {
      key: 'confirmed_at',
      header: t('admin.confirmed_queries.col_confirmed_at'),
      className: adminTdMonoClass,
      sortable: true,
      cell: (row) => formatDateTime(row.confirmed_at, localeLanguageTag(locale)),
    },
    {
      key: 'status',
      header: t('admin.confirmed_queries.col_status'),
      sortable: true,
      cell: (row) => (
        <span
          className={adminActiveBadgeClass(row.is_active)}
          aria-label={
            row.is_active
              ? t('admin.confirmed_queries.status_active_aria')
              : t('admin.confirmed_queries.status_inactive_aria')
          }
        >
          {row.is_active
            ? t('admin.confirmed_queries.status_active')
            : t('admin.confirmed_queries.status_inactive')}
        </span>
      ),
    },
    {
      key: 'actions',
      header: t('admin.confirmed_queries.col_actions'),
      cell: (row) =>
        row.is_active && (
          <Button variant="ghost" size="sm" onClick={() => void handleDeactivate(row.id)}>
            {t('admin.confirmed_queries.deactivate')}
          </Button>
        ),
    },
  ]

  return (
    <AdminPanelShell
      title={t('admin.confirmed_queries.title')}
      description={t('admin.confirmed_queries.description')}
      action={
        totalItems > 0 && (
          <span className="text-foreground-muted text-caption">
            {t('admin.confirmed_queries.row_count', { count: totalItems })}
          </span>
        )
      }
    >
      <label className={adminFormLabelClass} style={{ gap: 4, maxWidth: 360 }}>
        <span className={adminLabelTextClass}>{t('admin.confirmed_queries.datasource')}</span>
        <Select
          value={selectedDS}
          onChange={handleDatasourceChange}
          options={dsOptions}
          ariaLabel={t('admin.confirmed_queries.datasource')}
        />
      </label>

      {loading ? (
        <LoadingScreen minHeight="160px" />
      ) : totalItems === 0 ? (
        <EmptyState
          title={t('admin.confirmed_queries.empty_title')}
          description={t('admin.confirmed_queries.empty')}
          icon={<ConfirmedQueryEmptyIcon />}
        />
      ) : (
        <div className={adminTableContainerClass}>
          <DataTable
            columns={queryColumns}
            rows={rows}
            rowKey={(row) => row.id}
            rowClassName=""
            tableStyle={{ fontSize: 13, minWidth: 760 }}
            sort={sort}
            onSortToggle={handleSortToggle}
          />
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
            alwaysShow
          />
        </div>
      )}
    </AdminPanelShell>
  )
}
