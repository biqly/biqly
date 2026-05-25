import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import { apiGetMe, apiLogin, apiLogout, apiRefresh, apiRegister } from '../../api/auth'
import type { AuthUser } from '../../types/auth'

interface AuthContextType {
  user: AuthUser | null
  accessToken: string | null
  loading: boolean
  login: (email: string, password: string) => Promise<void>
  loginWithTokens: (accessToken: string, refreshToken: string) => Promise<void>
  register: (email: string, password: string, displayName: string) => Promise<void>
  logout: () => Promise<void>
  refreshUser: () => Promise<void>
}

const AuthContext = createContext<AuthContextType | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<AuthUser | null>(null)
  const [accessToken, setAccessToken] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  const clearAuth = () => {
    setUser(null)
    setAccessToken(null)
    localStorage.removeItem('biqly_refresh_token')
  }

  const handleAuthSuccess = async (accToken: string, refToken: string) => {
    setAccessToken(accToken)
    localStorage.setItem('biqly_refresh_token', refToken)
    try {
      const profile = await apiGetMe(accToken)
      setUser(profile)
    } catch (err) {
      clearAuth()
      throw err
    }
  }

  const login = async (email: string, password: string) => {
    const resp = await apiLogin(email, password)
    await handleAuthSuccess(resp.access_token, resp.refresh_token)
  }

  const loginWithTokens = async (accToken: string, refToken: string) => {
    await handleAuthSuccess(accToken, refToken)
  }

  const register = async (email: string, password: string, displayName: string) => {
    const resp = await apiRegister(email, password, displayName)
    await handleAuthSuccess(resp.access_token, resp.refresh_token)
  }

  const logout = async () => {
    const refToken = localStorage.getItem('biqly_refresh_token')
    clearAuth()
    if (refToken) {
      try {
        await apiLogout(refToken)
      } catch {
        /* ignore */
      }
    }
  }

  const refreshUser = async () => {
    if (!accessToken) return
    try {
      const profile = await apiGetMe(accessToken)
      setUser(profile)
    } catch {
      /* ignore */
    }
  }

  useEffect(() => {
    const initAuth = async () => {
      const refToken = localStorage.getItem('biqly_refresh_token')
      if (!refToken) {
        setLoading(false)
        return
      }

      try {
        const resp = await apiRefresh(refToken)
        await handleAuthSuccess(resp.access_token, resp.refresh_token)
      } catch {
        clearAuth()
      } finally {
        setLoading(false)
      }
    }

    initAuth()
  }, [])

  // Silent refresh timer
  useEffect(() => {
    if (!accessToken) return

    // Refresh every 14 minutes (since access token expires in 15 minutes)
    const interval = window.setInterval(async () => {
      const refToken = localStorage.getItem('biqly_refresh_token')
      if (!refToken) {
        clearAuth()
        return
      }
      try {
        const resp = await apiRefresh(refToken)
        setAccessToken(resp.access_token)
        localStorage.setItem('biqly_refresh_token', resp.refresh_token)
      } catch {
        clearAuth()
      }
    }, 14 * 60 * 1000)

    return () => window.clearInterval(interval)
  }, [accessToken])

  return (
    <AuthContext.Provider
      value={{
        user,
        accessToken,
        loading,
        login,
        loginWithTokens,
        register,
        logout,
        refreshUser,
      }}
    >
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) {
    throw new Error('useAuth must be used within an AuthProvider')
  }
  return ctx
}
