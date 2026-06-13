import { useState } from 'react'

import { requestDatasourceAccess } from '../../api/admin'
import { useT } from '../../i18n'
import {
  lockedStateAlertClass,
  lockedStateBtnClass,
  lockedStateCardClass,
  lockedStateDescClass,
  lockedStateIconClass,
  lockedStateOverlayClass,
  lockedStateSuccessAlertClass,
  lockedStateTitleClass,
} from '../../lib/lockedStateClasses'
import { useAuth } from '../auth/AuthProvider'
import { ErrorAlert } from './ErrorAlert'
interface LockedStateProps {
  datasourceId: string
  datasourceName?: string
}

export function LockedState({ datasourceId, datasourceName }: LockedStateProps) {
  const t = useT()
  const { accessToken } = useAuth()
  const [requesting, setRequesting] = useState(false)
  const [success, setSuccess] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleRequest = async () => {
    if (!accessToken || requesting) {
      return
    }
    setRequesting(true)
    setError(null)
    setSuccess(false)

    try {
      const res = await requestDatasourceAccess(accessToken, datasourceId)
      if (res.success) {
        setSuccess(true)
      } else {
        setError(t('datasources.request_failed', { error: t('common.unknown_error') }))
      }
    } catch (err: unknown) {
      setError(
        t('datasources.request_failed', {
          error: err instanceof Error ? err.message : String(err),
        }),
      )
    } finally {
      setRequesting(false)
    }
  }

  return (
    <div className={lockedStateOverlayClass}>
      <div className={lockedStateCardClass}>
        <div className={lockedStateIconClass}>
          <svg
            xmlns="http://www.w3.org/2000/svg"
            viewBox="0 0 24 24"
            fill="none"
            stroke="currentColor"
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
            className="feather feather-lock"
            aria-hidden="true"
          >
            <rect x="3" y="11" width="18" height="11" rx="2" ry="2" />
            <path d="M7 11V7a5 5 0 0 1 10 0v4" />
          </svg>
        </div>
        <h2 className={lockedStateTitleClass}>{t('datasources.locked_title')}</h2>
        <p className={lockedStateDescClass}>
          {datasourceName ? `"${datasourceName}" — ` : ''}
          {t('datasources.locked_desc')}
        </p>

        {success && (
          <div className={lockedStateSuccessAlertClass}>{t('datasources.request_success')}</div>
        )}

        <ErrorAlert error={error} className={lockedStateAlertClass} />

        {!success && (
          <button
            type="button"
            className={lockedStateBtnClass}
            onClick={() => {
              void handleRequest()
            }}
            disabled={requesting}
          >
            {requesting ? t('common.loading') : t('datasources.btn_request_access')}
          </button>
        )}
      </div>
    </div>
  )
}
