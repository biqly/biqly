import { type SubmitEvent, useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import { apiResetPassword } from '../../api/auth'
import abiLogo from '../../assets/abi-logo.png'
import { useT } from '../../i18n'
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
        navigate('/auth/signin')
      }, 3000)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : t('auth.reset_failed'))
    } finally {
      setLoading(false)
    }
  }

  return (
    <div className="auth-page">
      <div className="auth-card">
        <div className="auth-header">
          <div className="auth-logo">
            <img src={abiLogo} alt="" width={34} height={34} />
          </div>
          <h1 className="auth-title">{t('auth.title_reset')}</h1>
          <p className="auth-subtitle">
            <a
              href="/auth/signin"
              onClick={(e) => {
                e.preventDefault()
                navigate('/auth/signin')
              }}
            >
              {t('auth.back_to_login')}
            </a>
          </p>
        </div>

        {success ? (
          <div className="auth-success" style={{ marginBottom: '16px' }}>
            {t('auth.reset_success')}
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="auth-form">
            {error && (
              <div className="auth-error" role="alert" aria-live="assertive">
                {error}
              </div>
            )}

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
                disabled={loading || !token}
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
                disabled={loading || !token}
                autoComplete="new-password"
              />
            </div>

            <button
              type="submit"
              className="auth-btn"
              disabled={loading || !token || !password || !confirmPassword}
            >
              {loading && <div className="spinner" />}
              {t('auth.btn_reset')}
            </button>
          </form>
        )}
      </div>
    </div>
  )
}
