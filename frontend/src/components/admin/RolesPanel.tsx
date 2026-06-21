import { useCallback, useEffect, useMemo, useState } from 'react'

import { getRolePermissions, listPermissions, listRoles, setRolePermissions } from '../../api/admin'
import { useAsyncState } from '../../hooks/useAsyncState'
import { useFetch } from '../../hooks/useFetch'
import { useRowSelection } from '../../hooks/useRowSelection'
import { useT } from '../../i18n'
import type { Permission, Role } from '../../types/auth'
import { sameIdSet, selectionStateFor } from '../../utils/selection'
import { useAuth } from '../auth/AuthProvider'
import { ErrorAlert } from '../ui/ErrorAlert'
import { LoadingOverlay } from '../ui/LoadingOverlay'
import {
  adminBtnPrimaryClass,
  adminBtnSecondaryClass,
  adminRoleListItemClass,
} from './adminClasses'
import { ReadOnlyNote } from './ReadOnlyNote'

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
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      {error && <ErrorAlert error={`${t('common.error')}: ${error}`} />}
      {!canEdit && <ReadOnlyNote />}

      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'minmax(220px, 280px) 1fr',
          gap: 24,
          alignItems: 'start',
        }}
      >
        <section style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <h2 style={{ marginTop: 0, fontSize: 18 }}>
            {t('admin.roles.title', { count: roles.length })}
          </h2>
          <div style={containerStyle}>
            <LoadingOverlay loading={loadingMeta}>
              <ul style={{ listStyle: 'none', padding: 0, margin: 0 }}>
                {roles.length === 0 ? (
                  <li style={{ padding: 16, textAlign: 'center', color: '#9ca3af', fontSize: 13 }}>
                    —
                  </li>
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
                          <strong style={{ fontSize: 14 }}>{r.name}</strong>
                          {r.description && (
                            <div
                              style={{
                                fontSize: 12,
                                color: 'var(--text-secondary, #a1a1aa)',
                                marginTop: 4,
                              }}
                            >
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

        <section style={{ display: 'flex', flexDirection: 'column', gap: 12, minWidth: 0 }}>
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              gap: 12,
              flexWrap: 'wrap',
            }}
          >
            <h2 style={{ marginTop: 0, fontSize: 18, marginBottom: 0 }}>
              {selectedRole
                ? t('admin.roles.permissions_for', { role: selectedRole.name })
                : t('admin.roles.permissions_title', { count: allPerms.length })}
            </h2>
            {selectedRole && dirty && (
              <div style={{ display: 'flex', gap: 8 }}>
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
            <p style={hintStyle}>{t('admin.roles.select_role_hint')}</p>
          ) : (
            <div style={containerStyle}>
              <LoadingOverlay loading={loadingMeta || loadingRolePerms}>
                <div
                  style={{ padding: '12px 16px', maxHeight: 'min(70vh, 640px)', overflowY: 'auto' }}
                >
                  {permsByResource.map(({ resource, permissions }) => {
                    const groupState = selectionStateFor(
                      assignedIds,
                      permissions.map((p) => p.id),
                    )
                    const allChecked = groupState === 'all'
                    return (
                      <div key={resource} style={{ marginBottom: 20 }}>
                        <label style={groupLabelStyle}>
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
                          <span
                            style={{ marginLeft: 8, fontSize: 12, color: 'var(--text-secondary)' }}
                          >
                            {permissions.filter((p) => assignedIds.has(p.id)).length}/
                            {permissions.length}
                          </span>
                        </label>
                        <ul style={{ listStyle: 'none', padding: '8px 0 0 28px', margin: 0 }}>
                          {permissions.map((p) => (
                            <li key={p.id} style={{ marginBottom: 8 }}>
                              <label style={permLabelStyle}>
                                <input
                                  type="checkbox"
                                  checked={assignedIds.has(p.id)}
                                  onChange={() => assigned.toggle(p.id)}
                                  disabled={!canEdit}
                                />
                                <span
                                  style={{
                                    fontFamily: 'var(--font-mono, monospace)',
                                    fontSize: 12,
                                  }}
                                >
                                  {p.name}
                                </span>
                                <span style={{ marginLeft: 8 }}>{actionBadge(p.action)}</span>
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
    </div>
  )
}

function resourceBadge(res: string) {
  let style: React.CSSProperties = {
    padding: '2px 8px',
    borderRadius: '12px',
    fontSize: '11px',
    fontWeight: 600,
    textTransform: 'uppercase',
    letterSpacing: '0.4px',
    display: 'inline-block',
  }
  switch (res) {
    case 'admin':
      style = { ...style, background: 'rgba(239, 68, 68, 0.12)', color: 'var(--error, #ef4444)' }
      break
    case 'ai':
      style = { ...style, background: 'rgba(16, 185, 129, 0.12)', color: 'var(--success, #10b981)' }
      break
    case 'datasource':
      style = {
        ...style,
        background: 'var(--accent-glow, rgba(99, 102, 241, 0.15))',
        color: 'var(--accent)',
      }
      break
    case 'model':
      style = { ...style, background: 'rgba(245, 158, 11, 0.14)', color: 'var(--warning, #f59e0b)' }
      break
    default:
      style = {
        ...style,
        background: 'rgba(107, 114, 128, 0.1)',
        color: 'var(--text-secondary, #a1a1aa)',
      }
  }
  return <span style={style}>{res}</span>
}

function actionBadge(act: string) {
  return (
    <span
      style={{
        padding: '2px 6px',
        background: 'var(--bg-card-raised, rgba(255, 255, 255, 0.08))',
        border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
        color: 'var(--text-primary, #f4f4f5)',
        borderRadius: '4px',
        fontSize: '11px',
        fontFamily: 'var(--font-mono, monospace)',
        display: 'inline-block',
      }}
    >
      {act}
    </span>
  )
}

const containerStyle: React.CSSProperties = {
  background: 'var(--bg-card, #ffffff)',
  border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
  borderRadius: 8,
  overflow: 'hidden',
  boxShadow: 'var(--shadow-sm, 0 1px 3px rgba(0,0,0,0.05))',
}

const groupLabelStyle: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  gap: 8,
  cursor: 'pointer',
  fontWeight: 600,
}

const permLabelStyle: React.CSSProperties = {
  display: 'flex',
  alignItems: 'center',
  gap: 8,
  cursor: 'pointer',
  fontSize: 13,
}

const hintStyle: React.CSSProperties = {
  color: 'var(--text-secondary, #a1a1aa)',
  fontSize: 14,
  margin: 0,
}
