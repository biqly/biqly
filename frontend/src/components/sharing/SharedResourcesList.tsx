import { useEffect, useState, useCallback } from 'react'
import { listShares, deleteShare } from '../../api/admin'
import { useAuth } from '../auth/AuthProvider'
import { useT } from '../../i18n'
import type { ResourceShare } from '../../types/auth'

interface Props {
  resourceType?: string
  refreshKey?: number
}

export function SharedResourcesList({ resourceType, refreshKey }: Props) {
  const t = useT()
  const { accessToken } = useAuth()
  const [items, setItems] = useState<ResourceShare[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  const load = useCallback(async () => {
    if (!accessToken) return
    setLoading(true)
    try {
      setItems(await listShares(accessToken, resourceType))
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setLoading(false)
    }
  }, [accessToken, resourceType, refreshKey])

  useEffect(() => {
    load()
  }, [load])

  async function onRevoke(id: string) {
    if (!accessToken || !confirm(t('admin.sharing.confirm_revoke'))) return
    try {
      await deleteShare(accessToken, id)
      load()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  function permissionBadge(perm: string) {
    switch (perm) {
      case 'view': return t('admin.sharing.permission_view')
      case 'execute': return t('admin.sharing.permission_execute')
      case 'edit': return t('admin.sharing.permission_edit')
      default: return perm
    }
  }

  if (loading) return <div className="shared-list__loading">{t('common.loading')}</div>
  if (error) return <div className="shared-list__error">{error}</div>
  if (items.length === 0) return <p className="shared-list__empty">{t('admin.sharing.empty')}</p>

  return (
    <div className="shared-list">
      <table className="shared-list__table">
        <thead>
          <tr>
            <th>{t('admin.sharing.resource_type')}</th>
            <th>{t('admin.sharing.resource_id')}</th>
            <th>{t('admin.sharing.shared_with')}</th>
            <th>{t('admin.sharing.permission')}</th>
            <th>{t('admin.sharing.created_at')}</th>
            <th>{t('common.actions')}</th>
          </tr>
        </thead>
        <tbody>
          {items.map((share) => (
            <tr key={share.id}>
              <td><span className="shared-list__type-badge">{share.resource_type}</span></td>
              <td className="shared-list__mono">{share.resource_id.slice(0, 8)}…</td>
              <td>
                {share.shared_with
                  ? <span className="shared-list__mono">{share.shared_with.slice(0, 8)}…</span>
                  : share.workspace_id
                    ? <span className="shared-list__workspace">WS: {share.workspace_id.slice(0, 8)}…</span>
                    : '—'}
              </td>
              <td>
                <span className={`shared-list__perm shared-list__perm--${share.permission}`}>
                  {permissionBadge(share.permission)}
                </span>
              </td>
              <td>{new Date(share.created_at).toLocaleDateString()}</td>
              <td>
                <button onClick={() => onRevoke(share.id)} className="shared-list__revoke">
                  {t('common.delete')}
                </button>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
