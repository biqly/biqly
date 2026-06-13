/* eslint-disable react-refresh/only-export-components */

import { type ReactNode, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'

import { authCardClass, authPageClass } from '../../lib/authClasses'
import { LoadingScreen } from '../ui/LoadingScreen'
import { useAuth } from './AuthProvider'

let navigateRef: ((path: string) => void) | null = null

export function globalNavigate(path: string) {
  if (navigateRef) {
    navigateRef(path)
  } else {
    if (path === window.location.pathname) {
      return
    }
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
    navigateRef = (path) => {
      void navigate(path)
    }
    return () => {
      navigateRef = null
    }
  }, [navigate])

  useEffect(() => {
    if (!loading && !user) {
      void navigate('/auth/signin')
    }
  }, [user, loading, navigate])

  if (loading) {
    return (
      <div className={authPageClass}>
        <div className={authCardClass}>
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
      void navigate('/')
    }
  }, [user, loading, navigate])

  if (loading) {
    return (
      <div className={authPageClass}>
        <div className={authCardClass}>
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
