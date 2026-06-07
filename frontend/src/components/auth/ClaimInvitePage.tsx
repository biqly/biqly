import { type SubmitEvent, useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import { apiClaimInvitation, apiGetInvitation } from '../../api/auth'
import abiLogo from '../../assets/abi-logo.png'
import { useT } from '../../i18n'
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

    verifyToken()
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
    <div className="auth-page">
      <div className="auth-card">
        <div className="auth-header">
          <div className="auth-logo">
            <img src={abiLogo} alt="ABI" width={34} height={34} />
          </div>
          <h1 className="auth-title">{t('auth.title_invite')}</h1>
          {email && !success && (
            <p
              className="auth-subtitle"
              style={{ fontSize: '14px', color: 'var(--text-secondary)' }}
            >
              {t('auth.invite_setup_desc', { role: roleName })}
            </p>
          )}
        </div>

        {verifying ? (
          <div
            style={{
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              gap: '16px',
              padding: '16px',
            }}
          >
            <div
              className="spinner"
              style={{ width: '32px', height: '32px', borderTopColor: '#6366f1' }}
            ></div>
            <span style={{ fontSize: '14px', color: 'var(--text-secondary)' }}>
              Validating your invitation…
            </span>
          </div>
        ) : success ? (
          <div className="auth-success" style={{ marginBottom: '16px' }}>
            {t('auth.invite_setup_success')}
          </div>
        ) : error && !email ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
            <div className="auth-error" role="alert" aria-live="assertive">
              {error}
            </div>
            <button
              type="button"
              className="auth-btn"
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
            className="auth-form"
          >
            {error && (
              <div className="auth-error" role="alert" aria-live="assertive">
                {error}
              </div>
            )}

            <div className="form-group">
              <label className="form-label">{t('auth.email')}</label>
              <input
                type="text"
                className="form-input"
                value={email}
                disabled
                style={{ backgroundColor: 'var(--bg-secondary)', cursor: 'not-allowed' }}
              />
            </div>

            <div className="form-group">
              <label className="form-label" htmlFor="name-input">
                {t('auth.display_name')}
              </label>
              <input
                id="name-input"
                type="text"
                className="form-input"
                value={displayName}
                onChange={(e) => setDisplayName(e.target.value)}
                placeholder={email}
                disabled={loading}
                autoComplete="name"
              />
            </div>

            <div className="form-group">
              <label className="form-label" htmlFor="password-input">
                {t('auth.password')}
              </label>
              <input
                id="password-input"
                type="password"
                className="form-input"
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
                disabled={loading}
                autoComplete="new-password"
              />
              <PasswordStrengthMeter password={password} onValidityChange={handleValidity} />
            </div>

            <div className="form-group">
              <label className="form-label" htmlFor="confirm-password-input">
                {t('auth.confirm_password')}
              </label>
              <input
                id="confirm-password-input"
                type="password"
                className="form-input"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                required
                disabled={loading}
                autoComplete="new-password"
              />
            </div>

            <button
              type="submit"
              className="auth-btn"
              disabled={loading || !password || !confirmPassword}
            >
              {loading && <div className="spinner" />}
              {t('auth.btn_setup_account')}
            </button>
          </form>
        )}
      </div>
    </div>
  )
}
