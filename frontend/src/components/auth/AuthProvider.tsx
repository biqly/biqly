/* eslint-disable react-refresh/only-export-components */

import {
  createContext,
  type ReactNode,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
} from 'react'
import { useNavigate } from 'react-router-dom'

import { ApiError, setGlobalAccessToken } from '../../api/apiClient'
import {
  apiGetMe,
  apiGetMyPermissions,
  apiLogin,
  apiLogout,
  apiRefresh,
  apiRegister,
  apiSetActiveWorkspace,
  registerResponseHasSession,
} from '../../api/auth'
import type { AuthUser } from '../../types/auth'

const LEGACY_REFRESH_TOKEN_KEY = 'biqly_refresh_token'

// isAuthRejection reports whether the error is a definitive credential
// rejection from the server. Transient failures (429 rate limit, 5xx,
// network errors with status 0) must NOT end the session: the token may
// still be perfectly valid.
function isAuthRejection(err: unknown): boolean {
  return err instanceof ApiError && (err.status === 400 || err.status === 401 || err.status === 403)
}

const TRANSIENT_RETRY_DELAYS_MS = [1000, 2000]

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms))
}

// withTransientRetry retries fn on transient failures (429/5xx/network) with
// a short backoff, but rethrows credential rejections immediately.
async function withTransientRetry<T>(fn: () => Promise<T>): Promise<T> {
  for (let attempt = 0; ; attempt++) {
    try {
      return await fn()
    } catch (err) {
      const delay = TRANSIENT_RETRY_DELAYS_MS[attempt]
      if (isAuthRejection(err) || delay === undefined) {
        throw err
      }
      await sleep(delay)
    }
  }
}

// classifySessionExpiry inspects the server-returned error message and maps it
// to one of the i18n reasons so the signin page can show the right banner.
function classifySessionExpiry(err: unknown): string {
  if (!(err instanceof Error)) {
    return 'unknown'
  }
  const msg = err.message.toLowerCase()
  if (msg.includes('idle')) {
    return 'idle'
  }
  if (msg.includes('absolute') || msg.includes('maximum lifetime')) {
    return 'absolute'
  }
  if (msg.includes('revoked')) {
    return 'revoked'
  }
  return 'unknown'
}

interface AuthContextType {
  user: AuthUser | null
  accessToken: string | null
  roles: string[]
  permissions: string[]
  isSuperAdmin: boolean
  hasPermission: (...perms: string[]) => boolean
  loading: boolean
  login: (
    email: string,
    password: string,
  ) => Promise<{ mfaRequired?: boolean; mfaToken?: string } | void>
  loginWithTokens: (accessToken: string, roles?: string[]) => Promise<void>
  register: (
    email: string,
    password: string,
    displayName: string,
  ) => Promise<{ authenticated: boolean; verificationPending: boolean }>
  logout: () => Promise<void>
  refreshUser: () => Promise<void>
  setActiveWorkspace: (workspaceID: string) => Promise<void>
}

const AuthContext = createContext<AuthContextType | null>(null)

// Access tokens live 15 minutes (BI_AUTH_JWT_ACCESS_TTL); refresh slightly
// earlier so in-flight requests never carry an expired token.
const TOKEN_REFRESH_MS = 14 * 60 * 1000

export function AuthProvider({ children }: { children: ReactNode }) {
  const navigate = useNavigate()
  const [user, setUser] = useState<AuthUser | null>(null)
  const [accessToken, setAccessTokenState] = useState<string | null>(null)
  const [roles, setRoles] = useState<string[]>([])
  const [permissions, setPermissions] = useState<string[]>([])
  const [isSuperAdmin, setIsSuperAdmin] = useState(false)
  const [loading, setLoading] = useState(true)
  // When the current access token was obtained; drives the staleness check on
  // tab wake. A ref (not state) so event listeners see the latest value.
  const lastTokenAtRef = useRef(0)
  // Prevents concurrent refresh calls: the refresh token rotates on use, and a
  // second call with the already-rotated token trips the server's token-family
  // theft protection, revoking every session.
  const refreshInFlightRef = useRef(false)

  // Mirror the session token into the apiClient module so every fetch —
  // including call sites that don't thread the token explicitly — sends
  // Authorization. Required now that the backend enforces JWTs on /api.
  // The mirror must happen synchronously with the state update: child
  // components' effects run before this provider's own effects, so a
  // useEffect-based mirror would let them fire requests with a stale
  // (null) module token and get 401s.
  const setAccessToken = useCallback((token: string | null) => {
    setGlobalAccessToken(token)
    setAccessTokenState(token)
  }, [])

  const clearAuth = useCallback(() => {
    setUser(null)
    setAccessToken(null)
    setRoles([])
    setPermissions([])
    setIsSuperAdmin(false)
    localStorage.removeItem(LEGACY_REFRESH_TOKEN_KEY)
  }, [setAccessToken])

  // loadPermissions fetches the caller's effective permissions so the UI can
  // disable controls the user is not allowed to use. Failure is non-fatal:
  // the backend still enforces every mutation, so we just fall back to an
  // empty permission set (controls disabled) rather than blocking sign-in.
  const loadPermissions = useCallback(async (accToken: string) => {
    try {
      const p = await apiGetMyPermissions(accToken)
      setPermissions(p.permissions)
      setIsSuperAdmin(p.is_super_admin)
    } catch {
      setPermissions([])
      setIsSuperAdmin(false)
    }
  }, [])

  const handleAuthSuccess = useCallback(
    async (accToken: string, nextRoles: string[] = []) => {
      lastTokenAtRef.current = Date.now()
      setAccessToken(accToken)
      setRoles(nextRoles)
      localStorage.removeItem(LEGACY_REFRESH_TOKEN_KEY)
      try {
        // The token was just issued, so a /me failure here is almost always
        // transient (rate limit, blip). Retry briefly before giving up —
        // otherwise a single 429 bounces a valid session to the signin page.
        const profile = await withTransientRetry(() => apiGetMe(accToken))
        setUser(profile)
      } catch (err) {
        clearAuth()
        throw err
      }
      await loadPermissions(accToken)
    },
    [clearAuth, loadPermissions, setAccessToken],
  )

  const hasPermission = useCallback(
    (...perms: string[]) => {
      if (isSuperAdmin) {
        return true
      }
      if (perms.length === 0) {
        return false
      }
      return perms.some((p) => permissions.includes(p))
    },
    [isSuperAdmin, permissions],
  )

  const login = async (email: string, password: string) => {
    const resp = await apiLogin(email, password)
    if (resp.mfa_required) {
      return { mfaRequired: true, mfaToken: resp.mfa_token }
    }
    await handleAuthSuccess(resp.access_token, resp.roles)
    return undefined
  }

  const loginWithTokens = async (accToken: string, nextRoles: string[] = []) => {
    await handleAuthSuccess(accToken, nextRoles)
  }

  const register = async (email: string, password: string, displayName: string) => {
    const resp = await apiRegister(email, password, displayName)
    if (!registerResponseHasSession(resp)) {
      return { authenticated: false, verificationPending: resp.verification_pending === true }
    }
    await handleAuthSuccess(resp.access_token, resp.roles)
    return { authenticated: true, verificationPending: resp.verification_pending === true }
  }

  const logout = async () => {
    clearAuth()
    try {
      await apiLogout()
    } catch {
      /* ignore */
    }
  }

  const refreshUser = async () => {
    if (!accessToken) {
      return
    }
    try {
      const profile = await apiGetMe(accessToken)
      setUser(profile)
    } catch {
      /* ignore */
    }
  }

  const setActiveWorkspace = async (workspaceID: string) => {
    if (!accessToken) {
      throw new Error('not authenticated')
    }
    const resp = await apiSetActiveWorkspace(accessToken, workspaceID)
    setAccessToken(resp.access_token)
    setUser((prev) => (prev ? { ...prev, active_workspace_id: resp.active_workspace_id } : prev))
  }

  useEffect(() => {
    const initAuth = async () => {
      const legacyRefresh = localStorage.getItem(LEGACY_REFRESH_TOKEN_KEY)

      try {
        const resp = await withTransientRetry(() =>
          legacyRefresh ? apiRefresh(legacyRefresh) : apiRefresh(),
        )
        await handleAuthSuccess(resp.access_token, resp.roles)
      } catch {
        clearAuth()
      } finally {
        setLoading(false)
      }
    }

    void initAuth()
  }, [clearAuth, handleAuthSuccess])

  // Silent refresh: the interval covers active tabs, while the visibility and
  // focus listeners cover wake-from-sleep and throttled background tabs where
  // the timer did not fire before the 15-minute access token expired. Without
  // the wake path, the first requests after resume go out with a stale token
  // (401 from the auth service, 403 from optional-JWT services).
  useEffect(() => {
    if (!accessToken) {
      return
    }

    const silentRefresh = async () => {
      if (refreshInFlightRef.current) {
        return
      }
      refreshInFlightRef.current = true
      try {
        const resp = await apiRefresh()
        lastTokenAtRef.current = Date.now()
        setAccessToken(resp.access_token)
        setRoles(resp.roles)
        await loadPermissions(resp.access_token)
      } catch (err: unknown) {
        // Transient failure (rate limit, backend blip, network) — keep the
        // session; the current access token may still be valid and the next
        // interval tick retries the refresh.
        if (!isAuthRejection(err)) {
          return
        }
        // Refresh rejected — classify the server-side reason so the next
        // sign-in screen can render an explanatory banner
        // (idle/absolute/revoked) and not just bounce the user to a blank
        // login form.
        const reason = classifySessionExpiry(err)
        sessionStorage.setItem('biqly_session_expired_reason', reason)
        clearAuth()
        if (window.location.pathname !== '/auth/signin') {
          void navigate('/auth/signin?expired=' + reason)
        }
      } finally {
        refreshInFlightRef.current = false
      }
    }

    const refreshIfStale = () => {
      if (
        document.visibilityState === 'visible' &&
        Date.now() - lastTokenAtRef.current >= TOKEN_REFRESH_MS
      ) {
        void silentRefresh()
      }
    }

    const interval = window.setInterval(() => {
      void silentRefresh()
    }, TOKEN_REFRESH_MS)
    document.addEventListener('visibilitychange', refreshIfStale)
    window.addEventListener('focus', refreshIfStale)
    return () => {
      window.clearInterval(interval)
      document.removeEventListener('visibilitychange', refreshIfStale)
      window.removeEventListener('focus', refreshIfStale)
    }
  }, [accessToken, clearAuth, loadPermissions, navigate, setAccessToken])

  return (
    <AuthContext.Provider
      value={{
        user,
        accessToken,
        roles,
        permissions,
        isSuperAdmin,
        hasPermission,
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
