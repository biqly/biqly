import { useCallback, useEffect, useMemo, useState } from 'react'
import {
  getRolePermissions,
  listPermissions,
  listRoles,
  setRolePermissions,
} from '../../api/admin'
import { useT } from '../../i18n'
import type { Permission, Role } from '../../types/auth'
import { LoadingOverlay } from '../ui/LoadingOverlay'

const ALL_PAGE_SIZE = 500

export function RolesPanel({ token }: { token: string }) {
  const t = useT()
  const [roles, setRoles] = useState<Role[]>([])
  const [allPerms, setAllPerms] = useState<Permission[]>([])
  const [selectedRoleId, setSelectedRoleId] = useState<string | null>(null)
  const [assignedIds, setAssignedIds] = useState<Set<string>>(new Set())
  const [savedIds, setSavedIds] = useState<Set<string>>(new Set())
  const [error, setError] = useState<string | null>(null)
  const [loadingMeta, setLoadingMeta] = useState(true)
  const [loadingRolePerms, setLoadingRolePerms] = useState(false)
  const [saving, setSaving] = useState(false)

  const selectedRole = roles.find((r) => r.id === selectedRoleId) ?? null
  const dirty = useMemo(() => {
    if (assignedIds.size !== savedIds.size) return true
    for (const id of assignedIds) {
      if (!savedIds.has(id)) return true
    }
    return false
  }, [assignedIds, savedIds])

  useEffect(() => {
    let cancelled = false
    async function loadMeta() {
      try {
        setLoadingMeta(true)
        const [rolesRes, permsRes] = await Promise.all([
          listRoles(token, 1, ALL_PAGE_SIZE),
          listPermissions(token, 1, ALL_PAGE_SIZE),
        ])
        if (cancelled) return
        setRoles(rolesRes.roles)
        setAllPerms(permsRes.permissions)
        const firstRole = rolesRes.roles[0]
        if (firstRole) {
          setSelectedRoleId((prev) =>
            prev && rolesRes.roles.some((r) => r.id === prev) ? prev : firstRole.id,
          )
        }
        setError(null)
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : String(e))
      } finally {
        if (!cancelled) setLoadingMeta(false)
      }
    }
    loadMeta()
    return () => {
      cancelled = true
    }
  }, [token])

  const loadRolePermissions = useCallback(
    async (roleID: string) => {
      setLoadingRolePerms(true)
      try {
        const ids = await getRolePermissions(token, roleID)
        const set = new Set(ids)
        setAssignedIds(set)
        setSavedIds(new Set(set))
        setError(null)
      } catch (e) {
        setError(e instanceof Error ? e.message : String(e))
      } finally {
        setLoadingRolePerms(false)
      }
    },
    [token],
  )

  useEffect(() => {
    if (!selectedRoleId) return
    loadRolePermissions(selectedRoleId)
  }, [selectedRoleId, loadRolePermissions])

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

  function togglePermission(permId: string) {
    setAssignedIds((prev) => {
      const next = new Set(prev)
      if (next.has(permId)) next.delete(permId)
      else next.add(permId)
      return next
    })
  }

  function toggleResourceGroup(perms: Permission[], checked: boolean) {
    setAssignedIds((prev) => {
      const next = new Set(prev)
      for (const p of perms) {
        if (checked) next.add(p.id)
        else next.delete(p.id)
      }
      return next
    })
  }

  async function onSave() {
    if (!selectedRoleId || !dirty) return
    setSaving(true)
    try {
      await setRolePermissions(token, selectedRoleId, [...assignedIds])
      setSavedIds(new Set(assignedIds))
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setSaving(false)
    }
  }

  function onDiscard() {
    setAssignedIds(new Set(savedIds))
  }

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      {error && <div style={errStyle}>{t('common.error')}: {error}</div>}

      <div style={{ display: 'grid', gridTemplateColumns: 'minmax(220px, 280px) 1fr', gap: 24, alignItems: 'start' }}>
        <section style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <h2 style={{ marginTop: 0, fontSize: 18 }}>{t('admin.roles.title', { count: roles.length })}</h2>
          <div style={containerStyle}>
            <LoadingOverlay loading={loadingMeta}>
              <ul style={{ listStyle: 'none', padding: 0, margin: 0 }}>
                {roles.length === 0 ? (
                  <li style={{ padding: 16, textAlign: 'center', color: '#9ca3af', fontSize: 13 }}>—</li>
                ) : (
                  roles.map((r) => {
                    const active = r.id === selectedRoleId
                    return (
                      <li key={r.id}>
                        <button
                          type="button"
                          onClick={() => setSelectedRoleId(r.id)}
                          className={`admin-role-list-item${active ? ' admin-role-list-item--active' : ''}`}
                        >
                          <strong style={{ fontSize: 14 }}>{r.name}</strong>
                          {r.description && (
                            <div style={{ fontSize: 12, color: 'var(--text-secondary, #a1a1aa)', marginTop: 4 }}>
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
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 12, flexWrap: 'wrap' }}>
            <h2 style={{ marginTop: 0, fontSize: 18, marginBottom: 0 }}>
              {selectedRole
                ? t('admin.roles.permissions_for', { role: selectedRole.name })
                : t('admin.roles.permissions_title', { count: allPerms.length })}
            </h2>
            {selectedRole && dirty && (
              <div style={{ display: 'flex', gap: 8 }}>
                <button type="button" className="admin-btn-secondary" onClick={onDiscard} disabled={saving}>
                  {t('common.cancel')}
                </button>
                <button type="button" className="admin-btn-primary" onClick={() => void onSave()} disabled={saving}>
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
                <div style={{ padding: '12px 16px', maxHeight: 'min(70vh, 640px)', overflowY: 'auto' }}>
                  {permsByResource.map(({ resource, permissions }) => {
                    const allChecked = permissions.every((p) => assignedIds.has(p.id))
                    const someChecked = permissions.some((p) => assignedIds.has(p.id))
                    return (
                      <div key={resource} style={{ marginBottom: 20 }}>
                        <label style={groupLabelStyle}>
                          <input
                            type="checkbox"
                            checked={allChecked}
                            ref={(el) => {
                              if (el) el.indeterminate = someChecked && !allChecked
                            }}
                            onChange={(e) => toggleResourceGroup(permissions, e.target.checked)}
                          />
                          {resourceBadge(resource)}
                          <span style={{ marginLeft: 8, fontSize: 12, color: 'var(--text-secondary)' }}>
                            {permissions.filter((p) => assignedIds.has(p.id)).length}/{permissions.length}
                          </span>
                        </label>
                        <ul style={{ listStyle: 'none', padding: '8px 0 0 28px', margin: 0 }}>
                          {permissions.map((p) => (
                            <li key={p.id} style={{ marginBottom: 8 }}>
                              <label style={permLabelStyle}>
                                <input
                                  type="checkbox"
                                  checked={assignedIds.has(p.id)}
                                  onChange={() => togglePermission(p.id)}
                                />
                                <span style={{ fontFamily: 'var(--font-mono, monospace)', fontSize: 12 }}>{p.name}</span>
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
      style = { ...style, background: 'var(--accent-glow, rgba(99, 102, 241, 0.15))', color: 'var(--accent, #6366f1)' }
      break
    case 'model':
      style = { ...style, background: 'rgba(245, 158, 11, 0.14)', color: 'var(--warning, #f59e0b)' }
      break
    default:
      style = { ...style, background: 'rgba(107, 114, 128, 0.1)', color: 'var(--text-secondary, #a1a1aa)' }
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

const errStyle: React.CSSProperties = {
  color: 'var(--error, crimson)',
  padding: 16,
  fontWeight: 600,
}
