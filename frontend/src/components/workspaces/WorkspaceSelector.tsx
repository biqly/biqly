import { useEffect, useState } from 'react'

import { listWorkspaces } from '../../api/admin'
import { useAsyncState } from '../../hooks/useAsyncState'
import { useFetch } from '../../hooks/useFetch'
import { useT } from '../../i18n'
import type { Workspace } from '../../types/auth'
import { useAuth } from '../auth/AuthProvider'
import { Select } from '../ui/Select'
import { resolveActiveWorkspace } from './workspaceSelection'

const storageKey = 'biqly_active_workspace_id'
const EMPTY_WORKSPACES: Workspace[] = []

export function WorkspaceSelector({ token }: { token: string }) {
  const t = useT()
  const { user, setActiveWorkspace } = useAuth()
  const [activeID, setActiveID] = useState<string | null>(
    () => user?.active_workspace_id ?? localStorage.getItem(storageKey),
  )

  // scope=member: only workspaces the user can actually switch to (switching
  // requires membership server-side, even for super admins).
  const { data: workspacesRes, loading } = useFetch(
    () => listWorkspaces(token, 1, 100, 'member'),
    [token],
  )
  const workspaces = workspacesRes?.workspaces ?? EMPTY_WORKSPACES

  const { loading: switching, run: runSwitch } = useAsyncState()

  useEffect(() => {
    if (user?.active_workspace_id) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setActiveID(user.active_workspace_id)
      localStorage.setItem(storageKey, user.active_workspace_id)
    }
  }, [user?.active_workspace_id])

  useEffect(() => {
    if (workspaces.length > 0) {
      const active = resolveActiveWorkspace(workspaces, activeID)
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setActiveID(active?.id ?? null)
      if (active) {
        localStorage.setItem(storageKey, active.id)
      }
    }
  }, [workspaces, activeID])

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
    await runSwitch(async () => {
      await setActiveWorkspace(nextID)
      setActiveID(nextID)
      localStorage.setItem(storageKey, nextID)
    })
  }

  return (
    <label className="mb-3 flex min-w-0 flex-col gap-1">
      <span className="text-foreground-faint text-[0.68rem] font-extrabold tracking-normal uppercase">
        {t('admin.workspaces.selector_label')}
      </span>
      <Select
        className="[&_.ui-select-trigger--stacked]:min-h-[2.9rem] [&_.ui-select-trigger--stacked]:items-center [&_.ui-select-trigger--stacked]:py-[0.45rem]"
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
