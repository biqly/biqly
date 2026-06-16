import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import { apiVerifyEmail } from '../../api/auth'
import abiLogo from '../../assets/abi-logo.png'
import { useT } from '../../i18n'
import {
  authCardClass,
  authIconBoxClass,
  authPageClass,
  authSubmitBtnClass,
} from '../../lib/authClasses'
import { legacyFeedbackClass } from '../../lib/feedbackClasses'
export default function VerifyEmailPage() {
  const navigate = useNavigate()
  const t = useT()
  const [status, setStatus] = useState<'verifying' | 'success' | 'error'>('verifying')
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const verify = async () => {
      const params = new URLSearchParams(window.location.search)
      const token = params.get('token')

      if (!token) {
        setStatus('error')
        setError(t('auth.verify_failed'))
        return
      }

      try {
        await apiVerifyEmail(token)
        setStatus('success')
        setTimeout(() => {
          void navigate('/auth/signin')
        }, 3000)
      } catch (err: unknown) {
        setStatus('error')
        setError(err instanceof Error ? err.message : t('auth.verify_failed'))
      }
    }

    void verify()
  }, [navigate, t])

  return (
    <div className={authPageClass}>
      <div className={authCardClass}>
        <div className="mb-6 flex flex-col items-center text-center">
          <div className={authIconBoxClass}>
            <img src={abiLogo} alt="" className="h-8.5 w-8.5 object-contain" />
          </div>
          <h1 className="text-foreground mb-1 text-2xl font-bold tracking-tight">
            {t('auth.title_verify')}
          </h1>
        </div>

        {status === 'verifying' && (
          <div className="flex flex-col items-center gap-4 p-4">
            <div className="border-t-accent h-8 w-8 animate-spin rounded-full border-2 border-white/30"></div>
            <span className="text-foreground-muted text-sm">Verifying your email address…</span>
          </div>
        )}

        {status === 'success' && (
          <div
            className={legacyFeedbackClass(
              'border-success text-success text-caption mb-4 rounded border-l-[3px] bg-emerald-500/8 p-[10px_12px] text-center',
            )}
          >
            {t('auth.verify_success')}
          </div>
        )}

        {status === 'error' && (
          <div className="flex flex-col gap-4">
            <div
              className={legacyFeedbackClass(
                'bg-error/8 border-error text-error text-caption mb-2 rounded border-l-[3px] p-[10px_12px]',
              )}
            >
              {error}
            </div>
            <button
              type="button"
              className={authSubmitBtnClass}
              onClick={() => {
                void navigate('/auth/signin')
              }}
            >
              {t('auth.back_to_login')}
            </button>
          </div>
        )}
      </div>
    </div>
  )
}
