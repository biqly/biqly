import { useEffect, useRef, useState, type SubmitEvent } from 'react'
import abiLogo from '../../assets/abi-logo.png'
import { apiPasskeyLoginBegin, apiPasskeyLoginFinish } from '../../api/auth'
import { useT } from '../../i18n'
import { base64urlToBuffer, bufferToBase64url } from '../../utils/webauthn'
import { useAuth } from './AuthProvider'
import { globalNavigate } from './AuthGuard'

const FAILED_LOGIN_BACKOFFS_MS = [0, 1000, 2000, 4000, 8000]

export default function SignInPage() {
  const t = useT()
  const { login, loginWithTokens } = useAuth()

  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [passkeyLoading, setPasskeyLoading] = useState(false)
  const [throttleMs, setThrottleMs] = useState(0)
  const [sessionBanner, setSessionBanner] = useState<{ title: string; body: string } | null>(null)
  const failureCountRef = useRef(0)

  // Render an explanatory banner when redirected here from a session-expiry
  // event (set by AuthProvider via ?expired=<reason>).
  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    const reason = params.get('expired')
    if (!reason) return

    const titleKey = reason === 'idle' ? 'session_expired_title_idle'
      : reason === 'absolute' ? 'session_expired_title_absolute'
      : reason === 'revoked' ? 'session_expired_title_revoked'
      : 'session_expired_title_generic'

    const bodyKey = reason === 'idle' ? 'session_expired_idle'
      : reason === 'absolute' ? 'session_expired_absolute'
      : reason === 'revoked' ? 'session_expired_revoked'
      : 'session_expired_generic'

    setSessionBanner({
      title: t(`auth.${titleKey}` as const),
      body: t(`auth.${bodyKey}` as const),
    })
    window.history.replaceState(null, '', '/auth/signin')
  }, [t])

  useEffect(() => {
    if (throttleMs <= 0) return
    const interval = window.setInterval(() => {
      setThrottleMs((prev) => Math.max(0, prev - 250))
    }, 250)
    return () => window.clearInterval(interval)
  }, [throttleMs])

  const handleSubmit = async (e: SubmitEvent<HTMLFormElement>) => {
    e.preventDefault()
    if (!email || !password || throttleMs > 0) return

    setLoading(true)
    setError(null)
    try {
      await login(email, password)
      failureCountRef.current = 0
      globalNavigate('/datasources')
    } catch (err: any) {
      setError(err.message || 'Login failed')
      // Exponential client-side backoff. Server already rate-limits, this is
      // pure UX so a frustrated user does not hammer the form and pin the
      // browser tab.
      const idx = Math.min(failureCountRef.current + 1, FAILED_LOGIN_BACKOFFS_MS.length - 1)
      failureCountRef.current = idx
      const wait = FAILED_LOGIN_BACKOFFS_MS[idx] ?? 0
      if (wait > 0) setThrottleMs(wait)
    } finally {
      setLoading(false)
    }
  }

  const handlePasskeyLogin = async () => {
    if (!email) {
      setError('Please enter your email to login with passkey')
      return
    }

    setPasskeyLoading(true)
    setError(null)
    try {
      const beginResp = await apiPasskeyLoginBegin(email)
      const publicKeyOptions = beginResp.publicKey

      const options: CredentialRequestOptions = {
        publicKey: {
          ...publicKeyOptions,
          challenge: base64urlToBuffer(publicKeyOptions.challenge),
          allowCredentials: publicKeyOptions.allowCredentials?.map((cred: any) => ({
            ...cred,
            id: base64urlToBuffer(cred.id),
          })),
        },
      }

      const credential = await navigator.credentials.get(options)
      if (!credential) {
        throw new Error('No credential returned')
      }

      const assertion = credential as PublicKeyCredential
      const response = assertion.response as AuthenticatorAssertionResponse
      const credentialJson = {
        id: assertion.id,
        rawId: bufferToBase64url(assertion.rawId),
        type: assertion.type,
        response: {
          clientDataJSON: bufferToBase64url(response.clientDataJSON),
          authenticatorData: bufferToBase64url(response.authenticatorData),
          signature: bufferToBase64url(response.signature),
          userHandle: response.userHandle ? bufferToBase64url(response.userHandle) : null,
        },
      }

      const finishResp = await apiPasskeyLoginFinish(credentialJson)
      await loginWithTokens(finishResp.access_token, finishResp.refresh_token)
      globalNavigate('/datasources')
    } catch (err: any) {
      setError(err.message || 'Passkey login failed')
    } finally {
      setPasskeyLoading(false)
    }
  }

  const handleOAuth = (provider: string) => {
    window.location.href = `/api/auth/oauth/${provider}`
  }

  return (
    <div className="auth-page">
      <div className="auth-card">
        <div className="auth-header">
          <div className="auth-logo">
            <img src={abiLogo} alt="" width={34} height={34} />
          </div>
          <h1 className="auth-title">{t('auth.title_signin')}</h1>
          <p className="auth-subtitle">
            {t('auth.no_account')}{' '}
            <a href="/auth/signup" onClick={(e) => { e.preventDefault(); globalNavigate('/auth/signup'); }}>
              {t('auth.btn_signup')}
            </a>
          </p>
        </div>

        {sessionBanner && (
          <div className="auth-info" role="status" aria-live="polite">
            <strong>{sessionBanner.title}</strong>
            <div>{sessionBanner.body}</div>
          </div>
        )}

        <form onSubmit={handleSubmit} className="auth-form">
          {error && <div className="auth-error" role="alert" aria-live="assertive">{error}</div>}
          {throttleMs > 0 && (
            <div className="auth-error" role="status" aria-live="polite">
              {t('auth.login_throttled')} ({Math.ceil(throttleMs / 1000)}s)
            </div>
          )}

          <div className="form-group">
            <label className="form-label" htmlFor="email-input">{t('auth.email')}</label>
            <input
              id="email-input"
              type="email"
              className="form-input"
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              disabled={loading || passkeyLoading}
              autoComplete="username"
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
              disabled={loading || passkeyLoading}
              autoComplete="current-password"
            />
          </div>

          <div className="form-row">
            <label className="form-checkbox-label">
              <input type="checkbox" disabled={loading || passkeyLoading} />
              {t('auth.remember_me')}
            </label>
            <a
              href="/auth/forgot-password"
              className="form-link"
              onClick={(e) => { e.preventDefault(); globalNavigate('/auth/forgot-password'); }}
            >
              {t('auth.forgot_password')}
            </a>
          </div>

          <button
            type="submit"
            className="auth-btn"
            disabled={loading || passkeyLoading || !email || !password || throttleMs > 0}
          >
            {loading && <div className="spinner" />}
            {t('auth.btn_signin')}
          </button>
        </form>

        <div className="auth-divider">{t('auth.or')}</div>

        <div className="social-btns">
          <button
            type="button"
            className="social-btn"
            onClick={() => handleOAuth('github')}
            disabled={loading || passkeyLoading}
          >
            <svg className="social-icon" viewBox="0 0 24 24" fill="currentColor">
              <path d="M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61C4.422 18.07 3.633 17.7 3.633 17.7c-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.399 3-.405 1.02.006 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 22.092 24 17.592 24 12.297c0-6.627-5.373-12-12-12"/>
            </svg>
            {t('auth.github_continue')}
          </button>

          <button
            type="button"
            className="social-btn"
            onClick={() => handleOAuth('google')}
            disabled={loading || passkeyLoading}
          >
            <svg className="social-icon" viewBox="0 0 24 24">
              <path fill="#EA4335" d="M12 5.04c1.62 0 3.08.56 4.22 1.64l3.15-3.15C17.45 1.74 14.9 1 12 1 7.35 1 3.38 3.67 1.38 7.57l3.85 2.99c.96-2.88 3.66-4.52 6.77-4.52z"/>
              <path fill="#4285F4" d="M23.49 12.27c0-.81-.07-1.59-.2-2.34H12v4.44h6.44c-.28 1.47-1.11 2.72-2.36 3.56l3.66 2.84c2.14-1.97 3.38-4.87 3.38-8.5z"/>
              <path fill="#FBBC05" d="M5.23 14.44c-.24-.72-.38-1.49-.38-2.29s.14-1.57.38-2.29L1.38 6.86C.5 8.61 0 10.56 0 12.64s.5 4.03 1.38 5.78l3.85-2.99z"/>
              <path fill="#34A853" d="M12 23c3.24 0 5.97-1.07 7.96-2.91l-3.66-2.84c-1.1.74-2.5 1.18-4.3 1.18-3.11 0-5.81-1.64-6.77-4.52L1.38 16.9C3.38 20.8 7.35 23 12 23z"/>
            </svg>
            {t('auth.google_continue')}
          </button>

          <button
            type="button"
            className={`social-btn social-btn--passkey ${passkeyLoading ? 'passkey-pulse' : ''}`}
            onClick={handlePasskeyLogin}
            disabled={loading || passkeyLoading}
          >
            🔑 {t('auth.passkey_continue')}
          </button>
        </div>
      </div>
    </div>
  )
}
