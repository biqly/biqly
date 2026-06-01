import { useEffect, useState } from 'react'
import { createWorkspace, deleteWorkspace, listWorkspaces } from '../../api/admin'
import { useT } from '../../i18n'
import type { Workspace } from '../../types/auth'
import { WorkspaceSettingsPage } from '../workspaces/WorkspaceSettingsPage'
import { Pagination } from '../ui/Pagination'
import { LoadingOverlay } from '../ui/LoadingOverlay'
import { useQueryParam } from '../../hooks/useQueryParam'
import { useConfirm } from '../../hooks/useConfirm'

export function WorkspacesPanel({ token }: { token: string }) {
  const t = useT()
  const confirm = useConfirm()
  const [items, setItems] = useState<Workspace[]>([])
  const [totalItems, setTotalItems] = useState(0)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [newName, setNewName] = useState('')
  const [newDesc, setNewDesc] = useState('')
  const [selectedWSParam, setSelectedWSParam] = useQueryParam('workspaceId')
  const [, setWorkspaceLabelParam] = useQueryParam('workspaceLabel')
  const selectedWS = selectedWSParam || null

  // Pagination
  const [currentPage, setCurrentPage] = useState(1)
  const pageSize = 10
  const totalPages = Math.ceil(totalItems / pageSize)
  const displayedItems = items

  async function reload() {
    setLoading(true)
    try {
      const res = await listWorkspaces(token, currentPage, pageSize)
      setItems(res.workspaces || [])
      setTotalItems(res.total || 0)
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => {
    reload()
  }, [token, currentPage])

  async function onCreate(e: React.SubmitEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!newName.trim()) return
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
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  async function onDelete(id: string, name: string) {
    const ok = await confirm({
      title: t('admin.workspaces.confirm_delete', { name }),
      variant: 'danger',
    })
    if (!ok) return
    try {
      await deleteWorkspace(token, id)
      reload()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  if (selectedWS) {
    return (
      <WorkspaceSettingsPage
        token={token}
        workspaceID={selectedWS}
        onBack={() => {
          setSelectedWSParam('')
          setWorkspaceLabelParam('')
          reload()
        }}
      />
    )
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      <h2 style={{ marginTop: 0, fontSize: 18 }}>{t('admin.workspaces.title')}</h2>

      <form onSubmit={onCreate} style={{ display: 'flex', gap: 12, alignItems: 'flex-end', flexWrap: 'wrap' }}>
        <label style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
          <span style={{ fontSize: 12, color: 'var(--text-secondary, #a1a1aa)' }}>{t('admin.workspaces.name')}</span>
          <input value={newName} onChange={(e) => setNewName(e.target.value)} style={inputStyle} required />
        </label>
        <label style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
          <span style={{ fontSize: 12, color: 'var(--text-secondary, #a1a1aa)' }}>{t('admin.workspaces.description')}</span>
          <input value={newDesc} onChange={(e) => setNewDesc(e.target.value)} style={inputStyle} />
        </label>
        <button type="submit" style={btnPrimary}>
          {t('common.create')}
        </button>
      </form>

      {error && <div style={errStyle}>{t('common.error')}: {error}</div>}

      <div style={containerStyle}>
        <LoadingOverlay loading={loading}>
          <div style={{ minHeight: displayedItems.length === 0 && loading ? 120 : 'auto', display: 'flex', flexDirection: 'column' }}>
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
                      borderBottom: i === displayedItems.length - 1 && totalPages <= 1 ? 'none' : '1px solid var(--border, rgba(255, 255, 255, 0.06))',
                      display: 'flex',
                      justifyContent: 'space-between',
                      alignItems: 'center',
                      flexWrap: 'wrap',
                      gap: 12,
                    }}
                  >
                    <div style={{ display: 'flex', flexDirection: 'column', gap: 4 }}>
                      <div style={{ display: 'flex', alignItems: 'center', gap: 8 }}>
                        <strong style={{ fontSize: 15, color: 'var(--text-primary, #f4f4f5)' }}>{w.name}</strong>
                        <span
                          className="ws-settings__badge"
                          data-type={w.is_personal ? 'personal' : 'team'}
                          style={{ fontSize: 10, padding: '1px 6px' }}
                        >
                          {w.is_personal ? t('admin.workspaces.type_personal') : t('admin.workspaces.type_team')}
                        </span>
                      </div>
                      {w.description && <div style={{ fontSize: 13, color: 'var(--text-secondary, #a1a1aa)' }}>{w.description}</div>}
                      <div style={{ fontSize: 11, color: 'var(--text-muted, #8a8a92)', fontFamily: 'var(--font-mono, monospace)' }}>{w.slug}</div>
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
                        <button onClick={() => onDelete(w.id, w.name)} style={btnSecondary}>{t('common.delete')}</button>
                      )}
                    </div>
                  </li>
                ))
              )}
            </ul>
          </div>
        </LoadingOverlay>
        <Pagination
          currentPage={currentPage}
          totalPages={totalPages}
          onPageChange={setCurrentPage}
          totalItems={totalItems}
          itemsPerPage={pageSize}
        />
      </div>
    </div>
  )
}

const inputStyle: React.CSSProperties = {
  padding: '8px 12px',
  border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
  borderRadius: 6,
  fontSize: 14,
  minWidth: 240,
  background: 'var(--input-bg, #fff)',
  color: 'var(--text-primary, #111)',
}

const btnPrimary: React.CSSProperties = {
  padding: '8px 16px',
  background: 'var(--accent, #4f46e5)',
  color: 'white',
  border: 0,
  borderRadius: 6,
  cursor: 'pointer',
  fontSize: 13,
  fontWeight: 500,
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

const textMuted: React.CSSProperties = {
  color: 'var(--text-secondary, #8a8a92)',
  fontSize: 14,
  padding: 16,
}

const errStyle: React.CSSProperties = {
  color: 'var(--error, crimson)',
  padding: 16,
  fontWeight: 600,
}

