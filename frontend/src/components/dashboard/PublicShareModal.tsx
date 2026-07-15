import { useCallback, useEffect, useState } from 'react'

import { ApiError } from '../../api/apiClient'
import {
  createDashboardPublicShare,
  type CreatedPublicShare,
  getDashboardPublicShare,
  type PublicShareStatus,
  revokeDashboardPublicShare,
} from '../../api/dashboardShare'
import { localeLanguageTag, useLocale, useT } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
import { errorMessage } from '../../utils/error'
import { formatDateOnly } from '../../utils/formatters'
import { Modal } from '../ui/Modal'

interface Props {
  dashboardId: string
  open: boolean
  onClose: () => void
}

const iframeSnippet = (url: string) =>
  `<iframe src="${url}" width="100%" height="600" frameborder="0" title="biqly dashboard"></iframe>`

type CopyTarget = 'link' | 'iframe'

export function PublicShareModal({ dashboardId, open, onClose }: Props) {
  const t = useT()
  const [locale] = useLocale()
  const [loading, setLoading] = useState(false)
  const [busy, setBusy] = useState(false)
  const [status, setStatus] = useState<PublicShareStatus | null>(null)
  // Holds the plaintext token/URL only right after a create/rotate call —
  // the backend never returns it again, so this resets to null whenever the
  // modal is reopened and on revoke.
  const [created, setCreated] = useState<CreatedPublicShare | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [disabledByAdmin, setDisabledByAdmin] = useState(false)
  const [copied, setCopied] = useState<CopyTarget | null>(null)

  // Resets the one-time token/error state left over from a previous open,
  // then reloads the current status — every field is reset here (instead of
  // directly in the effect body below) so the effect only ever performs the
  // React-approved "call an async function" side effect.
  const load = useCallback(async () => {
    setLoading(true)
    setError(null)
    setDisabledByAdmin(false)
    setCreated(null)
    setCopied(null)
    try {
      const s = await getDashboardPublicShare(dashboardId)
      setStatus(s)
    } catch (e) {
      setError(errorMessage(e))
    } finally {
      setLoading(false)
    }
  }, [dashboardId])

  useEffect(() => {
    if (!open) {
      return
    }
    // eslint-disable-next-line react-hooks/set-state-in-effect -- async data fetch on open.
    void load()
  }, [open, load])

  async function handleCreateOrRotate() {
    setBusy(true)
    setError(null)
    setDisabledByAdmin(false)
    try {
      const result = await createDashboardPublicShare(dashboardId)
      setCreated(result)
      setStatus({ active: true, created_at: result.created_at })
    } catch (e) {
      if (e instanceof ApiError && e.status === 409) {
        setDisabledByAdmin(true)
      } else {
        setError(errorMessage(e))
      }
    } finally {
      setBusy(false)
    }
  }

  async function handleRevoke() {
    setBusy(true)
    setError(null)
    try {
      await revokeDashboardPublicShare(dashboardId)
      setCreated(null)
      setStatus({ active: false })
    } catch (e) {
      setError(errorMessage(e))
    } finally {
      setBusy(false)
    }
  }

  function copy(text: string, target: CopyTarget) {
    void navigator.clipboard.writeText(text)
    setCopied(target)
    setTimeout(() => setCopied(null), 2000)
  }

  const url = created ? `${window.location.origin}${created.url_path}` : null

  return (
    <Modal
      open={open}
      onClose={onClose}
      title={t('publicShare.title')}
      subtitle={t('publicShare.description')}
    >
      {loading ? (
        <p className="text-foreground-muted text-caption">{t('common.loading')}</p>
      ) : disabledByAdmin ? (
        <p className="text-error text-caption">{t('publicShare.disabled_by_admin')}</p>
      ) : (
        <>
          {error && <p className="text-error text-caption">{error}</p>}

          {url && created ? (
            <div className="flex flex-col gap-3">
              <p className="text-foreground-muted text-caption">{t('publicShare.token_notice')}</p>
              <div className="flex items-center gap-2">
                <input
                  readOnly
                  value={url}
                  className="flex-1 font-mono text-xs"
                  aria-label={t('publicShare.copy_link')}
                />
                <button
                  type="button"
                  className={buttonClass('secondary', { size: 'sm', autoWidth: true })}
                  onClick={() => copy(url, 'link')}
                >
                  {copied === 'link' ? t('publicShare.copied') : t('publicShare.copy_link')}
                </button>
              </div>
              <div className="flex items-start gap-2">
                <code className="border-border bg-card-raised flex-1 rounded-md border p-2 text-xs break-all">
                  {iframeSnippet(url)}
                </code>
                <button
                  type="button"
                  className={buttonClass('secondary', { size: 'sm', autoWidth: true })}
                  onClick={() => copy(iframeSnippet(url), 'iframe')}
                >
                  {copied === 'iframe' ? t('publicShare.copied') : t('publicShare.copy_iframe')}
                </button>
              </div>
            </div>
          ) : status?.active ? (
            <div className="flex flex-col gap-1">
              <p className="text-foreground text-caption font-semibold">
                {t('publicShare.status_active_title')}
              </p>
              {status.created_at && (
                <p className="text-foreground-muted text-caption">
                  {t('publicShare.active_since', {
                    date: formatDateOnly(status.created_at, localeLanguageTag(locale)),
                  })}
                </p>
              )}
              <p className="text-foreground-muted text-caption">
                {t('publicShare.status_active_no_token')}
              </p>
            </div>
          ) : null}

          <div className="flex justify-end gap-2">
            {status?.active ? (
              <>
                <button
                  type="button"
                  className={buttonClass('danger-outline')}
                  onClick={() => {
                    void handleRevoke()
                  }}
                  disabled={busy}
                >
                  {t('publicShare.revoke')}
                </button>
                <button
                  type="button"
                  className={buttonClass('secondary')}
                  onClick={() => {
                    void handleCreateOrRotate()
                  }}
                  disabled={busy}
                >
                  {t('publicShare.rotate')}
                </button>
              </>
            ) : (
              <button
                type="button"
                className={buttonClass('primary')}
                onClick={() => {
                  void handleCreateOrRotate()
                }}
                disabled={busy}
              >
                {t('publicShare.enable')}
              </button>
            )}
          </div>
        </>
      )}
    </Modal>
  )
}
