import { type SubmitEvent, useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import { apiResetPassword } from '../../api/auth'
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
import PasswordStrengthMeter from './PasswordStrengthMeter'
export default function ResetPasswordPage() {
  const navigate = useNavigate()
  const t = useT()
  const [token, setToken] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState(false)
  const [loading, setLoading] = useState(false)
  const [passwordValid, setPasswordValid] = useState(false)

  const handleValidity = useCallback((info: { valid: boolean }) => {
    setPasswordValid(info.valid)
  }, [])

  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    const tok = params.get('token')
    if (tok) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setToken(tok)
    } else {
      setError('Invalid or missing reset token')
    }
  }, [])

  const handleSubmit = async (e: SubmitEvent<HTMLFormElement>) => {
    e.preventDefault()
    if (!token || !password || !confirmPassword) {
      return
    }

    if (password !== confirmPassword) {
      setError('Passwords do not match')
      return
    }

    if (!passwordValid) {
      setError('Password does not meet all security requirements')
      return
    }

    setLoading(true)
    setError(null)
    setSuccess(false)

    try {
      await apiResetPassword(token, password)
      setSuccess(true)
      setTimeout(() => {
        void navigate('/auth/signin')
      }, 3000)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : t('auth.reset_failed'))
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
          <h1 className="text-foreground mb-1 text-[24px] font-bold tracking-tight">
            {t('auth.title_reset')}
          </h1>
          <p className="text-foreground-muted text-[14px]">
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
              'border-success text-success mb-4 rounded border-l-[3px] bg-emerald-500/8 p-[10px_12px] text-center text-[13px]',
            )}
          >
            {t('auth.reset_success')}
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
                  'bg-error/8 border-error text-error mb-2 rounded border-l-[3px] p-[10px_12px] text-[13px]',
                )}
                role="alert"
                aria-live="assertive"
              >
                {error}
              </div>
            )}

            <div className={authFieldClass}>
              <label className={authLabelClass} htmlFor="password-input">
                {t('auth.password')}
              </label>
              <input
                id="password-input"
                type="password"
                className={authInputClass}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                disabled={loading || !token}
                autoComplete="new-password"
              />

              <PasswordStrengthMeter password={password} onValidityChange={handleValidity} />
            </div>

            <div className={authFieldClass}>
              <label className={authLabelClass} htmlFor="confirm-password-input">
                {t('auth.confirm_password')}
              </label>
              <input
                id="confirm-password-input"
                type="password"
                className={authInputClass}
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                required
                disabled={loading || !token}
                autoComplete="new-password"
              />
            </div>

            <button
              type="submit"
              className={authSubmitBtnClass}
              disabled={loading || !token || !password || !confirmPassword}
            >
              {loading && (
                <div className="h-4 w-4 animate-spin rounded-full border-2 border-white/30 border-t-white" />
              )}
              {t('auth.btn_reset')}
            </button>
          </form>
        )}
      </div>
    </div>
  )
}
