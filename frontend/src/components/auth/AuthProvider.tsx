import { createContext, useContext, useEffect, useState, type ReactNode } from 'react'
import { useNavigate } from 'react-router-dom'
import { apiGetMe, apiLogin, apiLogout, apiRefresh, apiRegister, apiSetActiveWorkspace } from '../../api/auth'
import type { AuthUser } from '../../types/auth'

// classifySessionExpiry inspects the server-returned error message and maps it
// to one of the i18n reasons so the signin page can show the right banner.
function classifySessionExpiry(err: unknown): string {
  if (!(err instanceof Error)) return 'unknown'
  const msg = err.message.toLowerCase()
  if (msg.includes('idle')) return 'idle'
  if (msg.includes('absolute') || msg.includes('maximum lifetime')) return 'absolute'
  if (msg.includes('revoked')) return 'revoked'
  return 'unknown'
}

interface AuthContextType {
  user: AuthUser | null
  accessToken: string | null
  roles: string[]
  loading: boolean
  login: (email: string, password: string) => Promise<{ mfaRequired?: boolean; mfaToken?: string } | void>
  loginWithTokens: (accessToken: string, refreshToken: string, roles?: string[]) => Promise<void>
  register: (email: string, password: string, displayName: string) => Promise<void>
  logout: () => Promise<void>
  refreshUser: () => Promise<void>
  setActiveWorkspace: (workspaceID: string) => Promise<void>
}

const AuthContext = createContext<AuthContextType | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const navigate = useNavigate()
  const [user, setUser] = useState<AuthUser | null>(null)
  const [accessToken, setAccessToken] = useState<string | null>(null)
  const [roles, setRoles] = useState<string[]>([])
  const [loading, setLoading] = useState(true)

  const clearAuth = () => {
    setUser(null)
    setAccessToken(null)
    setRoles([])
    localStorage.removeItem('biqly_refresh_token')
  }

  const handleAuthSuccess = async (accToken: string, refToken: string, nextRoles: string[] = []) => {
    setAccessToken(accToken)
    setRoles(nextRoles)
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
    if (resp.mfa_required) {
      return { mfaRequired: true, mfaToken: resp.mfa_token }
    }
    await handleAuthSuccess(resp.access_token, resp.refresh_token, resp.roles)
  }

  const loginWithTokens = async (accToken: string, refToken: string, nextRoles: string[] = []) => {
    await handleAuthSuccess(accToken, refToken, nextRoles)
  }

  const register = async (email: string, password: string, displayName: string) => {
    const resp = await apiRegister(email, password, displayName)
    await handleAuthSuccess(resp.access_token, resp.refresh_token, resp.roles)
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

  const setActiveWorkspace = async (workspaceID: string) => {
    if (!accessToken) throw new Error('not authenticated')
    const resp = await apiSetActiveWorkspace(accessToken, workspaceID)
    setAccessToken(resp.access_token)
    setUser((prev) => (prev ? { ...prev, active_workspace_id: resp.active_workspace_id } : prev))
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
        await handleAuthSuccess(resp.access_token, resp.refresh_token, resp.roles)
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
        setRoles(resp.roles)
        localStorage.setItem('biqly_refresh_token', resp.refresh_token)
      } catch (err: unknown) {
        // Refresh failed — classify the server-side reason so the next sign-in
        // screen can render an explanatory banner (idle/absolute/revoked) and
        // not just bounce the user to a blank login form.
        const reason = classifySessionExpiry(err)
        sessionStorage.setItem('biqly_session_expired_reason', reason)
        clearAuth()
        if (window.location.pathname !== '/auth/signin') {
          navigate('/auth/signin?expired=' + reason)
        }
      }
    }, 14 * 60 * 1000)

    return () => window.clearInterval(interval)
  }, [accessToken, navigate])

  return (
    <AuthContext.Provider
      value={{
        user,
        accessToken,
        roles,
        loading,
        login,
        loginWithTokens,
        register,
        logout,
        refreshUser,
        setActiveWorkspace,
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
