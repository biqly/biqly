import { useEffect, useState } from 'react'
import { listPermissions, listRoles } from '../../api/admin'
import { useT } from '../../i18n'
import type { Permission, Role } from '../../types/auth'

export function RolesPanel({ token }: { token: string }) {
  const t = useT()
  const [roles, setRoles] = useState<Role[]>([])
  const [perms, setPerms] = useState<Permission[]>([])
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    let cancelled = false
    async function load() {
      try {
        const [r, p] = await Promise.all([listRoles(token), listPermissions(token)])
        if (cancelled) return
        setRoles(r)
        setPerms(p)
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
  }, [token])

  if (loading) return <div>{t('common.loading')}</div>
  if (error) return <div style={{ color: 'crimson' }}>{t('common.error')}: {error}</div>

  return (
    <div style={{ display: 'grid', gridTemplateColumns: '1fr 2fr', gap: 24 }}>
      <section>
        <h2 style={{ marginTop: 0 }}>{t('admin.roles.title', { count: roles.length })}</h2>
        <ul style={{ listStyle: 'none', padding: 0, display: 'flex', flexDirection: 'column', gap: 6 }}>
          {roles.map((r) => (
            <li key={r.id} style={{ padding: 8, border: '1px solid var(--border-color, #e5e7eb)', borderRadius: 6 }}>
              <strong>{r.name}</strong>
              {r.description && <div style={{ fontSize: 12, color: '#6b7280' }}>{r.description}</div>}
            </li>
          ))}
        </ul>
      </section>
      <section>
        <h2 style={{ marginTop: 0 }}>{t('admin.roles.permissions_title', { count: perms.length })}</h2>
        <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: 14 }}>
          <thead>
            <tr style={{ textAlign: 'left' }}>
              <th style={{ borderBottom: '1px solid #e5e7eb', padding: 6 }}>Resource</th>
              <th style={{ borderBottom: '1px solid #e5e7eb', padding: 6 }}>Action</th>
              <th style={{ borderBottom: '1px solid #e5e7eb', padding: 6 }}>{t('admin.roles.name')}</th>
            </tr>
          </thead>
          <tbody>
            {perms.map((p) => (
              <tr key={p.id}>
                <td style={{ borderBottom: '1px solid #f3f4f6', padding: 6 }}>{p.resource}</td>
                <td style={{ borderBottom: '1px solid #f3f4f6', padding: 6 }}>{p.action}</td>
                <td style={{ borderBottom: '1px solid #f3f4f6', padding: 6, fontFamily: 'monospace' }}>{p.name}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </section>
    </div>
  )
}
