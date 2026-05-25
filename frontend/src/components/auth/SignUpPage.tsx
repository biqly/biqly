import { useState, type FormEvent } from 'react'
import { useT } from '../../i18n'
import { useAuth } from './AuthProvider'
import { globalNavigate } from './AuthGuard'

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

  const isLengthValid = password.length >= 8
  const isUppercaseValid = /[A-Z]/.test(password)
  const isDigitValid = /[0-9]/.test(password)
  const isSpecialValid = /[^A-Za-z0-9]/.test(password)

  const rulesMet = [isLengthValid, isUppercaseValid, isDigitValid, isSpecialValid].filter(Boolean).length

  let strengthClass = ''
  let strengthLabel = ''
  let strengthLevel = 0

  if (password.length > 0) {
    if (rulesMet <= 2) {
      strengthClass = 'strength-bar--weak'
      strengthLabel = t('auth.strength_weak')
      strengthLevel = 1
    } else if (rulesMet === 3) {
      strengthClass = 'strength-bar--medium'
      strengthLabel = t('auth.strength_medium')
      strengthLevel = 2
    } else if (rulesMet === 4) {
      strengthClass = 'strength-bar--strong'
      strengthLabel = t('auth.strength_strong')
      strengthLevel = 3
    }
  }

  const handleSubmit = async (e: FormEvent) => {
    e.preventDefault()
    if (!displayName || !email || !password || !confirmPassword) return

    if (password !== confirmPassword) {
      setError('Passwords do not match')
      return
    }

    if (rulesMet < 4) {
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
            📊
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
          {error && <div className="auth-error">{error}</div>}

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

            {password.length > 0 && (
              <div className="password-strength">
                <div className="strength-label-row">
                  <span>{t('auth.strength_label')}</span>
                  <span>{strengthLabel}</span>
                </div>
                <div className="strength-bars">
                  <div className={`strength-bar ${strengthLevel >= 1 ? strengthClass : ''}`} />
                  <div className={`strength-bar ${strengthLevel >= 2 ? strengthClass : ''}`} />
                  <div className={`strength-bar ${strengthLevel >= 3 ? strengthClass : ''}`} />
                </div>
                <ul className="password-rules">
                  <li className={`rule-item ${isLengthValid ? 'valid' : ''}`}>
                    <span className="rule-bullet" />
                    {t('auth.rule_length')}
                  </li>
                  <li className={`rule-item ${isUppercaseValid ? 'valid' : ''}`}>
                    <span className="rule-bullet" />
                    {t('auth.rule_uppercase')}
                  </li>
                  <li className={`rule-item ${isDigitValid ? 'valid' : ''}`}>
                    <span className="rule-bullet" />
                    {t('auth.rule_digit')}
                  </li>
                  <li className={`rule-item ${isSpecialValid ? 'valid' : ''}`}>
                    <span className="rule-bullet" />
                    {t('auth.rule_special')}
                  </li>
                </ul>
              </div>
            )}
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
