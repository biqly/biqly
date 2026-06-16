import { type SubmitEvent, useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import { apiClaimInvitation, apiGetInvitation } from '../../api/auth'
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
import { useAuth } from './AuthProvider'
import PasswordStrengthMeter from './PasswordStrengthMeter'
export default function ClaimInvitePage() {
  const navigate = useNavigate()
  const t = useT()
  const { loginWithTokens } = useAuth()
  const [token, setToken] = useState('')
  const [email, setEmail] = useState('')
  const [roleName, setRoleName] = useState('')
  const [displayName, setDisplayName] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [verifying, setVerifying] = useState(true)
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
    if (!tok) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setError(t('auth.invite_invalid_token'))
      setVerifying(false)
      return
    }

    setToken(tok)

    const verifyToken = async () => {
      try {
        const invite = await apiGetInvitation(tok)
        setEmail(invite.email)
        setRoleName(invite.role_name)
      } catch (err: unknown) {
        setError(err instanceof Error ? err.message : t('auth.invite_invalid_token'))
      } finally {
        setVerifying(false)
      }
    }

    void verifyToken()
  }, [t])

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

    try {
      const resp = await apiClaimInvitation(token, password, displayName)
      setSuccess(true)
      setTimeout(() => {
        void (async () => {
          try {
            await loginWithTokens(resp.access_token, resp.roles)
            void navigate('/')
          } catch {
            void navigate('/auth/signin')
          }
        })()
      }, 2000)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Failed to complete account setup')
      setLoading(false)
    }
  }

  return (
    <div className={authPageClass}>
      <div className={authCardClass}>
        <div className="mb-6 flex flex-col items-center text-center">
          <div className="mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-linear-to-br from-[#6366f1] to-[#8b5cf6] text-white shadow-[0_4px_12px_rgba(99,102,241,0.3)]">
            <img src={abiLogo} alt="ABI" className="h-8.5 w-8.5 object-contain" />
          </div>
          <h1 className="text-foreground mb-1 text-[24px] font-bold tracking-tight">
            {t('auth.title_invite')}
          </h1>
          {email && !success && (
            <p className="text-foreground-muted text-[14px]">
              {t('auth.invite_setup_desc', { role: roleName })}
            </p>
          )}
        </div>

        {verifying ? (
          <div className="flex flex-col items-center gap-4 p-4">
            <div className="border-t-accent h-8 w-8 animate-spin rounded-full border-2 border-white/30"></div>
            <span className="text-foreground-muted text-[14px]">Validating your invitation…</span>
          </div>
        ) : success ? (
          <div
            className={legacyFeedbackClass(
              'border-success text-success mb-4 rounded border-l-[3px] bg-emerald-500/8 p-[10px_12px] text-center text-[13px]',
            )}
          >
            {t('auth.invite_setup_success')}
          </div>
        ) : error && !email ? (
          <div className="flex flex-col gap-4">
            <div
              className={legacyFeedbackClass(
                'bg-error/8 border-error text-error mb-2 rounded border-l-[3px] p-[10px_12px] text-[13px]',
              )}
              role="alert"
              aria-live="assertive"
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
              <label className={authLabelClass}>{t('auth.email')}</label>
              <input type="text" className={authInputClass} value={email} disabled readOnly />
            </div>

            <div className={authFieldClass}>
              <label className={authLabelClass} htmlFor="name-input">
                {t('auth.display_name')}
              </label>
              <input
                id="name-input"
                type="text"
                className={authInputClass}
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
                placeholder={email}
                disabled={loading}
                autoComplete="name"
              />
            </div>

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
                disabled={loading}
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
                disabled={loading}
                autoComplete="new-password"
              />
            </div>

            <button
              type="submit"
              className={authSubmitBtnClass}
              disabled={loading || !password || !confirmPassword}
            >
              {loading && (
                <div className="h-4 w-4 animate-spin rounded-full border-2 border-white/30 border-t-white" />
              )}
              {t('auth.btn_setup_account')}
            </button>
          </form>
        )}
      </div>
    </div>
  )
}
