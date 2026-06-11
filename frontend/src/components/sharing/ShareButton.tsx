import '../../styles/sharing.css'

import { useEffect, useMemo, useState } from 'react'

import { createShare, listUsers, listWorkspaces } from '../../api/admin'
import { useT } from '../../i18n'
import type { AuthUser, Workspace } from '../../types/auth'
import { shareUserSelectOptions, workspaceSelectOptions } from '../admin/adminSelectOptions'
import { useAuth } from '../auth/AuthProvider'
import { Select } from '../ui/Select'

interface Props {
  resourceType: string
  resourceID: string
  onShared?: () => void
  /** Controlled open state; when provided together with onOpenChange the
   * built-in trigger can be hidden and the modal driven externally. */
  open?: boolean
  onOpenChange?: (open: boolean) => void
  showTrigger?: boolean
}

const LOOKUP_PAGE_SIZE = 500

export function ShareButton({
  resourceType,
  resourceID,
  onShared,
  open: openProp,
  onOpenChange,
  showTrigger = true,
}: Props) {
  const t = useT()
  const { accessToken } = useAuth()
  const [internalOpen, setInternalOpen] = useState(false)
  const open = openProp ?? internalOpen
  const setOpen = (v: boolean) => {
    setInternalOpen(v)
    onOpenChange?.(v)
  }
  const [mode, setMode] = useState<'user' | 'workspace'>('user')
  const [targetID, setTargetID] = useState('')
  const [permission, setPermission] = useState<'view' | 'execute' | 'edit'>('view')
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [users, setUsers] = useState<AuthUser[]>([])
  const [workspaces, setWorkspaces] = useState<Workspace[]>([])
  const [lookupsLoading, setLookupsLoading] = useState(false)

  useEffect(() => {
    if (!open || !accessToken) {
      return
    }
    let cancelled = false
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setLookupsLoading(true)
    Promise.all([
      listUsers(accessToken, { pageSize: LOOKUP_PAGE_SIZE }),
      listWorkspaces(accessToken, 1, LOOKUP_PAGE_SIZE),
    ])
      .then(([userRes, wsRes]) => {
        if (cancelled) {
          return
        }
        setUsers(userRes.users)
        setWorkspaces(wsRes.workspaces)
      })
      .catch((e) => {
        if (!cancelled) {
          setError(e instanceof Error ? e.message : String(e))
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLookupsLoading(false)
        }
      })
    return () => {
      cancelled = true
    }
  }, [open, accessToken])

  const targetOptions = useMemo(() => {
    if (mode === 'user') {
      return shareUserSelectOptions(users, lookupsLoading)
    }
    return workspaceSelectOptions(workspaces, lookupsLoading)
  }, [mode, users, workspaces, lookupsLoading])

  async function onSubmit(e: React.SubmitEvent<HTMLFormElement>) {
    e.preventDefault()
    if (!accessToken || !targetID.trim()) {
      return
    }
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

  function closeModal() {
    setOpen(false)
    setTargetID('')
    setError(null)
  }

  return (
    <>
      {showTrigger && (
        <button onClick={() => setOpen(true)} className="share-btn">
          <svg
            width="14"
            height="14"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
          >
            <path d="M4 12v8a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-8" />
            <polyline points="16 6 12 2 8 6" />
            <line x1="12" y1="2" x2="12" y2="15" />
          </svg>
          {t('admin.sharing.share')}
        </button>
      )}

      {open && (
        <div className="share-modal__overlay" onClick={closeModal}>
          <div className="share-modal" onClick={(e) => e.stopPropagation()}>
            <div className="share-modal__header">
              <h3>{t('admin.sharing.share_resource')}</h3>
              <button
                onClick={closeModal}
                className="share-modal__close"
                aria-label={t('common.close')}
              >
                ×
              </button>
            </div>

            <form
              onSubmit={(e) => {
                void onSubmit(e)
              }}
              className="share-modal__form"
            >
              <div className="share-modal__mode-tabs">
                <button
                  type="button"
                  className={`share-modal__tab ${mode === 'user' ? 'share-modal__tab--active' : ''}`}
                  onClick={() => {
                    setMode('user')
                    setTargetID('')
                  }}
                >
                  {t('admin.sharing.share_with_user')}
                </button>
                <button
                  type="button"
                  className={`share-modal__tab ${mode === 'workspace' ? 'share-modal__tab--active' : ''}`}
                  onClick={() => {
                    setMode('workspace')
                    setTargetID('')
                  }}
                >
                  {t('admin.sharing.share_with_workspace')}
                </button>
              </div>

              <div className="share-modal__field">
                <span>
                  {mode === 'user' ? t('admin.sharing.user_id') : t('admin.sharing.workspace')}
                </span>
                <Select
                  searchable
                  value={targetID}
                  options={targetOptions}
                  onChange={setTargetID}
                  placeholder={
                    mode === 'user'
                      ? t('admin.sharing.user_id_placeholder')
                      : t('admin.sharing.workspace_id_placeholder')
                  }
                  disabled={lookupsLoading}
                />
              </div>

              <label className="share-modal__field">
                <span>{t('admin.sharing.permission')}</span>
                <Select
                  value={permission}
                  options={[
                    { value: 'view', label: t('admin.sharing.permission_view') },
                    { value: 'execute', label: t('admin.sharing.permission_execute') },
                    { value: 'edit', label: t('admin.sharing.permission_edit') },
                  ]}
                  onChange={(v) => setPermission(v)}
                />
              </label>

              {error && <div className="share-modal__error">{error}</div>}

              <div className="share-modal__actions">
                <button type="button" onClick={closeModal} className="share-modal__btn-cancel">
                  {t('common.cancel')}
                </button>
                <button
                  type="submit"
                  disabled={loading || !targetID.trim()}
                  className="share-modal__btn-submit"
                >
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
