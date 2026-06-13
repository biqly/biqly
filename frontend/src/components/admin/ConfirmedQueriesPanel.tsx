import { useCallback, useEffect, useMemo, useState } from 'react'

import {
  type ConfirmedQuery,
  deactivateConfirmedQuery,
  listConfirmedQueries,
} from '../../api/aiAdmin'
import { useConfirm } from '../../hooks/useConfirm'
import { useDatasources } from '../../hooks/useDatasources'
import { usePaginatedList } from '../../hooks/usePaginatedList'
import { useSortState } from '../../hooks/useSortState'
import { useToast } from '../../hooks/useToast'
import { useT } from '../../i18n'
import type { PageQuery } from '../../types/pagination'
import { Button } from '../ui/Button'
import type { ColumnDef } from '../ui/DataTable'
import { DataTable } from '../ui/DataTable'
import { LoadingScreen } from '../ui/LoadingScreen'
import { Pagination } from '../ui/Pagination'
import { Select } from '../ui/Select'
import {
  adminActiveBadgeClass,
  adminFormLabelClass,
  adminLabelTextClass,
  adminTableContainerClass,
  adminTdMonoClass,
  adminTextMutedClass,
} from './adminClasses'
import { datasourceSelectOptions } from './adminSelectOptions'

// ConfirmedQueriesPanel lists the NL→SQL pairs learned from thumbs-up feedback
// (the AI memory store) and lets admins pull a pair out of few-shot recall.
export function ConfirmedQueriesPanel() {
  const t = useT()
  const toast = useToast()
  const confirm = useConfirm()
  const { datasources, loading: loadingDS } = useDatasources()

  const [selectedDS, setSelectedDS] = useState('')
  const [deactivatingId, setDeactivatingId] = useState<string | null>(null)

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
    const ok = await confirm({
      title: t('admin.confirmed_queries.deactivate_confirm_title'),
      message: t('admin.confirmed_queries.deactivate_confirm_message'),
      variant: 'warning',
    })
    if (!ok) {
      return
    }
    setDeactivatingId(id)
    try {
      await deactivateConfirmedQuery(id)
      reload()
      toast.success(t('admin.confirmed_queries.deactivated'))
    } catch (e) {
      toast.error(e instanceof Error ? e.message : String(e))
    } finally {
      setDeactivatingId(null)
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
      cell: (row) => new Date(row.confirmed_at).toLocaleString(),
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
          <Button
            variant="ghost"
            size="sm"
            disabled={deactivatingId === row.id}
            onClick={() => void handleDeactivate(row.id)}
          >
            {t('admin.confirmed_queries.deactivate')}
          </Button>
        ),
    },
  ]

  return (
    <div className="page-stack">
      <div>
        <h2 style={{ margin: 0 }}>{t('admin.confirmed_queries.title')}</h2>
        <p className="form-hint" style={{ marginTop: 8 }}>
          {t('admin.confirmed_queries.description')}
        </p>
      </div>

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
        <p className={adminTextMutedClass}>{t('admin.confirmed_queries.empty')}</p>
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
            alwaysShow
          />
        </div>
      )}
    </div>
  )
}
