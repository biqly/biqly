import { useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import { apiOAuthExchange } from '../../api/auth'
import abiLogo from '../../assets/abi-logo.png'
import { useT } from '../../i18n'
import { useAuth } from './AuthProvider'

export default function OAuthCallback() {
  const navigate = useNavigate()
  const t = useT()
  const { loginWithTokens } = useAuth()
  const [error, setError] = useState<string | null>(null)
  // React 18 StrictMode mounts effects twice in dev; guard against double
  // redemption of the single-use OAuth callback code. Without this the second
  // call gets 400 and would redirect the just-signed-in user back to signin.
  const startedRef = useRef(false)

  useEffect(() => {
    if (startedRef.current) {
      return
    }
    startedRef.current = true

    const handleCallback = async () => {
      const params = new URLSearchParams(window.location.search)
      const code = params.get('code')

      if (!code) {
        setError(t('auth.oauth_failed'))
        setTimeout(() => {
          navigate('/auth/signin')
        }, 3000)
        return
      }

      // Strip the code from the URL before the network call so that, even if
      // the component is force-remounted, the second mount cannot reuse it.
      window.history.replaceState(null, '', '/auth/callback')

      try {
        const resp = await apiOAuthExchange(code)
        if (resp.mfa_required && resp.mfa_token) {
          navigate(`/auth/signin?mfa_token=${encodeURIComponent(resp.mfa_token)}`)
          return
        }
        await loginWithTokens(resp.access_token, resp.refresh_token, resp.roles ?? [])
        navigate('/datasources')
      } catch (err: unknown) {
        const message = err instanceof Error ? err.message : t('auth.oauth_failed')
        setError(message)
        setTimeout(() => {
          navigate('/auth/signin')
        }, 3000)
      }
    }

    void handleCallback()
  }, [loginWithTokens, t])

  return (
    <div className="auth-page">
      <div className="auth-card">
        <div className="auth-header">
          <div className="auth-logo">
            <img src={abiLogo} alt="" width={34} height={34} />
          </div>
          <h1 className="auth-title">Authenticating…</h1>
        </div>

        {error ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
            <div className="auth-error">{error}</div>
            <button type="button" className="auth-btn" onClick={() => navigate('/auth/signin')}>
              {t('auth.back_to_login')}
            </button>
          </div>
        ) : (
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
              Completing sign in…
            </span>
          </div>
        )}
      </div>
    </div>
  )
}
