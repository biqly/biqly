import { useEffect, type ReactNode } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from './AuthProvider'

let navigateRef: ((path: string) => void) | null = null

export function globalNavigate(path: string) {
  if (navigateRef) {
    navigateRef(path)
  } else {
    if (path === window.location.pathname) return
    window.history.pushState(null, '', path)
    window.dispatchEvent(new PopStateEvent('popstate'))
  }
}

interface AuthGuardProps {
  children: ReactNode
}

export function AuthGuard({ children }: AuthGuardProps) {
  const { user, loading } = useAuth()
  const navigate = useNavigate()

  useEffect(() => {
    navigateRef = navigate
    return () => {
      navigateRef = null
    }
  }, [navigate])

  useEffect(() => {
    if (!loading && !user) {
      navigate('/auth/signin')
    }
  }, [user, loading, navigate])

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
