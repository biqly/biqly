import { useCallback, useState } from 'react'

import { createWorkspace, deleteWorkspace, listWorkspaces } from '../../api/admin'
import { useConfirmedMutation } from '../../hooks/useConfirmedMutation'
import { usePaginatedList } from '../../hooks/usePaginatedList'
import { errorMessage } from '../../hooks/usePaginatedListLogic'
import { useQueryParam } from '../../hooks/useQueryParam'
import { useT } from '../../i18n'
import { cn } from '../../lib/cn'
import type { Workspace } from '../../types/auth'
import type { PageQuery } from '../../types/pagination'
import { DEFAULT_TABLE_PAGE_SIZE_OPTIONS } from '../../utils/paging'
import { DataState } from '../ui/DataState'
import { FormField } from '../ui/FormField'
import { Pagination } from '../ui/Pagination'
import { WorkspaceSettingsPage } from '../workspaces/WorkspaceSettingsPage'
import { adminBtnPrimaryClass } from './adminClasses'
import { AdminPanelShell } from './AdminPanelShell'

export function WorkspacesPanel({ token }: { token: string }) {
  const t = useT()
  const confirmMutation = useConfirmedMutation()
  const [newName, setNewName] = useState('')
  const [newDesc, setNewDesc] = useState('')
  const [selectedWSParam, setSelectedWSParam] = useQueryParam('workspaceId')
  const [, setWorkspaceLabelParam] = useQueryParam('workspaceLabel')
  const selectedWS = selectedWSParam || null

  const fetcher = useCallback(
    async (q: PageQuery) => {
      const res = await listWorkspaces(token, q.page, q.pageSize)
      return { items: res.workspaces, total: res.total }
    },
    [token],
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
  } = usePaginatedList<Workspace>({ fetcher, initialPageSize: 10, fetchKey: token })

  async function onCreate(e: React.SubmitEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!newName.trim()) {
      return
    }
    try {
      await createWorkspace(token, newName.trim(), newDesc.trim() || undefined)
      setNewName('')
      setNewDesc('')
      if (currentPage !== 1) {
        setCurrentPage(1)
      } else {
        reload()
      }
    } catch (e) {
      setError(errorMessage(e))
    }
  }

  async function onDelete(id: string, name: string) {
    const ok = await confirmMutation(() => deleteWorkspace(token, id), {
      title: t('admin.workspaces.confirm_delete', { name }),
      successMessage: t('admin.workspaces.deleted'),
      variant: 'danger',
    })
    if (ok) {
      reload()
    }
  }

  if (selectedWS) {
    return <WorkspaceSettingsPage token={token} workspaceID={selectedWS} />
  }

  return (
    <AdminPanelShell
      title={t('admin.workspaces.title')}
      description={t('admin.workspaces.description')}
    >
      <form
        onSubmit={(e) => {
          void onCreate(e)
        }}
        className="flex flex-wrap items-end gap-3"
      >
        <FormField
          label={t('admin.workspaces.name')}
          value={newName}
          onChange={setNewName}
          required
        />
        <FormField label={t('admin.workspaces.desc_label')} value={newDesc} onChange={setNewDesc} />
        <button type="submit" className={adminBtnPrimaryClass}>
          {t('common.create')}
        </button>
      </form>

      <div className="bg-card border-border overflow-hidden rounded-lg border shadow-sm">
        <DataState
          loading={loading}
          error={error}
          errorPrefix={t('common.error')}
          empty={displayedItems.length === 0}
        >
          <ul className="flex list-none flex-col p-0">
            {displayedItems.length === 0 ? (
              <li className="text-foreground-faint p-6 text-center text-sm">
                {loading ? '' : '—'}
              </li>
            ) : (
              displayedItems.map((w, i) => (
                <li
                  key={w.id}
                  className={cn(
                    'flex flex-wrap items-center justify-between gap-3 px-5 py-4',
                    !(i === displayedItems.length - 1 && totalPages <= 1) &&
                      'border-border/45 border-b',
                  )}
                >
                  <div className="flex flex-col gap-1">
                    <div className="flex items-center gap-2">
                      <strong className="text-foreground text-sm font-semibold">{w.name}</strong>
                      <span
                        className="ws-settings__badge text-micro px-1.5 py-0.5"
                        data-type={w.is_personal ? 'personal' : 'team'}
                      >
                        {w.is_personal
                          ? t('admin.workspaces.type_personal')
                          : t('admin.workspaces.type_team')}
                      </span>
                    </div>
                    {w.description && (
                      <div className="text-foreground-muted text-sm">{w.description}</div>
                    )}
                    <div className="text-caption text-foreground-muted font-mono">{w.slug}</div>
                  </div>
                  <div className="flex gap-2">
                    <button
                      onClick={() => {
                        setSelectedWSParam(w.id)
                        setWorkspaceLabelParam(w.name)
                      }}
                      className="border-accent text-accent hover:bg-accent/5 cursor-pointer rounded-md border bg-transparent px-3 py-1.5 text-sm font-medium"
                    >
                      {t('admin.workspaces.settings')}
                    </button>
                    {!w.is_personal && (
                      <button
                        onClick={() => {
                          void onDelete(w.id, w.name)
                        }}
                        className="border-border text-error hover:bg-error/5 hover:border-error cursor-pointer rounded-md border bg-transparent px-3 py-1.5 text-sm"
                      >
                        {t('common.delete')}
                      </button>
                    )}
                  </div>
                </li>
              ))
            )}
          </ul>
        </DataState>
        <Pagination
          currentPage={currentPage}
          totalPages={totalPages}
          onPageChange={setCurrentPage}
          totalItems={totalItems}
          itemsPerPage={pageSize}
          alwaysShow
          pageSizeOptions={DEFAULT_TABLE_PAGE_SIZE_OPTIONS}
          onPageSizeChange={(size) => {
            setPageSize(size)
            setCurrentPage(1)
          }}
        />
      </div>
    </AdminPanelShell>
  )
}

// CSS-in-JS style objects removed and migrated to Tailwind classes.
