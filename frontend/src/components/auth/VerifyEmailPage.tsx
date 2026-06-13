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
        <div className="flex flex-col items-center text-center mb-6">
          <div className="flex items-center justify-center w-12 h-12 rounded-xl bg-gradient-to-br from-[#6366f1] to-[#8b5cf6] mb-4 text-white shadow-[0_4px_12px_rgba(99,102,241,0.3)]">
            <img src={abiLogo} alt="" className="w-[34px] h-[34px] object-contain" />
          </div>
          <h1 className="text-[24px] font-bold text-foreground mb-1 tracking-tight">
            {t('auth.title_verify')}
          </h1>
        </div>

        {status === 'verifying' && (
          <div className="flex flex-col items-center gap-4 p-4">
            <div className="w-8 h-8 border-2 border-white/30 rounded-full border-t-accent animate-spin"></div>
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
              className="flex items-center justify-center gap-2 w-full py-[11px] px-[16px] rounded-lg border-none bg-gradient-to-br from-accent to-[var(--accent-strong)] text-white text-[14px] font-semibold cursor-pointer transition-all duration-150 shadow-[0_4px_10px_rgba(99,102,241,0.2)] hover:opacity-95 hover:-translate-y-[1px] active:translate-y-0 disabled:opacity-60 disabled:cursor-not-allowed disabled:transform-none"
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
