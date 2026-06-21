import { useMemo, useState } from 'react'

import { createShare, listUsers, listWorkspaces } from '../../api/admin'
import { useAsyncState } from '../../hooks/useAsyncState'
import { useFetch } from '../../hooks/useFetch'
import { useT } from '../../i18n'
import { legacyCardClass } from '../../lib/cardClasses'
import { cn } from '../../lib/cn'
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
const EMPTY_USERS: AuthUser[] = []
const EMPTY_WORKSPACES: Workspace[] = []

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

  const {
    data: lookupsData,
    loading: lookupsLoading,
    error: lookupError,
  } = useFetch(
    async () => {
      const [userRes, wsRes] = await Promise.all([
        listUsers(accessToken ?? '', { pageSize: LOOKUP_PAGE_SIZE }),
        listWorkspaces(accessToken ?? '', 1, LOOKUP_PAGE_SIZE),
      ])
      return { users: userRes.users, workspaces: wsRes.workspaces }
    },
    [accessToken],
    { enabled: open && Boolean(accessToken) },
  )

  const users = lookupsData?.users ?? EMPTY_USERS
  const workspaces = lookupsData?.workspaces ?? EMPTY_WORKSPACES

  const {
    loading,
    error: mutationError,
    setError: setMutationError,
    run: runSubmit,
  } = useAsyncState()
  const error = lookupError ?? mutationError

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
    await runSubmit(async () => {
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
    })
  }

  function closeModal() {
    setOpen(false)
    setTargetID('')
    setMutationError(null)
  }

  return (
    <>
      {showTrigger && (
        <button
          onClick={() => setOpen(true)}
          className={`border-border text-foreground-muted hover:border-accent hover:text-accent inline-flex cursor-pointer items-center gap-1.25 rounded-md border bg-transparent px-3 py-1.25 text-xs transition-all duration-150`}
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
          className="motion-safe:animate-fade-in fixed inset-0 z-1000 flex items-center justify-center bg-black/40"
          onClick={closeModal}
        >
          <div
            className={legacyCardClass(
              'bg-card border-border motion-safe:animate-slide-up w-full max-w-105 rounded-xl border shadow-(--shadow,0_20px_60px_rgba(0,0,0,0.2))',
            )}
            onClick={(e) => e.stopPropagation()}
          >
            <div className={`border-border flex items-center justify-between border-b px-5 py-4`}>
              <h3 className="m-0 text-base font-semibold">{t('admin.sharing.share_resource')}</h3>
              <button
                onClick={closeModal}
                className="text-foreground-muted hover:text-foreground cursor-pointer border-0 bg-transparent p-1 text-xl leading-none"
                aria-label={t('common.close')}
              >
                ×
              </button>
            </div>

            <form
              onSubmit={(e) => {
                void onSubmit(e)
              }}
              className="flex flex-col gap-3.5 p-5"
            >
              <div className={`border-border flex gap-0 overflow-hidden rounded-md border`}>
                <button
                  type="button"
                  className={cn(
                    'flex-1 cursor-pointer border-0 bg-transparent px-4 py-2 text-xs transition-all duration-150',
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
                  className={cn(
                    'flex-1 cursor-pointer border-0 bg-transparent px-4 py-2 text-xs transition-all duration-150',
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

              <div className="text-foreground-muted flex flex-col gap-1 text-xs">
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

              <label className="text-foreground-muted flex flex-col gap-1 text-xs">
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
                    'bg-error/10 text-error rounded-sm px-3 py-2 text-xs',
                  )}
                >
                  {error}
                </div>
              )}

              <div className="mt-1 flex justify-end gap-2">
                <button
                  type="button"
                  onClick={closeModal}
                  className={`border-border text-foreground hover:border-accent hover:text-accent text-caption cursor-pointer rounded-md border bg-transparent px-4 py-2 transition-colors`}
                >
                  {t('common.cancel')}
                </button>
                <button
                  type="submit"
                  disabled={loading || !targetID.trim()}
                  className="bg-accent text-caption cursor-pointer rounded-md border-0 px-4 py-2 font-medium text-white transition-all hover:brightness-110 disabled:cursor-not-allowed disabled:opacity-60"
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
