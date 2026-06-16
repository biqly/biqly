import clsx from 'clsx'
import { useEffect, useMemo, useState } from 'react'

import { createShare, listUsers, listWorkspaces } from '../../api/admin'
import { useT } from '../../i18n'
import { legacyCardClass } from '../../lib/cardClasses'
import { legacyFeedbackClass } from '../../lib/feedbackClasses'
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
  const [lookupsLoading, setLookupsLoading] = useState(false)
  const [users, setUsers] = useState<AuthUser[]>([])
  const [workspaces, setWorkspaces] = useState<Workspace[]>([])

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

  async function onSubmit(e: React.FormEvent<HTMLFormElement>) {
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
        <button
          onClick={() => setOpen(true)}
          className={`inline-flex items-center gap-[5px] py-[5px] px-3 bg-transparent border border-border rounded-[6px] cursor-pointer text-[12px] text-foreground-muted transition-all duration-150 hover:border-accent hover:text-accent`}
        >
          <svg
            width="14"
            height="14"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            className="shrink-0"
          >
            <path d="M4 12v8a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2v-8" />
            <polyline points="16 6 12 2 8 6" />
            <line x1="12" y1="2" x2="12" y2="15" />
          </svg>
          {t('admin.sharing.share')}
        </button>
      )}

      {open && (
        <div
          className="fixed inset-0 z-1000 flex items-center justify-center bg-black/40 motion-safe:animate-fade-in"
          onClick={closeModal}
        >
          <div
            className={legacyCardClass(
              'bg-card border border-border rounded-[12px] w-full max-w-[420px] shadow-(--shadow,0_20px_60px_rgba(0,0,0,0.2)) motion-safe:animate-slide-up',
            )}
            onClick={(e) => e.stopPropagation()}
          >
            <div className={`flex items-center justify-between py-4 px-5 border-b border-border`}>
              <h3 className="m-0 text-[16px] font-semibold">{t('admin.sharing.share_resource')}</h3>
              <button
                onClick={closeModal}
                className="cursor-pointer border-0 bg-transparent p-1 text-[20px] leading-none text-foreground-muted hover:text-foreground"
                aria-label={t('common.close')}
              >
                ×
              </button>
            </div>

            <form
              onSubmit={(e) => {
                void onSubmit(e)
              }}
              className="flex flex-col gap-[14px] p-5"
            >
              <div className={`flex gap-0 border border-border rounded-[6px] overflow-hidden`}>
                <button
                  type="button"
                  className={clsx(
                    'flex-1 cursor-pointer border-0 bg-transparent px-4 py-2 text-[12px] transition-all duration-150',
                    mode === 'user'
                      ? 'bg-accent text-white'
                      : 'text-foreground-muted hover:text-foreground',
                  )}
                  onClick={() => {
                    setMode('user')
                    setTargetID('')
                  }}
                >
                  {t('admin.sharing.share_with_user')}
                </button>
                <button
                  type="button"
                  className={clsx(
                    'flex-1 cursor-pointer border-0 bg-transparent px-4 py-2 text-[12px] transition-all duration-150',
                    mode === 'workspace'
                      ? 'bg-accent text-white'
                      : 'text-foreground-muted hover:text-foreground',
                  )}
                  onClick={() => {
                    setMode('workspace')
                    setTargetID('')
                  }}
                >
                  {t('admin.sharing.share_with_workspace')}
                </button>
              </div>

              <div className="flex flex-col gap-1 text-[12px] text-foreground-muted">
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

              <label className="flex flex-col gap-1 text-[12px] text-foreground-muted">
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

              {error && (
                <div
                  className={legacyFeedbackClass(
                    'py-2 px-3 bg-error/10 rounded-[4px] text-error text-[12px]',
                  )}
                >
                  {error}
                </div>
              )}

              <div className="mt-1 flex justify-end gap-2">
                <button
                  type="button"
                  onClick={closeModal}
                  className={`py-2 px-4 bg-transparent border border-border rounded-[6px] cursor-pointer text-[13px] text-foreground hover:border-accent hover:text-accent transition-colors`}
                >
                  {t('common.cancel')}
                </button>
                <button
                  type="submit"
                  disabled={loading || !targetID.trim()}
                  className="cursor-pointer rounded-[6px] border-0 bg-accent px-4 py-2 text-[13px] font-medium text-white transition-all hover:brightness-110 disabled:cursor-not-allowed disabled:opacity-60"
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
