import '../../styles/sharing.css'

import { useCallback } from 'react'

import { deleteShare, listShares } from '../../api/admin'
import { useConfirm } from '../../hooks/useConfirm'
import { usePaginatedList } from '../../hooks/usePaginatedList'
import { errorMessage } from '../../hooks/usePaginatedListLogic'
import { useT } from '../../i18n'
import type { ResourceShare } from '../../types/auth'
import type { PageQuery } from '../../types/pagination'
import { useAuth } from '../auth/AuthProvider'
import { DataState } from '../ui/DataState'
import type { ColumnDef } from '../ui/DataTable'
import { DataTable } from '../ui/DataTable'
import { EmptyState } from '../ui/EmptyState'
import { Pagination } from '../ui/Pagination'

interface Props {
  resourceType?: string
  refreshKey?: number
}

export function SharedResourcesList({ resourceType, refreshKey }: Props) {
  const t = useT()
  const confirm = useConfirm()
  const { accessToken } = useAuth()

  const fetcher = useCallback(
    async (q: PageQuery) => {
      const res = await listShares(accessToken ?? '', {
        page: q.page,
        pageSize: q.pageSize,
        resourceType,
      })
      return { items: res.shares, total: res.total }
    },
    [accessToken, resourceType],
  )
  const {
    items: displayedItems,
    loading,
    error,
    setError,
    page: currentPage,
    setPage: setCurrentPage,
    pageSize,
    totalPages,
    total: totalItems,
    reload,
  } = usePaginatedList<ResourceShare>({
    fetcher,
    initialPageSize: 10,
    enabled: Boolean(accessToken),
    fetchKey: `${accessToken ?? ''}|${refreshKey ?? 0}`,
    resetPageKey: resourceType ?? '',
  })

  async function onRevoke(id: string) {
    if (!accessToken) {
      return
    }
    const ok = await confirm({
      title: t('admin.sharing.confirm_revoke'),
      variant: 'danger',
    })
    if (!ok) {
      return
    }
    try {
      await deleteShare(accessToken, id)
      reload()
    } catch (e) {
      setError(errorMessage(e))
    }
  }

  function permissionBadge(perm: string) {
    switch (perm) {
      case 'view':
        return t('admin.sharing.permission_view')
      case 'execute':
        return t('admin.sharing.permission_execute')
      case 'edit':
        return t('admin.sharing.permission_edit')
      default:
        return perm
    }
  }

  const shareColumns: ColumnDef<ResourceShare>[] = [
    {
      key: 'type',
      header: t('admin.sharing.resource_type'),
      cell: (share) => <span className="shared-list__type-badge">{share.resource_type}</span>,
    },
    {
      key: 'resource_id',
      header: t('admin.sharing.resource_id'),
      className: 'shared-list__mono',
      cell: (share) => `${share.resource_id.slice(0, 8)}…`,
    },
    {
      key: 'shared_with',
      header: t('admin.sharing.shared_with'),
      cell: (share) =>
        share.shared_with ? (
          <span className="shared-list__mono">{share.shared_with.slice(0, 8)}…</span>
        ) : share.workspace_id ? (
          <span className="shared-list__workspace">WS: {share.workspace_id.slice(0, 8)}…</span>
        ) : (
          '—'
        ),
    },
    {
      key: 'permission',
      header: t('admin.sharing.permission'),
      cell: (share) => (
        <span className={`shared-list__perm shared-list__perm--${share.permission}`}>
          {permissionBadge(share.permission)}
        </span>
      ),
    },
    {
      key: 'created_at',
      header: t('admin.sharing.created_at'),
      cell: (share) => new Date(share.created_at).toLocaleDateString(),
    },
    {
      key: 'actions',
      header: t('common.actions'),
      cell: (share) => (
        <button
          onClick={() => {
            void onRevoke(share.id)
          }}
          className="shared-list__revoke"
        >
          {t('common.delete')}
        </button>
      ),
    },
  ]

  return (
    <div className="shared-list">
      <div style={containerStyle}>
        <DataState
          loading={loading}
          error={error}
          empty={displayedItems.length === 0}
          emptyState={<EmptyState description={t('admin.sharing.empty')} />}
        >
          <DataTable
            columns={shareColumns}
            rows={displayedItems}
            rowKey={(share) => share.id}
            tableClassName="shared-list__table"
            headRowClassName=""
            headerCellClassName=""
            rowClassName=""
            cellClassName=""
          />
        </DataState>
        {displayedItems.length > 0 && (
          <Pagination
            currentPage={currentPage}
            totalPages={totalPages}
            onPageChange={setCurrentPage}
            totalItems={totalItems}
            itemsPerPage={pageSize}
          />
        )}
      </div>
    </div>
  )
}

const containerStyle: React.CSSProperties = {
  background: 'var(--bg-card, #ffffff)',
  border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
  borderRadius: 8,
  overflow: 'hidden',
  boxShadow: 'var(--shadow-sm, 0 1px 3px rgba(0,0,0,0.05))',
}
