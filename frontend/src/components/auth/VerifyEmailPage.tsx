import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import { apiVerifyEmail } from '../../api/auth'
import abiLogo from '../../assets/abi-logo.png'
import { useT } from '../../i18n'
import { authCardClass, authPageClass } from '../../lib/authClasses'
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
          <div className="mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-linear-to-br from-[#6366f1] to-[#8b5cf6] text-white shadow-[0_4px_12px_rgba(99,102,241,0.3)]">
            <img src={abiLogo} alt="" className="h-8.5 w-8.5 object-contain" />
          </div>
          <h1 className="mb-1 text-[24px] font-bold tracking-tight text-foreground">
            {t('auth.title_verify')}
          </h1>
        </div>

        {status === 'verifying' && (
          <div className="flex flex-col items-center gap-4 p-4">
            <div className="h-8 w-8 animate-spin rounded-full border-2 border-white/30 border-t-accent"></div>
            <span className="text-[14px] text-foreground-muted">Verifying your email address…</span>
          </div>
        )}

        {status === 'success' && (
          <div
            className={legacyFeedbackClass(
              'p-[10px_12px] bg-emerald-500/8 border-l-[3px] border-success text-success text-[13px] rounded text-center mb-4',
            )}
          >
            {t('auth.verify_success')}
          </div>
        )}

        {status === 'error' && (
          <div className="flex flex-col gap-4">
            <div
              className={legacyFeedbackClass(
                'p-[10px_12px] bg-error/8 border-l-[3px] border-error text-error text-[13px] rounded mb-2',
              )}
            >
              {error}
            </div>
            <button
              type="button"
              className="flex w-full cursor-pointer items-center justify-center gap-2 rounded-lg border-none bg-linear-to-br from-accent to-accent-strong px-4 py-2.75 text-[14px] font-semibold text-white shadow-[0_4px_10px_rgba(99,102,241,0.2)] transition-all duration-150 hover:-translate-y-px hover:opacity-95 active:translate-y-0 disabled:transform-none disabled:cursor-not-allowed disabled:opacity-60"
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
