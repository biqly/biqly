import { useCallback, useEffect, useMemo, useState } from 'react'

import { getRolePermissions, listPermissions, listRoles, setRolePermissions } from '../../api/admin'
import { useAsyncState } from '../../hooks/useAsyncState'
import { useFetch } from '../../hooks/useFetch'
import { useRowSelection } from '../../hooks/useRowSelection'
import { useT } from '../../i18n'
import { cn } from '../../lib/cn'
import type { Permission, Role } from '../../types/auth'
import { sameIdSet, selectionStateFor } from '../../utils/selection'
import { useAuth } from '../auth/AuthProvider'
import { LoadingOverlay } from '../ui/LoadingOverlay'
import {
  adminBtnPrimaryClass,
  adminBtnSecondaryClass,
  adminRoleListItemClass,
} from './adminClasses'
import { AdminPanelShell } from './AdminPanelShell'

const ALL_PAGE_SIZE = 500
const EMPTY_ROLES: Role[] = []
const EMPTY_PERMISSIONS: Permission[] = []

export function RolesPanel({ token }: { token: string }) {
  const t = useT()
  const { hasPermission } = useAuth()
  // Editing role→permission mappings requires admin:roles (server-enforced).
  const canEdit = hasPermission('admin:roles')
  const [selectedRoleId, setSelectedRoleId] = useState<string | null>(null)
  const assigned = useRowSelection()
  const assignedIds = assigned.selected
  const { replace: replaceAssigned } = assigned
  const [savedIds, setSavedIds] = useState<Set<string>>(new Set())

  const fetchMeta = useCallback(
    () =>
      Promise.all([listRoles(token, 1, ALL_PAGE_SIZE), listPermissions(token, 1, ALL_PAGE_SIZE)]),
    [token],
  )

  const { data: metaData, loading: loadingMeta, error: metaError } = useFetch(fetchMeta, [token])
  const roles = metaData?.[0]?.roles ?? EMPTY_ROLES
  const allPerms = metaData?.[1]?.permissions ?? EMPTY_PERMISSIONS

  const {
    data: rolePermsData,
    loading: loadingRolePerms,
    error: rolePermsError,
  } = useFetch(
    () => (selectedRoleId ? getRolePermissions(token, selectedRoleId) : Promise.resolve([])),
    [selectedRoleId, token],
    { enabled: Boolean(selectedRoleId) },
  )

  const {
    loading: saving,
    error: saveError,
    setError: setSaveError,
    run: runSave,
  } = useAsyncState({ useSaving: true })
  const error = metaError ?? rolePermsError ?? saveError

  const selectedRole = roles.find((r) => r.id === selectedRoleId) ?? null
  const dirty = useMemo(() => !sameIdSet(assignedIds, savedIds), [assignedIds, savedIds])

  useEffect(() => {
    if (roles.length > 0) {
      const firstRole = roles[0]
      if (firstRole) {
        // eslint-disable-next-line react-hooks/set-state-in-effect
        setSelectedRoleId((prev) =>
          prev && roles.some((r) => r.id === prev) ? prev : firstRole.id,
        )
      }
    }
  }, [roles])

  useEffect(() => {
    if (rolePermsData) {
      replaceAssigned(rolePermsData)
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setSavedIds(new Set(rolePermsData))
    }
  }, [rolePermsData, replaceAssigned])

  const permsByResource = useMemo(() => {
    const map = new Map<string, Permission[]>()
    for (const p of allPerms) {
      const list = map.get(p.resource) ?? []
      list.push(p)
      map.set(p.resource, list)
    }
    const resources = [...map.keys()].sort()
    return resources.map((resource) => ({
      resource,
      permissions: (map.get(resource) ?? []).sort((a, b) => a.name.localeCompare(b.name)),
    }))
  }, [allPerms])

  function toggleResourceGroup(perms: Permission[], checked: boolean) {
    assigned.setMany(
      perms.map((p) => p.id),
      checked,
    )
  }

  async function onSave() {
    if (!selectedRoleId || !dirty) {
      return
    }
    setSaveError(null)
    await runSave(async () => {
      await setRolePermissions(token, selectedRoleId, [...assignedIds])
      setSavedIds(new Set(assignedIds))
    })
  }

  function onDiscard() {
    assigned.replace(savedIds)
  }

  return (
    <AdminPanelShell
      title={t('admin.roles.title', { count: roles.length })}
      readOnly={!canEdit}
      error={error}
      maxWidth="100%"
    >
      <div className="grid w-full grid-cols-1 items-start gap-6 md:grid-cols-[minmax(220px,280px)_1fr]">
        {/* ROLES SIDEBAR */}
        <section className="flex min-w-0 flex-col gap-3">
          <h3 className="text-foreground m-0 text-base font-bold">
            {t('admin.roles.title', { count: roles.length })}
          </h3>
          <div className="border-border bg-card overflow-hidden rounded-lg border shadow-sm">
            <LoadingOverlay loading={loadingMeta}>
              <ul className="m-0 list-none p-0">
                {roles.length === 0 ? (
                  <li className="text-foreground-muted p-4 text-center text-xs">—</li>
                ) : (
                  roles.map((r) => {
                    const active = r.id === selectedRoleId
                    return (
                      <li key={r.id}>
                        <button
                          type="button"
                          onClick={() => setSelectedRoleId(r.id)}
                          className={adminRoleListItemClass(active)}
                        >
                          <strong className="text-sm font-semibold">{r.name}</strong>
                          {r.description && (
                            <div className="text-foreground-muted mt-1 text-xs">
                              {r.description}
                            </div>
                          )}
                        </button>
                      </li>
                    )
                  })
                )}
              </ul>
            </LoadingOverlay>
          </div>
        </section>

        {/* PERMISSIONS PANEL */}
        <section className="flex min-w-0 flex-col gap-3">
          <div className="flex flex-wrap items-center justify-between gap-3">
            <h3 className="text-foreground m-0 text-base font-bold">
              {selectedRole
                ? t('admin.roles.permissions_for', { role: selectedRole.name })
                : t('admin.roles.permissions_title', { count: allPerms.length })}
            </h3>
            {selectedRole && dirty && (
              <div className="flex gap-2">
                <button
                  type="button"
                  className={adminBtnSecondaryClass}
                  onClick={onDiscard}
                  disabled={saving}
                >
                  {t('common.cancel')}
                </button>
                <button
                  type="button"
                  className={adminBtnPrimaryClass}
                  onClick={() => void onSave()}
                  disabled={saving}
                >
                  {saving ? t('common.saving') : t('admin.roles.save_permissions')}
                </button>
              </div>
            )}
          </div>

          {!selectedRole ? (
            <p className="text-foreground-muted m-0 text-sm">{t('admin.roles.select_role_hint')}</p>
          ) : (
            <div className="border-border bg-card overflow-hidden rounded-lg border shadow-sm">
              <LoadingOverlay loading={loadingMeta || loadingRolePerms}>
                <div className="max-h-[min(70vh,640px)] overflow-y-auto p-3 md:p-4">
                  {permsByResource.map(({ resource, permissions }) => {
                    const groupState = selectionStateFor(
                      assignedIds,
                      permissions.map((p) => p.id),
                    )
                    const allChecked = groupState === 'all'
                    return (
                      <div key={resource} className="mb-5">
                        <label className="flex cursor-pointer items-center gap-2 font-semibold">
                          <input
                            type="checkbox"
                            checked={allChecked}
                            ref={(el) => {
                              if (el) {
                                el.indeterminate = groupState === 'some'
                              }
                            }}
                            onChange={(e) => toggleResourceGroup(permissions, e.target.checked)}
                            disabled={!canEdit}
                          />
                          {resourceBadge(resource)}
                          <span className="text-foreground-muted ml-2 text-xs">
                            {permissions.filter((p) => assignedIds.has(p.id)).length}/
                            {permissions.length}
                          </span>
                        </label>
                        <ul className="m-0 list-none px-0 pt-2 pb-0 pl-7">
                          {permissions.map((p) => (
                            <li key={p.id} className="mb-2">
                              <label className="flex cursor-pointer items-center gap-2 text-sm">
                                <input
                                  type="checkbox"
                                  checked={assignedIds.has(p.id)}
                                  onChange={() => assigned.toggle(p.id)}
                                  disabled={!canEdit}
                                />
                                <span className="font-mono text-xs">{p.name}</span>
                                <span className="ml-2">{actionBadge(p.action)}</span>
                              </label>
                            </li>
                          ))}
                        </ul>
                      </div>
                    )
                  })}
                </div>
              </LoadingOverlay>
            </div>
          )}
        </section>
      </div>
    </AdminPanelShell>
  )
}

function resourceBadge(res: string) {
  let colorClasses = 'bg-foreground/10 text-foreground-muted'
  switch (res) {
    case 'admin':
      colorClasses = 'bg-red-500/12 text-error'
      break
    case 'ai':
      colorClasses = 'bg-emerald-500/12 text-success'
      break
    case 'datasource':
      colorClasses = 'bg-accent/15 text-accent'
      break
    case 'model':
      colorClasses = 'bg-amber-500/14 text-warning'
      break
  }
  return (
    <span
      className={cn(
        'text-2xs inline-block rounded-full px-2 py-0.5 font-semibold tracking-[0.4px] uppercase',
        colorClasses,
      )}
    >
      {res}
    </span>
  )
}

function actionBadge(act: string) {
  return (
    <span className="border-border bg-card-raised text-2xs text-foreground inline-block rounded border px-1.5 py-0.5 font-mono">
      {act}
    </span>
  )
}
