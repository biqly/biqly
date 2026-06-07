import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import { apiVerifyEmail } from '../../api/auth'
import abiLogo from '../../assets/abi-logo.png'
import { useT } from '../../i18n'

export default function VerifyEmailPage() {
  const navigate = useNavigate()
  const t = useT()
  const [status, setStatus] = useState<'verifying' | 'success' | 'error'>('verifying')
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const verify = async () => {
      const params = new URLSearchParams(window.location.search)
      const token = params.get('token')

      if (!token) {
        setStatus('error')
        setError(t('auth.verify_failed'))
        return
      }

      try {
        await apiVerifyEmail(token)
        setStatus('success')
        setTimeout(() => {
          void navigate('/auth/signin')
        }, 3000)
      } catch (err: unknown) {
        setStatus('error')
        setError(err instanceof Error ? err.message : t('auth.verify_failed'))
      }
    }

    void verify()
  }, [t])

  return (
    <div className="auth-page">
      <div className="auth-card">
        <div className="auth-header">
          <div className="auth-logo">
            <img src={abiLogo} alt="" width={34} height={34} />
          </div>
          <h1 className="auth-title">{t('auth.title_verify')}</h1>
        </div>

        {status === 'verifying' && (
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
              Verifying your email address…
            </span>
          </div>
        )}

        {status === 'success' && <div className="auth-success">{t('auth.verify_success')}</div>}

        {status === 'error' && (
          <div style={{ display: 'flex', flexDirection: 'column', gap: '16px' }}>
            <div className="auth-error">{error}</div>
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
        )}
      </div>
    </div>
  )
}
