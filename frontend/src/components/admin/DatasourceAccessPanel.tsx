import { useCallback, useMemo, useState } from 'react'

import {
  grantDatasourceAccess,
  listDatasourceAccess,
  revokeDatasourceAccess,
  updateDatasourceAccess,
} from '../../api/admin'
import { useAdminLookups } from '../../hooks/useAdminLookups'
import { useConfirm } from '../../hooks/useConfirm'
import { usePaginatedList } from '../../hooks/usePaginatedList'
import { errorMessage } from '../../hooks/usePaginatedListLogic'
import { localeLanguageTag, useLocale, useT } from '../../i18n'
import type { DatasourceAccess } from '../../types/auth'
import type { PageQuery } from '../../types/pagination'
import { formatDateTime } from '../../utils/formatters'
import { useAuth } from '../auth/AuthProvider'
import { DataState } from '../ui/DataState'
import type { ColumnDef } from '../ui/DataTable'
import { DataTable } from '../ui/DataTable'
import { Pagination } from '../ui/Pagination'
import { Select } from '../ui/Select'
import {
  adminBtnPrimaryClass,
  adminBtnSecondaryClass,
  adminFormLabelClass,
  adminLabelTextClass,
  adminLevelClass,
  adminTableContainerClass,
  adminTdMonoClass,
} from './adminClasses'
import type { DatasourceAccessLevel } from './adminSelectOptions'
import {
  datasourceAccessLevelOptions,
  datasourcePickerOptions,
  userSelectOptions,
} from './adminSelectOptions'
import { ReadOnlyNote } from './ReadOnlyNote'

export function DatasourceAccessPanel({ token }: { token: string }) {
  const t = useT()
  const [locale] = useLocale()
  const confirm = useConfirm()
  const { hasPermission } = useAuth()
  // Granting/revoking datasource access requires datasource:grant_access (server-enforced).
  const canEdit = hasPermission('datasource:grant_access')

  const [userID, setUserID] = useState('')
  const [datasourceID, setDatasourceID] = useState('')
  const [level, setLevel] = useState<'read' | 'write' | 'admin'>('read')

  // Lookups for friendly name mapping using custom hook
  const { users, datasources } = useAdminLookups(token)

  const fetcher = useCallback(
    async (q: PageQuery) => {
      const res = await listDatasourceAccess(token, q.page, q.pageSize)
      return { items: res.access, total: res.total }
    },
    [token],
  )
  const {
    items: rows,
    loading,
    error,
    setError,
    page: currentPage,
    setPage: setCurrentPage,
    pageSize,
    totalPages,
    total: totalItems,
    reload,
  } = usePaginatedList<DatasourceAccess>({ fetcher, initialPageSize: 10, fetchKey: token })

  const userOptions = useMemo(
    () => userSelectOptions(users, t('evaluation.placeholder_select')),
    [users, t],
  )
  const dsOptions = useMemo(
    () => datasourcePickerOptions(datasources, t('evaluation.placeholder_select')),
    [datasources, t],
  )
  const levelOptions = useMemo(() => datasourceAccessLevelOptions(), [])

  async function onGrant(e: React.SubmitEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!userID || !datasourceID) {
      return
    }
    try {
      await grantDatasourceAccess(token, userID, datasourceID, level)
      setUserID('')
      setDatasourceID('')
      setCurrentPage(1)
      reload()
    } catch (e) {
      setError(errorMessage(e))
    }
  }

  async function onRevoke(uid: string, dsid: string) {
    const ok = await confirm({
      title: t('admin.datasource_access.confirm_revoke'),
      variant: 'danger',
    })
    if (!ok) {
      return
    }
    try {
      await revokeDatasourceAccess(token, uid, dsid)
      setCurrentPage(1)
      reload()
    } catch (e) {
      setError(errorMessage(e))
    }
  }

  async function onChangeLevel(id: string, newLevel: 'read' | 'write' | 'admin') {
    try {
      await updateDatasourceAccess(token, id, newLevel)
      reload()
    } catch (e) {
      setError(errorMessage(e))
    }
  }

  const accessColumns: ColumnDef<DatasourceAccess>[] = [
    {
      key: 'user',
      header: t('admin.fields.user'),
      className: adminTdMonoClass,
      cell: (r) => {
        const userObj = users.find((u) => u.id === r.user_id)
        return userObj ? userObj.email : r.user_id
      },
    },
    {
      key: 'datasource',
      header: 'Datasource',
      className: adminTdMonoClass,
      cell: (r) => {
        const dsObj = datasources.find((d) => d.id === r.datasource_id)
        return dsObj ? dsObj.name : r.datasource_id
      },
    },
    {
      key: 'level',
      header: t('admin.datasource_access.level'),
      cell: (r) => (
        <Select
          size="sm"
          value={r.access_level}
          options={levelOptions}
          onChange={(v) => {
            void onChangeLevel(r.id, v as DatasourceAccessLevel)
          }}
          className={adminLevelClass(r.access_level)}
          disabled={!canEdit}
        />
      ),
    },
    {
      key: 'granted_at',
      header: t('admin.datasource_access.granted_at'),
      cell: (r) => formatDateTime(r.granted_at, localeLanguageTag(locale)),
    },
    {
      key: 'actions',
      header: '',
      align: 'right',
      cell: (r) => (
        <button
          onClick={() => {
            void onRevoke(r.user_id, r.datasource_id)
          }}
          className={adminBtnSecondaryClass}
          disabled={!canEdit}
        >
          {t('common.delete')}
        </button>
      ),
    },
  ]

  return (
    <div className="page-stack">
      <h2 style={{ marginTop: 0, fontSize: 18 }}>{t('admin.datasource_access.title')}</h2>

      {!canEdit && <ReadOnlyNote />}

      <form
        onSubmit={(e) => {
          void onGrant(e)
        }}
        style={{ display: 'flex', gap: 12, alignItems: 'flex-end', flexWrap: 'wrap' }}
      >
        <label className={adminFormLabelClass} style={{ gap: 4, minWidth: 240 }}>
          <span className={adminLabelTextClass}>{t('admin.fields.user')}</span>
          <Select
            value={userID}
            options={userOptions}
            onChange={setUserID}
            placeholder={t('evaluation.placeholder_select')}
            disabled={!canEdit}
          />
        </label>
        <label className={adminFormLabelClass} style={{ gap: 4, minWidth: 240 }}>
          <span className={adminLabelTextClass}>Datasource</span>
          <Select
            value={datasourceID}
            options={dsOptions}
            onChange={setDatasourceID}
            placeholder={t('evaluation.placeholder_select')}
            disabled={!canEdit}
          />
        </label>
        <label className={adminFormLabelClass} style={{ gap: 4, minWidth: 240 }}>
          <span className={adminLabelTextClass}>{t('admin.datasource_access.level')}</span>
          <Select
            value={level}
            options={levelOptions}
            onChange={(v) => setLevel(v as DatasourceAccessLevel)}
            disabled={!canEdit}
          />
        </label>
        <button type="submit" className={adminBtnPrimaryClass} disabled={!canEdit}>
          {t('admin.datasource_access.grant')}
        </button>
      </form>

      <div className={adminTableContainerClass}>
        <DataState
          loading={loading}
          error={error}
          errorPrefix={t('common.error')}
          empty={rows.length === 0}
        >
          <DataTable columns={accessColumns} rows={rows} rowKey={(r) => r.id} loading={loading} />
        </DataState>
        <Pagination
          currentPage={currentPage}
          totalPages={totalPages}
          onPageChange={setCurrentPage}
          totalItems={totalItems}
          itemsPerPage={pageSize}
          alwaysShow
        />
      </div>
    </div>
  )
}
