import { useEffect, type ReactNode } from 'react'
import { useAuth } from './AuthProvider'

export function globalNavigate(path: string) {
  if (path === window.location.pathname) return
  window.history.pushState(null, '', path)
  window.dispatchEvent(new PopStateEvent('popstate'))
}

interface AuthGuardProps {
  children: ReactNode
}

export function AuthGuard({ children }: AuthGuardProps) {
  const { user, loading } = useAuth()

  useEffect(() => {
    if (!loading && !user) {
      globalNavigate('/auth/signin')
    }
  }, [user, loading])

  if (loading) {
    return (
      <div className="auth-page">
        <div className="auth-card" style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '16px' }}>
          <div className="spinner" style={{ width: '32px', height: '32px', borderTopColor: '#6366f1' }}></div>
          <span style={{ fontSize: '14px', color: 'var(--text-secondary)' }}>Loading session…</span>
        </div>
      </div>
    )
  }

  if (!user) {
    return null
  }

  return <>{children}</>
}
