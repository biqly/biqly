import { useEffect, type ReactNode } from 'react'
import { useNavigate } from 'react-router-dom'
import { useAuth } from './AuthProvider'
import { LoadingScreen } from '../ui/LoadingScreen'

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
        <div className="auth-card">
          <LoadingScreen minHeight="auto" variant="center" label="Loading session…" />
        </div>
      </div>
    )
  }

  if (!user) {
    return null
  }

  return <>{children}</>
}

export function GuestGuard({ children }: AuthGuardProps) {
  const { user, loading } = useAuth()
  const navigate = useNavigate()

  useEffect(() => {
    if (!loading && user) {
      navigate('/')
    }
  }, [user, loading, navigate])

  if (loading) {
    return (
      <div className="auth-page">
        <div className="auth-card">
          <LoadingScreen minHeight="auto" variant="center" label="Loading session…" />
        </div>
      </div>
    )
  }

  if (user) {
    return null
  }

  return <>{children}</>
}
