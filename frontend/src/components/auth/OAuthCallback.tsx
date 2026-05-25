import { useEffect, useState } from 'react'
import abiLogo from '../../assets/abi-logo.png'
import { useT } from '../../i18n'
import { useAuth } from './AuthProvider'
import { globalNavigate } from './AuthGuard'

export default function OAuthCallback() {
  const t = useT()
  const { loginWithTokens } = useAuth()
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const handleCallback = async () => {
      const params = new URLSearchParams(window.location.search)
      const accessToken = params.get('access_token')
      const refreshToken = params.get('refresh_token')

      if (!accessToken || !refreshToken) {
        setError(t('auth.oauth_failed'))
        setTimeout(() => {
          globalNavigate('/auth/signin')
        }, 3000)
        return
      }

      try {
        await loginWithTokens(accessToken, refreshToken)
        globalNavigate('/datasources')
      } catch (err: any) {
        setError(err.message || t('auth.oauth_failed'))
        setTimeout(() => {
          globalNavigate('/auth/signin')
        }, 3000)
      }
    }

    handleCallback()
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
            <div className="auth-error">
              {error}
            </div>
            <button
              type="button"
              className="auth-btn"
              onClick={() => globalNavigate('/auth/signin')}
            >
              {t('auth.back_to_login')}
            </button>
          </div>
        ) : (
          <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '16px', padding: '16px' }}>
            <div className="spinner" style={{ width: '32px', height: '32px', borderTopColor: '#6366f1' }}></div>
            <span style={{ fontSize: '14px', color: 'var(--text-secondary)' }}>Completing sign in…</span>
          </div>
        )}
      </div>
    </div>
  )
}
