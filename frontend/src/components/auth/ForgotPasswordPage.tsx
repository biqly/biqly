import { type SubmitEvent, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import { apiForgotPassword } from '../../api/auth'
import abiLogo from '../../assets/abi-logo.png'
import { useT } from '../../i18n'

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
    <div className="auth-page">
      <div className="auth-card">
        <div className="auth-header">
          <div className="auth-logo">
            <img src={abiLogo} alt="" width={34} height={34} />
          </div>
          <h1 className="auth-title">{t('auth.title_forgot')}</h1>
          <p className="auth-subtitle">
            <a
              href="/auth/signin"
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
          <div className="auth-success" style={{ marginBottom: '16px' }}>
            {t('auth.forgot_success')}
          </div>
        ) : (
          <form
            onSubmit={(e) => {
              void handleSubmit(e)
            }}
            className="auth-form"
          >
            {error && <div className="auth-error">{error}</div>}

            <div className="form-group">
              <label className="form-label" htmlFor="email-input">
                {t('auth.email')}
              </label>
              <input
                id="email-input"
                type="email"
                className="form-input"
                value={email}
                onChange={(e) => setEmail(e.target.value)}
                required
                disabled={loading}
                autoComplete="email"
              />
            </div>

            <button type="submit" className="auth-btn" disabled={loading || !email}>
              {loading && <div className="spinner" />}
              {t('auth.btn_send_reset')}
            </button>
          </form>
        )}
      </div>
    </div>
  )
}
