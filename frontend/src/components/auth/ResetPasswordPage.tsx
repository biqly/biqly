import { useState, useEffect, type FormEvent } from 'react'
import abiLogo from '../../assets/abi-logo.png'
import { apiResetPassword } from '../../api/auth'
import { useT } from '../../i18n'
import { globalNavigate } from './AuthGuard'

export default function ResetPasswordPage() {
  const t = useT()
  const [token, setToken] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [success, setSuccess] = useState(false)
  const [loading, setLoading] = useState(false)

  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    const tok = params.get('token')
    if (tok) {
      setToken(tok)
    } else {
      setError('Invalid or missing reset token')
    }
  }, [])

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
    if (!token || !password || !confirmPassword) return

    if (password !== confirmPassword) {
      setError('Passwords do not match')
      return
    }

    if (rulesMet < 4) {
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
        globalNavigate('/auth/signin')
      }, 3000)
    } catch (err: any) {
      setError(err.message || t('auth.reset_failed'))
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
            <a href="/auth/signin" onClick={(e) => { e.preventDefault(); globalNavigate('/auth/signin'); }}>
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
            {error && <div className="auth-error">{error}</div>}

            <div className="form-group">
              <label className="form-label" htmlFor="password-input">{t('auth.password')}</label>
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
