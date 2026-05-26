import { useCallback, useState, type FormEvent } from 'react'
import abiLogo from '../../assets/abi-logo.png'
import { useT } from '../../i18n'
import { useAuth } from './AuthProvider'
import { globalNavigate } from './AuthGuard'
import PasswordStrengthMeter from './PasswordStrengthMeter'

export default function SignUpPage() {
  const t = useT()
  const { register } = useAuth()

  const [displayName, setDisplayName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [agree, setAgree] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [passwordValid, setPasswordValid] = useState(false)

  const handleValidity = useCallback((info: { valid: boolean }) => {
    setPasswordValid(info.valid)
  }, [])

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (!displayName || !email || !password || !confirmPassword) return

    if (password !== confirmPassword) {
      setError('Passwords do not match')
      return
    }

    if (!passwordValid) {
      setError('Password does not meet all security requirements')
      return
    }

    if (!agree) {
      setError('You must agree to the Terms of Use')
      return
    }

    setLoading(true)
    setError(null)

    try {
      await register(email, password, displayName)
      globalNavigate('/datasources')
    } catch (err: any) {
      setError(err.message || 'Registration failed')
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
          <h1 className="auth-title">{t('auth.title_signup')}</h1>
          <p className="auth-subtitle">
            {t('auth.already_account')}{' '}
            <a href="/auth/signin" onClick={(e) => { e.preventDefault(); globalNavigate('/auth/signin'); }}>
              {t('auth.btn_signin')}
            </a>
          </p>
        </div>

        <form onSubmit={handleSubmit} className="auth-form">
          {error && <div className="auth-error" role="alert" aria-live="assertive">{error}</div>}

          <div className="form-group">
            <label className="form-label" htmlFor="name-input">{t('auth.display_name')}</label>
            <input
              id="name-input"
              type="text"
              className="form-input"
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              required
              disabled={loading}
              autoComplete="name"
            />
          </div>

          <div className="form-group">
            <label className="form-label" htmlFor="email-input">{t('auth.email')}</label>
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

          <div className="form-group">
            <label className="form-label" htmlFor="password-input">{t('auth.password')}</label>
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
            <label className="form-label" htmlFor="confirm-password-input">{t('auth.confirm_password')}</label>
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

          <div className="form-row">
            <label className="form-checkbox-label">
              <input
                type="checkbox"
                checked={agree}
                onChange={(e) => setAgree(e.target.checked)}
                disabled={loading}
              />
              I agree to the Terms of Use & Privacy Policy
            </label>
          </div>

          <button
            type="submit"
            className="auth-btn"
            disabled={loading || !displayName || !email || !password || !confirmPassword || !agree}
          >
            {loading && <div className="spinner" />}
            {t('auth.btn_signup')}
          </button>
        </form>
      </div>
    </div>
  )
}
