import { useEffect, useState } from 'react'
import { listWorkspaces } from '../../api/admin'
import { useT } from '../../i18n'
import type { Workspace } from '../../types/auth'
import { resolveActiveWorkspace } from './workspaceSelection'

const storageKey = 'biqly_active_workspace_id'

export function WorkspaceSelector({ token }: { token: string }) {
  const t = useT()
  const [workspaces, setWorkspaces] = useState<Workspace[]>([])
  const [activeID, setActiveID] = useState<string | null>(() => localStorage.getItem(storageKey))
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    async function load() {
      try {
        const rows = await listWorkspaces(token)
        if (cancelled) return
        setWorkspaces(rows)
        const active = resolveActiveWorkspace(rows, activeID)
        setActiveID(active?.id ?? null)
        if (active) localStorage.setItem(storageKey, active.id)
      } catch {
        if (!cancelled) setWorkspaces([])
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    load()
    return () => {
      cancelled = true
    }
  }, [token])

  if (loading || workspaces.length === 0) return null

  const active = resolveActiveWorkspace(workspaces, activeID)

  return (
    <label className="workspace-selector">
      <span className="workspace-selector__label">{t('admin.workspaces.selector_label')}</span>
      <select
        value={active?.id ?? ''}
        onChange={(e) => {
          setActiveID(e.target.value)
          localStorage.setItem(storageKey, e.target.value)
        }}
        aria-label={t('admin.workspaces.selector_label')}
      >
        {workspaces.map((workspace) => (
          <option key={workspace.id} value={workspace.id}>
            {workspace.name}{workspace.is_personal ? ` ${t('admin.workspaces.personal_suffix')}` : ''}
          </option>
        ))}
      </select>
    </label>
  )
}
