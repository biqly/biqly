import { useEffect, useState } from 'react'

import { listWorkspaces } from '../../api/admin'
import { useT } from '../../i18n'
import type { Workspace } from '../../types/auth'
import { useAuth } from '../auth/AuthProvider'
import { Select } from '../ui/Select'
import { resolveActiveWorkspace } from './workspaceSelection'

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
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setActiveID(user.active_workspace_id)
      localStorage.setItem(storageKey, user.active_workspace_id)
    }
  }, [user?.active_workspace_id])

  useEffect(() => {
    let cancelled = false
    async function load() {
      try {
        const res = await listWorkspaces(token)
        if (cancelled) {
          return
        }
        setWorkspaces(res.workspaces)
        const active = resolveActiveWorkspace(res.workspaces, activeID)
        setActiveID(active?.id ?? null)
        if (active) {
          localStorage.setItem(storageKey, active.id)
        }
      } catch {
        if (!cancelled) {
          setWorkspaces([])
        }
      } finally {
        if (!cancelled) {
          setLoading(false)
        }
      }
    }
    void load()
    return () => {
      cancelled = true
    }
  }, [activeID, token])

  if (loading || workspaces.length === 0) {
    return null
  }

  const active = resolveActiveWorkspace(workspaces, activeID)

  const workspaceOptions = workspaces.map((workspace) => ({
    value: workspace.id,
    label: workspace.name,
    hint: workspace.is_personal ? t('admin.workspaces.type_personal') : undefined,
  }))

  const handleChange = async (nextID: string) => {
    if (!nextID || nextID === activeID) {
      return
    }
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
    <label className="flex flex-col gap-[0.35rem] mb-4 min-w-0">
      <span className="text-foreground-faint text-[0.68rem] font-extrabold tracking-normal uppercase">
        {t('admin.workspaces.selector_label')}
      </span>
      <Select
        className="[&_.ui-select-trigger--stacked]:items-start [&_.ui-select-trigger--stacked]:min-h-[2.65rem] [&_.ui-select-trigger--stacked]:pt-[0.42rem] [&_.ui-select-trigger--stacked]:pb-[0.42rem] [&_.ui-select-trigger--stacked_.ui-select-chevron]:mt-[0.2rem]"
        value={active?.id ?? ''}
        options={workspaceOptions}
        onChange={(v) => void handleChange(v)}
        ariaLabel={t('admin.workspaces.selector_label')}
        disabled={switching}
        showHintInTrigger
      />
    </label>
  )
}
