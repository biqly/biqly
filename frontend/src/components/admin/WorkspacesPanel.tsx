import { useCallback, useState } from 'react'

import { createWorkspace, deleteWorkspace, listWorkspaces } from '../../api/admin'
import { useConfirm } from '../../hooks/useConfirm'
import { usePaginatedList } from '../../hooks/usePaginatedList'
import { errorMessage } from '../../hooks/usePaginatedListLogic'
import { useQueryParam } from '../../hooks/useQueryParam'
import { useT } from '../../i18n'
import type { Workspace } from '../../types/auth'
import type { PageQuery } from '../../types/pagination'
import { DataState } from '../ui/DataState'
import { FormField } from '../ui/FormField'
import { Pagination } from '../ui/Pagination'
import { WorkspaceSettingsPage } from '../workspaces/WorkspaceSettingsPage'

export function WorkspacesPanel({ token }: { token: string }) {
  const t = useT()
  const confirm = useConfirm()
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
    const ok = await confirm({
      title: t('admin.workspaces.confirm_delete', { name }),
      variant: 'danger',
    })
    if (!ok) {
      return
    }
    try {
      await deleteWorkspace(token, id)
      reload()
    } catch (e) {
      setError(errorMessage(e))
    }
  }

  if (selectedWS) {
    return <WorkspaceSettingsPage token={token} workspaceID={selectedWS} />
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <h2 style={{ marginTop: 0, fontSize: 18 }}>{t('admin.workspaces.title')}</h2>

      <form
        onSubmit={(e) => {
          void onCreate(e)
        }}
        style={{ display: 'flex', gap: 12, alignItems: 'flex-end', flexWrap: 'wrap' }}
      >
        <FormField
          label={t('admin.workspaces.name')}
          value={newName}
          onChange={setNewName}
          required
        />
        <FormField
          label={t('admin.workspaces.description')}
          value={newDesc}
          onChange={setNewDesc}
        />
        <button type="submit" className="admin-btn-primary">
          {t('common.create')}
        </button>
      </form>

      <div style={containerStyle}>
        <DataState
          loading={loading}
          error={error}
          errorPrefix={t('common.error')}
          empty={displayedItems.length === 0}
        >
          <ul style={{ listStyle: 'none', padding: 0, display: 'flex', flexDirection: 'column' }}>
            {displayedItems.length === 0 ? (
              <li style={{ padding: 24, textAlign: 'center', color: '#9ca3af', fontSize: 14 }}>
                {loading ? '' : '—'}
              </li>
            ) : (
              displayedItems.map((w, i) => (
                <li
                  key={w.id}
                  style={{
                    padding: '16px 20px',
                    borderBottom:
                      i === displayedItems.length - 1 && totalPages <= 1
                        ? 'none'
                        : '1px solid var(--border, rgba(255, 255, 255, 0.06))',
                    display: 'flex',
                    justifyContent: 'space-between',
                    alignItems: 'center',
                    flexWrap: 'wrap',
                    gap: 12,
                  }}
                >
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                      <strong style={{ fontSize: 15, color: 'var(--text-primary, #f4f4f5)' }}>
                        {w.name}
                      </strong>
                      <span
                        className="ws-settings__badge"
                        data-type={w.is_personal ? 'personal' : 'team'}
                        style={{ fontSize: 10, padding: '1px 6px' }}
                      >
                        {w.is_personal
                          ? t('admin.workspaces.type_personal')
                          : t('admin.workspaces.type_team')}
                      </span>
                    </div>
                    {w.description && (
                      <div style={{ fontSize: 13, color: 'var(--text-secondary, #a1a1aa)' }}>
                        {w.description}
                      </div>
                    )}
                    <div
                      style={{
                        fontSize: 11,
                        color: 'var(--text-muted, #8a8a92)',
                        fontFamily: 'var(--font-mono, monospace)',
                      }}
                    >
                      {w.slug}
                    </div>
                  </div>
                  <div style={{ display: 'flex', gap: 8 }}>
                    <button
                      onClick={() => {
                        setSelectedWSParam(w.id)
                        setWorkspaceLabelParam(w.name)
                      }}
                      style={btnSettings}
                    >
                      {t('admin.workspaces.settings')}
                    </button>
                    {!w.is_personal && (
                      <button
                        onClick={() => {
                          void onDelete(w.id, w.name)
                        }}
                        style={btnSecondary}
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
        />
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

const btnSecondary: React.CSSProperties = {
  padding: '6px 12px',
  background: 'transparent',
  border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
  color: 'var(--error, crimson)',
  borderRadius: 6,
  cursor: 'pointer',
  fontSize: 13,
}

const btnSettings: React.CSSProperties = {
  padding: '6px 12px',
  background: 'transparent',
  border: '1px solid var(--accent, #4f46e5)',
  color: 'var(--accent, #4f46e5)',
  borderRadius: 6,
  cursor: 'pointer',
  fontSize: 13,
  fontWeight: 500,
}
