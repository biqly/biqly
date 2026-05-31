import { useEffect, useState } from 'react'
import { listPermissions, listRoles } from '../../api/admin'
import { useT } from '../../i18n'
import type { Permission, Role } from '../../types/auth'
import { Pagination } from '../ui/Pagination'
import { LoadingOverlay } from '../ui/LoadingOverlay'

export function RolesPanel({ token }: { token: string }) {
  const t = useT()
  const [roles, setRoles] = useState<Role[]>([])
  const [perms, setPerms] = useState<Permission[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  // Roles Pagination
  const [rolesPage, setRolesPage] = useState(1)
  const rolesPageSize = 10
  const [totalRoles, setTotalRoles] = useState(0)

  // Permissions Pagination
  const [permsPage, setPermsPage] = useState(1)
  const permsPageSize = 10
  const [totalPerms, setTotalPerms] = useState(0)

  useEffect(() => {
    let cancelled = false
    async function load() {
      try {
        setLoading(true)
        const [rRes, pRes] = await Promise.all([
          listRoles(token, rolesPage, rolesPageSize),
          listPermissions(token, permsPage, permsPageSize),
        ])
        if (cancelled) return
        setRoles(rRes.roles)
        setTotalRoles(rRes.total)
        setPerms(pRes.permissions)
        setTotalPerms(pRes.total)
      } catch (e) {
        if (!cancelled) setError(e instanceof Error ? e.message : String(e))
      } finally {
        if (!cancelled) setLoading(false)
      }
    }
    load()
    return () => {
      cancelled = true
    }
  }, [token, rolesPage, permsPage])

  const totalRolesPages = Math.ceil(totalRoles / rolesPageSize)
  const totalPermsPages = Math.ceil(totalPerms / permsPageSize)
  const displayedPerms = perms
  const displayedRoles = roles

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 16 }}>
      {error && <div style={errStyle}>{t('common.error')}: {error}</div>}

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 2fr', gap: 24, alignItems: 'start' }}>
        <section style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <h2 style={{ marginTop: 0, fontSize: 18 }}>{t('admin.roles.title', { count: totalRoles })}</h2>
          <div style={containerStyle}>
            <LoadingOverlay loading={loading}>
              <div style={{ minHeight: displayedRoles.length === 0 && loading ? 120 : 'auto', display: 'flex', flexDirection: 'column' }}>
                <ul style={{ listStyle: 'none', padding: 0, display: 'flex', flexDirection: 'column' }}>
                  {displayedRoles.length === 0 ? (
                    <li style={{ padding: 16, textAlign: 'center', color: '#9ca3af', fontSize: 13 }}>
                      {loading ? '' : '—'}
                    </li>
                  ) : (
                    displayedRoles.map((r, i) => (
                      <li
                        key={r.id}
                        style={{
                          padding: '12px 16px',
                          borderBottom: i === displayedRoles.length - 1 && totalRolesPages <= 1 ? 'none' : '1px solid var(--border, rgba(255, 255, 255, 0.06))',
                        }}
                      >
                        <strong style={{ fontSize: 14, color: 'var(--text-primary, #f4f4f5)' }}>{r.name}</strong>
                        {r.description && (
                          <div style={{ fontSize: 12, color: 'var(--text-secondary, #a1a1aa)', marginTop: 4 }}>
                            {r.description}
                          </div>
                        )}
                      </li>
                    ))
                  )}
                </ul>
              </div>
            </LoadingOverlay>
            <Pagination
              currentPage={rolesPage}
              totalPages={totalRolesPages}
              onPageChange={setRolesPage}
              totalItems={totalRoles}
              itemsPerPage={rolesPageSize}
            />
          </div>
        </section>

        <section style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <h2 style={{ marginTop: 0, fontSize: 18 }}>{t('admin.roles.permissions_title', { count: totalPerms })}</h2>
          <div style={containerStyle}>
            <LoadingOverlay loading={loading}>
              <div style={{ minHeight: displayedPerms.length === 0 && loading ? 120 : 'auto', display: 'flex', flexDirection: 'column' }}>
                <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
                  <thead>
                    <tr style={theadRow}>
                      <th style={thStyle}>Resource</th>
                      <th style={thStyle}>Action</th>
                      <th style={thStyle}>{t('admin.roles.name')}</th>
                    </tr>
                  </thead>
                  <tbody>
                    {displayedPerms.length === 0 ? (
                      <tr>
                        <td colSpan={3} style={{ padding: 24, textAlign: 'center', color: '#9ca3af' }}>
                          {loading ? '' : '—'}
                        </td>
                      </tr>
                    ) : (
                      displayedPerms.map((p) => (
                        <tr key={p.id} style={trRow}>
                          <td style={tdStyle}>{resourceBadge(p.resource)}</td>
                          <td style={tdStyle}>{actionBadge(p.action)}</td>
                          <td style={{ ...tdStyle, fontFamily: 'var(--font-mono, monospace)', fontSize: 12 }}>{p.name}</td>
                        </tr>
                      ))
                    )}
                  </tbody>
                </table>
              </div>
            </LoadingOverlay>
            <Pagination
              currentPage={permsPage}
              totalPages={totalPermsPages}
              onPageChange={setPermsPage}
              totalItems={totalPerms}
              itemsPerPage={permsPageSize}
            />
          </div>
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
        fontSize: '12px',
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

const theadRow: React.CSSProperties = {
  background: 'var(--table-header-bg, #f9fafb)',
  borderBottom: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
  textAlign: 'left',
}

const thStyle: React.CSSProperties = {
  padding: '12px 16px',
  fontWeight: 600,
  color: 'var(--table-header-fg, #4b5563)',
}

const trRow: React.CSSProperties = {
  borderBottom: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
}

const tdStyle: React.CSSProperties = {
  padding: '12px 16px',
  color: 'var(--text-primary, #f4f4f5)',
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

