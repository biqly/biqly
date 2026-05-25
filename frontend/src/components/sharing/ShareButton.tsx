import { useState } from 'react'
import { createShare } from '../../api/admin'
import { useAuth } from '../auth/AuthProvider'
import { useT } from '../../i18n'

interface Props {
  resourceType: string
  resourceID: string
  onShared?: () => void
}

export function ShareButton({ resourceType, resourceID, onShared }: Props) {
  const t = useT()
  const { accessToken } = useAuth()
  const [open, setOpen] = useState(false)
  const [mode, setMode] = useState<'user' | 'workspace'>('user')
  const [targetID, setTargetID] = useState('')
  const [permission, setPermission] = useState<'view' | 'execute' | 'edit'>('view')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function onSubmit(e: React.FormEvent) {
    e.preventDefault()
    if (!accessToken || !targetID.trim()) return
    setLoading(true)
    setError(null)
    try {
      await createShare(
        accessToken,
        resourceType,
        resourceID,
        permission,
        mode === 'user' ? targetID.trim() : undefined,
        mode === 'workspace' ? targetID.trim() : undefined,
      )
      setOpen(false)
      setTargetID('')
      onShared?.()
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e))
    } finally {
      setLoading(false)
    }
  }

  return (
    <>
      <button onClick={() => setOpen(true)} className="share-btn">
        <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
          <path d="M4 12v8a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-8" />
          <polyline points="16 6 12 2 8 6" />
          <line x1="12" y1="2" x2="12" y2="15" />
        </svg>
        {t('admin.sharing.share')}
      </button>

      {open && (
        <div className="share-modal__overlay" onClick={() => setOpen(false)}>
          <div className="share-modal" onClick={(e) => e.stopPropagation()}>
            <div className="share-modal__header">
              <h3>{t('admin.sharing.share_resource')}</h3>
              <button onClick={() => setOpen(false)} className="share-modal__close" aria-label={t('common.close')}>×</button>
            </div>

            <form onSubmit={onSubmit} className="share-modal__form">
              <div className="share-modal__mode-tabs">
                <button
                  type="button"
                  className={`share-modal__tab ${mode === 'user' ? 'share-modal__tab--active' : ''}`}
                  onClick={() => setMode('user')}
                >
                  {t('admin.sharing.share_with_user')}
                </button>
                <button
                  type="button"
                  className={`share-modal__tab ${mode === 'workspace' ? 'share-modal__tab--active' : ''}`}
                  onClick={() => setMode('workspace')}
                >
                  {t('admin.sharing.share_with_workspace')}
                </button>
              </div>

              <label className="share-modal__field">
                <span>{mode === 'user' ? t('admin.sharing.user_id') : t('admin.sharing.workspace')}</span>
                <input
                  value={targetID}
                  onChange={(e) => setTargetID(e.target.value)}
                  placeholder={mode === 'user' ? t('admin.sharing.user_id_placeholder') : t('admin.sharing.workspace_id_placeholder')}
                  required
                />
              </label>

              <label className="share-modal__field">
                <span>{t('admin.sharing.permission')}</span>
                <select value={permission} onChange={(e) => setPermission(e.target.value as 'view' | 'execute' | 'edit')}>
                  <option value="view">{t('admin.sharing.permission_view')}</option>
                  <option value="execute">{t('admin.sharing.permission_execute')}</option>
                  <option value="edit">{t('admin.sharing.permission_edit')}</option>
                </select>
              </label>

              {error && <div className="share-modal__error">{error}</div>}

              <div className="share-modal__actions">
                <button type="button" onClick={() => setOpen(false)} className="share-modal__btn-cancel">
                  {t('common.cancel')}
                </button>
                <button type="submit" disabled={loading} className="share-modal__btn-submit">
                  {loading ? t('common.saving') : t('admin.sharing.share')}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </>
  )
}
