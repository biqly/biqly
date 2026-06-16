import { useCallback } from 'react'

import { deleteShare, listShares } from '../../api/admin'
import { useConfirm } from '../../hooks/useConfirm'
import { usePaginatedList } from '../../hooks/usePaginatedList'
import { errorMessage } from '../../hooks/usePaginatedListLogic'
import { localeLanguageTag, useLocale, useT } from '../../i18n'
import { legacyCardClass } from '../../lib/cardClasses'
import { cn } from '../../lib/cn'
import { legacyFeedbackClass } from '../../lib/feedbackClasses'
import type { ResourceShare } from '../../types/auth'
import type { PageQuery } from '../../types/pagination'
import { formatDateOnly } from '../../utils/formatters'
import { DEFAULT_TABLE_PAGE_SIZE_OPTIONS } from '../../utils/paging'
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
  const [locale] = useLocale()
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
    setPageSize,
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
    let styleClass = ''
    let label = perm
    switch (perm) {
      case 'view':
        styleClass = 'bg-foreground-muted/10 text-foreground-muted'
        label = t('admin.sharing.permission_view')
        break
      case 'execute':
        styleClass = 'bg-warning/10 text-warning'
        label = t('admin.sharing.permission_execute')
        break
      case 'edit':
        styleClass = 'bg-success/10 text-success'
        label = t('admin.sharing.permission_edit')
        break
    }
    return (
      <span
        className={cn(
          'text-2xs inline-block rounded-[10px] px-2 py-0.5 font-semibold whitespace-nowrap',
          styleClass,
        )}
      >
        {label}
      </span>
    )
  }

  const shareColumns: ColumnDef<ResourceShare>[] = [
    {
      key: 'type',
      header: t('admin.sharing.resource_type'),
      cell: (share) => (
        <span className="text-accent text-2xs inline-block rounded-[10px] bg-(--accent-glow) px-2 py-0.5 font-medium">
          {share.resource_type}
        </span>
      ),
    },
    {
      key: 'resource_id',
      header: t('admin.sharing.resource_id'),
      className: 'font-mono text-xs',
      cell: (share) => `${share.resource_id.slice(0, 8)}…`,
    },
    {
      key: 'shared_with',
      header: t('admin.sharing.shared_with'),
      cell: (share) =>
        share.shared_with ? (
          <span className="font-mono text-xs">{share.shared_with.slice(0, 8)}…</span>
        ) : share.workspace_id ? (
          <span className="text-accent text-xs">WS: {share.workspace_id.slice(0, 8)}…</span>
        ) : (
          '—'
        ),
    },
    {
      key: 'permission',
      header: t('admin.sharing.permission'),
      cell: (share) => permissionBadge(share.permission),
    },
    {
      key: 'created_at',
      header: t('admin.sharing.created_at'),
      cell: (share) => formatDateOnly(share.created_at, localeLanguageTag(locale)),
    },
    {
      key: 'actions',
      header: t('common.actions'),
      cell: (share) => (
        <button
          onClick={() => {
            void onRevoke(share.id)
          }}
          className={legacyFeedbackClass(
            'text-error hover:bg-error/6 cursor-pointer rounded-sm border border-[rgba(239,68,68,0.3)] bg-transparent px-2.5 py-0.75 text-xs transition-colors',
          )}
        >
          {t('common.delete')}
        </button>
      ),
    },
  ]

  return (
    <div className="overflow-x-auto">
      <div
        className={legacyCardClass(
          'bg-card border-border shadow-card-sm overflow-hidden rounded-lg border',
        )}
      >
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
            tableClassName="w-full border-collapse text-caption"
            headRowClassName=""
            headerCellClassName="py-2 px-2.5 text-left border-b border-border font-semibold text-2xs uppercase tracking-[0.4px] text-foreground-muted"
            rowClassName=""
            cellClassName="py-2 px-2.5 text-left border-b border-border"
          />
        </DataState>
        {displayedItems.length > 0 && (
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
        )}
      </div>
    </div>
  )
}
