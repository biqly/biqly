import { useEffect, useState } from 'react'
import { listWorkspaces } from '../../api/admin'
import { useAuth } from '../auth/AuthProvider'
import { useT } from '../../i18n'
import type { Workspace } from '../../types/auth'
import { resolveActiveWorkspace } from './workspaceSelection'
import '../../styles/workspace.css'

const storageKey = 'biqly_active_workspace_id'

export function WorkspaceSelector({ token }: { token: string }) {
  const t = useT()
  const { user, setActiveWorkspace } = useAuth()
  const [workspaces, setWorkspaces] = useState<Workspace[]>([])
  const [activeID, setActiveID] = useState<string | null>(
    () => user?.active_workspace_id ?? localStorage.getItem(storageKey),
  )
  const [loading, setLoading] = useState(true)
  const [switching, setSwitching] = useState(false)

  useEffect(() => {
    if (user?.active_workspace_id) {
      setActiveID(user.active_workspace_id)
      localStorage.setItem(storageKey, user.active_workspace_id)
    }
  }, [user?.active_workspace_id])

  useEffect(() => {
    let cancelled = false
    async function load() {
      try {
        const res = await listWorkspaces(token)
        if (cancelled) return
        setWorkspaces(res.workspaces)
        const active = resolveActiveWorkspace(res.workspaces, activeID)
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

  const handleChange = async (nextID: string) => {
    if (!nextID || nextID === activeID) return
    setSwitching(true)
    try {
      await setActiveWorkspace(nextID)
      setActiveID(nextID)
      localStorage.setItem(storageKey, nextID)
    } catch {
      // revert on failure: server rejected (likely lost membership)
    } finally {
      setSwitching(false)
    }
  }

  return (
    <label className="workspace-selector">
      <span className="workspace-selector__label">{t('admin.workspaces.selector_label')}</span>
      <select
        value={active?.id ?? ''}
        onChange={(e) => void handleChange(e.target.value)}
        aria-label={t('admin.workspaces.selector_label')}
        disabled={switching}
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
