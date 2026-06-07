import '../../styles/sharing.css'

import { useCallback, useEffect, useState } from 'react'

import { deleteShare, listShares } from '../../api/admin'
import { useConfirm } from '../../hooks/useConfirm'
import { useT } from '../../i18n'
import type { ResourceShare } from '../../types/auth'
import { useAuth } from '../auth/AuthProvider'
import { LoadingOverlay } from '../ui/LoadingOverlay'
import { Pagination } from '../ui/Pagination'

interface Props {
  resourceType?: string
  refreshKey?: number
}

export function SharedResourcesList({ resourceType, refreshKey }: Props) {
  const t = useT()
  const confirm = useConfirm()
  const { accessToken } = useAuth()
  const [items, setItems] = useState<ResourceShare[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  // Pagination
  const [currentPage, setCurrentPage] = useState(1)
  const pageSize = 10
  const [totalItems, setTotalItems] = useState(0)
  const totalPages = Math.ceil(totalItems / pageSize)
  const displayedItems = items

  const load = useCallback(async () => {
    if (!accessToken) {
      return
    }
    setLoading(true)
    try {
      const res = await listShares(accessToken, {
        page: currentPage,
        pageSize,
        resourceType,
      })
      setItems(res.shares || [])
      setTotalItems(res.total || 0)
      setError(null)
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setLoading(false)
    }
  }, [accessToken, resourceType, refreshKey, currentPage])

  useEffect(() => {
    load()
  }, [load])

  useEffect(() => {
    setCurrentPage(1)
  }, [resourceType])

  async function onRevoke(id: string) {
    if (!accessToken) {
      return
    }
    const ok = await confirm({
      title: t('admin.sharing.confirm_revoke'),
      variant: 'danger',
    })
    if (!ok) {
      return
    }
    try {
      await deleteShare(accessToken, id)
      load()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    }
  }

  function permissionBadge(perm: string) {
    switch (perm) {
      case 'view':
        return t('admin.sharing.permission_view')
      case 'execute':
        return t('admin.sharing.permission_execute')
      case 'edit':
        return t('admin.sharing.permission_edit')
      default:
        return perm
    }
  }

  return (
    <div className="shared-list">
      {error && <div className="shared-list__error">{error}</div>}

      <div style={containerStyle}>
        <LoadingOverlay loading={loading}>
          <div
            style={{
              minHeight: displayedItems.length === 0 && loading ? 120 : 'auto',
              display: 'flex',
              flexDirection: 'column',
            }}
          >
            {displayedItems.length === 0 ? (
              <p
                className="shared-list__empty"
                style={{ margin: 0, padding: '48px 24px', textAlign: 'center' }}
              >
                {loading ? '' : t('admin.sharing.empty')}
              </p>
            ) : (
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
                  {displayedItems.map((share) => (
                    <tr key={share.id}>
                      <td>
                        <span className="shared-list__type-badge">{share.resource_type}</span>
                      </td>
                      <td className="shared-list__mono">{share.resource_id.slice(0, 8)}…</td>
                      <td>
                        {share.shared_with ? (
                          <span className="shared-list__mono">
                            {share.shared_with.slice(0, 8)}…
                          </span>
                        ) : share.workspace_id ? (
                          <span className="shared-list__workspace">
                            WS: {share.workspace_id.slice(0, 8)}…
                          </span>
                        ) : (
                          '—'
                        )}
                      </td>
                      <td>
                        <span
                          className={`shared-list__perm shared-list__perm--${share.permission}`}
                        >
                          {permissionBadge(share.permission)}
                        </span>
                      </td>
                      <td>{new Date(share.created_at).toLocaleDateString()}</td>
                      <td>
                        <button
                          onClick={() => {
                            void onRevoke(share.id)
                          }}
                          className="shared-list__revoke"
                        >
                          {t('common.delete')}
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </LoadingOverlay>
        {displayedItems.length > 0 && (
          <Pagination
            currentPage={currentPage}
            totalPages={totalPages}
            onPageChange={setCurrentPage}
            totalItems={totalItems}
            itemsPerPage={pageSize}
          />
        )}
      </div>
    </div>
  )
}

const containerStyle: React.CSSProperties = {
  background: 'var(--bg-card, #ffffff)',
  border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
  borderRadius: 8,
  overflow: 'hidden',
  boxShadow: 'var(--shadow-sm, 0 1px 3px rgba(0,0,0,0.05))',
}
