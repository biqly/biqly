import { type SubmitEvent, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import { apiForgotPassword } from '../../api/auth'
import abiLogo from '../../assets/abi-logo.png'
import { useT } from '../../i18n'
import {
  authCardClass,
  authFieldClass,
  authFormClass,
  authInputClass,
  authLabelClass,
  authPageClass,
  authSubmitBtnClass,
} from '../../lib/authClasses'
import { legacyFeedbackClass } from '../../lib/feedbackClasses'
export default function ForgotPasswordPage() {
  const navigate = useNavigate()
  const t = useT()
  const [email, setEmail] = useState('')
  const [success, setSuccess] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)

  const handleSubmit = async (e: SubmitEvent<HTMLFormElement>) => {
    e.preventDefault()
    if (!email) {
      return
    }

    setLoading(true)
    setError(null)
    setSuccess(false)

    try {
      await apiForgotPassword(email)
      setSuccess(true)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to send reset link')
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className={authPageClass}>
      <div className={authCardClass}>
        <div className="mb-6 flex flex-col items-center text-center">
          <div className="mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-linear-to-br from-[#6366f1] to-[#8b5cf6] text-white shadow-[0_4px_12px_rgba(99,102,241,0.3)]">
            <img src={abiLogo} alt="" className="h-8.5 w-8.5 object-contain" />
          </div>
          <h1 className="mb-1 text-[24px] font-bold tracking-tight text-foreground">
            {t('auth.title_forgot')}
          </h1>
          <p className="text-[14px] text-foreground-muted">
            <a
              href="/auth/signin"
              className="font-medium text-[#6366f1] no-underline hover:underline"
              onClick={(e) => {
                e.preventDefault()
                void navigate('/auth/signin')
              }}
            >
              {t('auth.back_to_login')}
            </a>
          </p>
        </div>

        {success ? (
          <div
            className={legacyFeedbackClass(
              'p-[10px_12px] bg-emerald-500/8 border-l-[3px] border-success text-success text-[13px] rounded text-center mb-4',
            )}
          >
            {t('auth.forgot_success')}
          </div>
        ) : (
          <form
            onSubmit={(e) => {
              void handleSubmit(e)
            }}
            className={authFormClass}
          >
            {error && (
              <div
                className={legacyFeedbackClass(
                  'p-[10px_12px] bg-error/8 border-l-[3px] border-error text-error text-[13px] rounded mb-2',
                )}
              >
                {error}
              </div>
            )}

            <div className={authFieldClass}>
              <label className={authLabelClass} htmlFor="email-input">
                {t('auth.email')}
              </label>
              <input
                id="email-input"
                type="email"
                className={authInputClass}
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                disabled={loading}
                autoComplete="email"
              />
            </div>

            <button type="submit" className={authSubmitBtnClass} disabled={loading || !email}>
              {loading && (
                <div className="h-4 w-4 animate-spin rounded-full border-2 border-white/30 border-t-white" />
              )}
              {t('auth.btn_send_reset')}
            </button>
          </form>
        )}
      </div>
    </div>
  )
}
