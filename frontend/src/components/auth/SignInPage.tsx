import { type SubmitEvent, useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import {
  apiGetPasswordPolicy,
  apiMFALogin,
  apiPasskeyLoginBegin,
  apiPasskeyLoginFinish,
  firstUserSetupRequiredFromPolicy,
  selfSignupEnabledFromPolicy,
} from '../../api/auth'
import abiLogo from '../../assets/abi-logo.png'
import { useAutofocus } from '../../hooks/useAutofocus'
import { useT } from '../../i18n'
import { authCardClass, authPageClass } from '../../lib/authClasses'
import {
  base64urlToBuffer,
  bufferToBase64url,
  resolvePasskeyLoginOptions,
} from '../../utils/webauthn'
import { useAuth } from './AuthProvider'
import { SignInCredentialsForm } from './SignInCredentialsForm'
import { SignInMfaForm } from './SignInMfaForm'
import { sessionExpiredBanner } from './signInSessionBanner'

const FAILED_LOGIN_BACKOFFS_MS = [0, 1000, 2000, 4000, 8000]

export default function SignInPage() {
  const navigate = useNavigate()
  const t = useT()
  const { login, loginWithTokens } = useAuth()

  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [passkeyLoading, setPasskeyLoading] = useState(false)
  const [throttleMs, setThrottleMs] = useState(0)
  const [sessionBanner, setSessionBanner] = useState<{ title: string; body: string } | null>(null)
  const failureCountRef = useRef(0)

  // Multi-Factor Authentication (MFA) States
  const [mfaRequired, setMfaRequired] = useState(false)
  const [mfaToken, setMfaToken] = useState('')
  const [mfaCode, setMfaCode] = useState('')
  const [mfaLoading, setMfaLoading] = useState(false)
  const [signupAllowed, setSignupAllowed] = useState(true)
  const [firstUserSetupRequired, setFirstUserSetupRequired] = useState(false)
  const [ldapEnabled, setLdapEnabled] = useState(false)
  const mfaCodeRef = useAutofocus<HTMLInputElement>(mfaRequired)

  useEffect(() => {
    let cancelled = false
    apiGetPasswordPolicy()
      .then((policy) => {
        if (cancelled) {
          return
        }
        setSignupAllowed(selfSignupEnabledFromPolicy(policy))
        setFirstUserSetupRequired(firstUserSetupRequiredFromPolicy(policy))
        setLdapEnabled(policy.ldap_enabled === true)
      })
      .catch(() => {
        if (!cancelled) {
          setSignupAllowed(true)
        }
      })
    return () => {
      cancelled = true
    }
  }, [])

  // Check for redirect MFA challenge token from OAuth flow on mount
  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    const token = params.get('mfa_token')
    if (token) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setMfaToken(token)
      setMfaRequired(true)
      // Clean query string
      window.history.replaceState(null, '', '/auth/signin')
    }
  }, [])

  // Render an explanatory banner when redirected here from a session-expiry
  // event (set by AuthProvider via ?expired=<reason>).
  useEffect(() => {
    const params = new URLSearchParams(window.location.search)
    const reason = params.get('expired')
    if (!reason) {
      return
    }

    // eslint-disable-next-line react-hooks/set-state-in-effect
    setSessionBanner(sessionExpiredBanner(reason, t))
    window.history.replaceState(null, '', '/auth/signin')
  }, [t])

  useEffect(() => {
    if (throttleMs <= 0) {
      return
    }
    const interval = window.setInterval(() => {
      setThrottleMs((prev) => Math.max(0, prev - 250))
    }, 250)
    return () => window.clearInterval(interval)
  }, [throttleMs])

  const handleSubmit = async (e: SubmitEvent<HTMLFormElement>) => {
    e.preventDefault()
    if (!email || !password || throttleMs > 0) {
      return
    }

    setLoading(true)
    setError(null)
    try {
      const mfaResult = await login(email, password)
      if (mfaResult && mfaResult.mfaRequired) {
        setMfaToken(mfaResult.mfaToken ?? '')
        setMfaRequired(true)
        return
      }
      failureCountRef.current = 0
      void navigate('/datasources')
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Login failed')
      // Exponential client-side backoff. Server already rate-limits, this is
      // pure UX so a frustrated user does not hammer the form and pin the
      // browser tab.
      const idx = Math.min(failureCountRef.current + 1, FAILED_LOGIN_BACKOFFS_MS.length - 1)
      failureCountRef.current = idx
      const wait = FAILED_LOGIN_BACKOFFS_MS[idx] ?? 0
      if (wait > 0) {
        setThrottleMs(wait)
      }
    } finally {
      setLoading(false)
    }
  }

  const handleMFALoginSubmit = async (e: SubmitEvent<HTMLFormElement>) => {
    e.preventDefault()
    if (!mfaCode.trim() || !mfaToken || mfaLoading) {
      return
    }

    setMfaLoading(true)
    setError(null)
    try {
      const resp = await apiMFALogin(mfaToken, mfaCode.trim())
      await loginWithTokens(resp.access_token, resp.roles)
      void navigate('/datasources')
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : '2FA Verification failed')
    } finally {
      setMfaLoading(false)
    }
  }

  const handlePasskeyLogin = async () => {
    setPasskeyLoading(true)
    setError(null)
    try {
      const beginResp = await apiPasskeyLoginBegin()
      const publicKeyOptions = resolvePasskeyLoginOptions(beginResp)

      const options: CredentialRequestOptions = {
        publicKey: {
          ...publicKeyOptions,
          challenge: base64urlToBuffer(publicKeyOptions.challenge),
          allowCredentials: publicKeyOptions.allowCredentials?.map((cred) => ({
            ...cred,
            type: cred.type ?? 'public-key',
            id: base64urlToBuffer(cred.id),
          })),
        },
      }

      const credential = await navigator.credentials.get(options)
      if (!credential) {
        throw new Error('No credential returned')
      }

      const assertion = credential as PublicKeyCredential
      const response = assertion.response as AuthenticatorAssertionResponse
      const credentialJson = {
        id: assertion.id,
        rawId: bufferToBase64url(assertion.rawId),
        type: assertion.type,
        response: {
          clientDataJSON: bufferToBase64url(response.clientDataJSON),
          authenticatorData: bufferToBase64url(response.authenticatorData),
          signature: bufferToBase64url(response.signature),
          userHandle: response.userHandle ? bufferToBase64url(response.userHandle) : null,
        },
      }

      const finishResp = await apiPasskeyLoginFinish(credentialJson)
      await loginWithTokens(finishResp.access_token, finishResp.roles)
      void navigate('/datasources')
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Passkey login failed')
    } finally {
      setPasskeyLoading(false)
    }
  }

  const handleOAuth = (provider: string) => {
    window.location.href = `/api/auth/oauth/${provider}`
  }

  return (
    <div className={authPageClass}>
      <div className={authCardClass}>
        <div className="mb-6 flex flex-col items-center text-center">
          <div className="mb-4 flex h-12 w-12 items-center justify-center rounded-xl bg-linear-to-br from-[#6366f1] to-[#8b5cf6] text-white shadow-[0_4px_12px_rgba(99,102,241,0.3)]">
            <img src={abiLogo} alt="" className="h-8.5 w-8.5 object-contain" />
          </div>
          <h1 className="mb-1 text-[24px] font-bold tracking-tight text-foreground">
            {t('auth.title_signin')}
          </h1>
          {firstUserSetupRequired ? (
            <div className="mt-2 rounded-lg border border-accent/30 bg-accent/8 px-3 py-2 text-[13px] text-foreground-muted">
              <p className="mb-1 font-semibold text-foreground">{t('auth.first_setup_title')}</p>
              <p className="mb-2">{t('auth.first_setup_body')}</p>
              <a
                href="/auth/signup"
                className="font-semibold text-accent no-underline hover:underline"
                onClick={(e) => {
                  e.preventDefault()
                  void navigate('/auth/signup')
                }}
              >
                {t('auth.first_setup_cta')}
              </a>
            </div>
          ) : signupAllowed ? (
            <p className="text-[14px] text-foreground-muted">
              {t('auth.no_account')}{' '}
              <a
                href="/auth/signup"
                className="font-medium text-[#6366f1] no-underline hover:underline"
                onClick={(e) => {
                  e.preventDefault()
                  void navigate('/auth/signup')
                }}
              >
                {t('auth.btn_signup')}
              </a>
            </p>
          ) : null}
        </div>

        {sessionBanner && (
          <div
            className="mb-2 rounded border-l-[3px] border-accent bg-accent/8 p-[10px_12px] text-[13px] text-foreground-muted"
            role="status"
            aria-live="polite"
          >
            <strong>{sessionBanner.title}</strong>
            <div>{sessionBanner.body}</div>
          </div>
        )}

        {mfaRequired ? (
          <SignInMfaForm
            t={t}
            error={error}
            mfaCode={mfaCode}
            setMfaCode={setMfaCode}
            mfaLoading={mfaLoading}
            mfaCodeRef={mfaCodeRef}
            onSubmit={(event) => {
              void handleMFALoginSubmit(event)
            }}
            onCancel={() => {
              setMfaRequired(false)
              setMfaToken('')
              setMfaCode('')
              setError(null)
            }}
          />
        ) : (
          <SignInCredentialsForm
            t={t}
            ldapEnabled={ldapEnabled}
            error={error}
            throttleMs={throttleMs}
            email={email}
            setEmail={setEmail}
            password={password}
            setPassword={setPassword}
            loading={loading}
            passkeyLoading={passkeyLoading}
            onSubmit={(event) => {
              void handleSubmit(event)
            }}
            onForgotPassword={() => {
              void navigate('/auth/forgot-password')
            }}
            onOAuth={handleOAuth}
            onPasskeyLogin={() => {
              void handlePasskeyLogin()
            }}
          />
        )}
      </div>
    </div>
  )
}
