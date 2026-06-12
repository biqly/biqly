import { useCallback, useEffect, useMemo, useState } from 'react'

import {
  type ConfirmedQuery,
  deactivateConfirmedQuery,
  listConfirmedQueries,
} from '../../api/aiAdmin'
import { useClientPagination } from '../../hooks/useClientPagination'
import { useConfirm } from '../../hooks/useConfirm'
import { useDatasources } from '../../hooks/useDatasources'
import { useToast } from '../../hooks/useToast'
import { useT } from '../../i18n'
import { LoadingScreen } from '../ui/LoadingScreen'
import { Pagination } from '../ui/Pagination'
import { Select } from '../ui/Select'
import { datasourceSelectOptions } from './adminSelectOptions'

// ConfirmedQueriesPanel lists the NL→SQL pairs learned from thumbs-up feedback
// (the AI memory store) and lets admins pull a pair out of few-shot recall.
export function ConfirmedQueriesPanel() {
  const t = useT()
  const toast = useToast()
  const confirm = useConfirm()
  const { datasources, loading: loadingDS } = useDatasources()

  const [selectedDS, setSelectedDS] = useState('')
  const [rows, setRows] = useState<ConfirmedQuery[]>([])
  const [loading, setLoading] = useState(false)
  const [deactivatingId, setDeactivatingId] = useState<string | null>(null)

  const {
    page: currentPage,
    setPage: setCurrentPage,
    pageSize,
    totalPages,
    pageRows: displayedRows,
  } = useClientPagination(rows, 10)

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

  const load = useCallback(
    async (datasourceId: string) => {
      if (!datasourceId) {
        return
      }
      setLoading(true)
      try {
        setRows(await listConfirmedQueries(datasourceId))
      } catch (e) {
        toast.error(e instanceof Error ? e.message : String(e))
      } finally {
        setLoading(false)
      }
    },
    [toast],
  )

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load(selectedDS)
  }, [selectedDS, load])

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
      setRows((prev) => prev.map((r) => (r.id === id ? { ...r, is_active: false } : r)))
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

  return (
    <div className="page-stack">
      <div>
        <h2 style={{ margin: 0 }}>{t('admin.confirmed_queries.title')}</h2>
        <p className="form-hint" style={{ marginTop: 8 }}>
          {t('admin.confirmed_queries.description')}
        </p>
      </div>

      <label className="admin-form-label" style={{ gap: 4, maxWidth: 360 }}>
        <span className="admin-label-text">{t('admin.confirmed_queries.datasource')}</span>
        <Select
          value={selectedDS}
          onChange={handleDatasourceChange}
          options={dsOptions}
          ariaLabel={t('admin.confirmed_queries.datasource')}
        />
      </label>

      {loading ? (
        <LoadingScreen minHeight="160px" />
      ) : rows.length === 0 ? (
        <p className="admin-text-muted">{t('admin.confirmed_queries.empty')}</p>
      ) : (
        <div className="admin-table-container">
          <table className="admin-table" style={{ fontSize: 13, minWidth: 760 }}>
            <thead>
              <tr className="admin-thead-row">
                <th className="admin-th">{t('admin.confirmed_queries.col_question')}</th>
                <th className="admin-th">{t('admin.confirmed_queries.col_query')}</th>
                <th className="admin-th">{t('admin.confirmed_queries.col_confirmed_at')}</th>
                <th className="admin-th">{t('admin.confirmed_queries.col_status')}</th>
                <th className="admin-th">{t('admin.confirmed_queries.col_actions')}</th>
              </tr>
            </thead>
            <tbody>
              {displayedRows.map((row) => (
                <tr key={row.id}>
                  <td className="admin-td">{row.nl_query}</td>
                  <td className="admin-td-mono">
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
                  </td>
                  <td className="admin-td-mono">{new Date(row.confirmed_at).toLocaleString()}</td>
                  <td className="admin-td">
                    <span
                      className={row.is_active ? 'admin-badge-active' : 'admin-badge-inactive'}
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
                  </td>
                  <td className="admin-td">
                    {row.is_active && (
                      <button
                        type="button"
                        className="btn btn-sm btn-ghost"
                        disabled={deactivatingId === row.id}
                        onClick={() => void handleDeactivate(row.id)}
                      >
                        {t('admin.confirmed_queries.deactivate')}
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          <Pagination
            currentPage={currentPage}
            totalPages={totalPages}
            onPageChange={setCurrentPage}
            totalItems={rows.length}
            itemsPerPage={pageSize}
            alwaysShow
          />
        </div>
      )}
    </div>
  )
}
