import { type SubmitEvent, useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import { apiGetPasswordPolicy, selfSignupEnabledFromPolicy } from '../../api/auth'
import abiLogo from '../../assets/abi-logo.png'
import { useLocale, useT } from '../../i18n'
import { Modal } from '../ui/Modal'
import { useAuth } from './AuthProvider'
import PasswordStrengthMeter from './PasswordStrengthMeter'
import { PrivacyPolicyEN, PrivacyPolicyTR, TermsOfUseEN, TermsOfUseTR } from './PolicyContent'

export default function SignUpPage() {
  const navigate = useNavigate()
  const t = useT()
  const [locale] = useLocale()
  const { register } = useAuth()

  const [displayName, setDisplayName] = useState('')
  const [email, setEmail] = useState('')
  const [password, setPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [agree, setAgree] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [passwordValid, setPasswordValid] = useState(false)
  const [termsOpen, setTermsOpen] = useState(false)
  const [privacyOpen, setPrivacyOpen] = useState(false)
  const [policyLoading, setPolicyLoading] = useState(true)
  const [signupAllowed, setSignupAllowed] = useState(true)

  useEffect(() => {
    let cancelled = false
    void apiGetPasswordPolicy()
      .then((policy) => {
        if (!cancelled) {
          setSignupAllowed(selfSignupEnabledFromPolicy(policy))
        }
      })
      .finally(() => {
        if (!cancelled) {
          setPolicyLoading(false)
        }
      })
    return () => {
      cancelled = true
    }
  }, [])

  const handleValidity = useCallback((info: { valid: boolean }) => {
    setPasswordValid(info.valid)
  }, [])

  const handleSubmit = async (e: SubmitEvent<HTMLFormElement>) => {
    e.preventDefault()
    if (!displayName || !email || !password || !confirmPassword) {
      return
    }

    if (password !== confirmPassword) {
      setError(t('auth.passwords_dont_match'))
      return
    }

    if (!passwordValid) {
      setError(t('auth.password_requirements_failed'))
      return
    }

    if (!agree) {
      setError(t('auth.terms_error'))
      return
    }

    setLoading(true)
    setError(null)

    try {
      await register(email, password, displayName)
      void navigate('/datasources')
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : 'Registration failed')
    } finally {
      setLoading(false)
    }
  }

  if (policyLoading) {
    return (
      <div className="auth-page">
        <div className="auth-card">
          <p
            className="text-[14px] text-foreground-muted"
            style={{ textAlign: 'center', margin: 0 }}
          >
            {t('common.loading')}
          </p>
        </div>
      </div>
    )
  }

  if (!signupAllowed) {
    return (
      <div className="auth-page">
        <div className="auth-card">
          <div className="flex flex-col items-center text-center mb-6">
            <div className="flex items-center justify-center w-12 h-12 rounded-xl bg-gradient-to-br from-[#6366f1] to-[#8b5cf6] mb-4 text-white shadow-[0_4px_12px_rgba(99,102,241,0.3)]">
              <img src={abiLogo} alt="" className="w-[34px] h-[34px] object-contain" />
            </div>
            <h1 className="text-[24px] font-bold text-foreground mb-1 tracking-tight">
              {t('auth.signup_closed_title')}
            </h1>
            <p className="text-[14px] text-foreground-muted">{t('auth.signup_closed_body')}</p>
            <p className="text-[14px] text-foreground-muted">{t('auth.signup_closed_contact')}</p>
          </div>
          <p className="text-[14px] text-foreground-muted" style={{ textAlign: 'center' }}>
            <a
              href="/auth/signin"
              className="text-[#6366f1] font-medium no-underline hover:underline"
              onClick={(e) => {
                e.preventDefault()
                void navigate('/auth/signin')
              }}
            >
              {t('auth.back_to_login')}
            </a>
          </p>
        </div>
      </div>
    )
  }

  return (
    <div className="auth-page">
      <div className="auth-card">
        <div className="flex flex-col items-center text-center mb-6">
          <div className="flex items-center justify-center w-12 h-12 rounded-xl bg-gradient-to-br from-[#6366f1] to-[#8b5cf6] mb-4 text-white shadow-[0_4px_12px_rgba(99,102,241,0.3)]">
            <img src={abiLogo} alt="" className="w-[34px] h-[34px] object-contain" />
          </div>
          <h1 className="text-[24px] font-bold text-foreground mb-1 tracking-tight">
            {t('auth.title_signup')}
          </h1>
          <p className="text-[14px] text-foreground-muted">
            {t('auth.already_account')}{' '}
            <a
              href="/auth/signin"
              className="text-[#6366f1] font-medium no-underline hover:underline"
              onClick={(e) => {
                e.preventDefault()
                void navigate('/auth/signin')
              }}
            >
              {t('auth.btn_signin')}
            </a>
          </p>
        </div>

        <form
          onSubmit={(e) => {
            void handleSubmit(e)
          }}
          className="flex flex-col gap-4"
        >
          {error && (
            <div
              className="p-[10px_12px] bg-error/8 border-l-[3px] border-error text-error text-[13px] rounded mb-2"
              role="alert"
              aria-live="assertive"
            >
              {error}
            </div>
          )}

          <div className="flex flex-col gap-1">
            <label className="text-[13px] font-medium text-foreground-muted" htmlFor="name-input">
              {t('auth.display_name')}
            </label>
            <input
              id="name-input"
              type="text"
              className={`w-full py-[10px] px-[14px] rounded-lg border border-border bg-[var(--bg-input,#ffffff)] text-foreground text-[14px] transition-all duration-250 focus:outline-none focus:border-accent focus:shadow-[0_0_0_3px_rgba(99,102,241,0.15)]`}
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              required
              disabled={loading}
              autoComplete="name"
            />
          </div>

          <div className="flex flex-col gap-1">
            <label className="text-[13px] font-medium text-foreground-muted" htmlFor="email-input">
              {t('auth.email')}
            </label>
            <input
              id="email-input"
              type="email"
              className={`w-full py-[10px] px-[14px] rounded-lg border border-border bg-[var(--bg-input,#ffffff)] text-foreground text-[14px] transition-all duration-250 focus:outline-none focus:border-accent focus:shadow-[0_0_0_3px_rgba(99,102,241,0.15)]`}
              value={email}
              onChange={(e) => setEmail(e.target.value)}
              required
              disabled={loading}
              autoComplete="email"
            />
          </div>

          <div className="flex flex-col gap-1">
            <label
              className="text-[13px] font-medium text-foreground-muted"
              htmlFor="password-input"
            >
              {t('auth.password')}
            </label>
            <input
              id="password-input"
              type="password"
              className={`w-full py-[10px] px-[14px] rounded-lg border border-border bg-[var(--bg-input,#ffffff)] text-foreground text-[14px] transition-all duration-250 focus:outline-none focus:border-accent focus:shadow-[0_0_0_3px_rgba(99,102,241,0.15)]`}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
              disabled={loading}
              autoComplete="new-password"
            />

            <PasswordStrengthMeter password={password} onValidityChange={handleValidity} />
          </div>

          <div className="flex flex-col gap-1">
            <label
              className="text-[13px] font-medium text-foreground-muted"
              htmlFor="confirm-password-input"
            >
              {t('auth.confirm_password')}
            </label>
            <input
              id="confirm-password-input"
              type="password"
              className={`w-full py-[10px] px-[14px] rounded-lg border border-border bg-[var(--bg-input,#ffffff)] text-foreground text-[14px] transition-all duration-250 focus:outline-none focus:border-accent focus:shadow-[0_0_0_3px_rgba(99,102,241,0.15)]`}
              value={confirmPassword}
              onChange={(e) => setConfirmPassword(e.target.value)}
              required
              disabled={loading}
              autoComplete="new-password"
            />
          </div>

          <div className="flex items-center justify-between text-[13px]">
            <label
              className="flex items-center gap-1.5 cursor-pointer text-foreground-muted"
              style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', cursor: 'pointer' }}
            >
              <input
                type="checkbox"
                checked={agree}
                onChange={(e) => setAgree(e.target.checked)}
                disabled={loading}
                style={{ cursor: 'pointer' }}
              />
              <span style={{ fontSize: '0.875rem', color: 'var(--text-secondary)' }}>
                {t('auth.agree_to')}{' '}
                <button
                  type="button"
                  onClick={() => setTermsOpen(true)}
                  style={{
                    background: 'transparent',
                    border: 0,
                    padding: 0,
                    color: 'var(--accent)',
                    textDecoration: 'underline',
                    cursor: 'pointer',
                    fontSize: 'inherit',
                    fontFamily: 'inherit',
                    display: 'inline',
                  }}
                >
                  {t('auth.terms_of_use')}
                </button>{' '}
                {t('auth.and')}{' '}
                <button
                  type="button"
                  onClick={() => setPrivacyOpen(true)}
                  style={{
                    background: 'transparent',
                    border: 0,
                    padding: 0,
                    color: 'var(--accent)',
                    textDecoration: 'underline',
                    cursor: 'pointer',
                    fontSize: 'inherit',
                    fontFamily: 'inherit',
                    display: 'inline',
                  }}
                >
                  {t('auth.privacy_policy')}
                </button>
                {t('auth.agree_suffix') && ` ${t('auth.agree_suffix')}`}
              </span>
            </label>
          </div>

          <button
            type="submit"
            className="flex items-center justify-center gap-2 w-full py-[11px] px-[16px] rounded-lg border-none bg-gradient-to-br from-accent to-[var(--accent-strong)] text-white text-[14px] font-semibold cursor-pointer transition-all duration-150 shadow-[0_4px_10px_rgba(99,102,241,0.2)] hover:opacity-95 hover:-translate-y-[1px] active:translate-y-0 disabled:opacity-60 disabled:cursor-not-allowed disabled:transform-none"
            disabled={loading || !displayName || !email || !password || !confirmPassword || !agree}
          >
            {loading && (
              <div className="w-4 h-4 border-2 border-white/30 rounded-full border-t-white animate-spin" />
            )}
            {t('auth.btn_signup')}
          </button>
        </form>
      </div>

      {/* Terms of Use Modal */}
      <Modal open={termsOpen} title={t('auth.terms_of_use')} onClose={() => setTermsOpen(false)}>
        {locale.startsWith('tr') ? <TermsOfUseTR /> : <TermsOfUseEN />}
        <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: '1.5rem' }}>
          <button
            type="button"
            className="btn btn-primary"
            onClick={() => {
              setAgree(true)
              setTermsOpen(false)
            }}
            style={{ width: 'auto' }}
          >
            {t('common.confirm_ok') || 'OK'}
          </button>
        </div>
      </Modal>

      {/* Privacy Policy Modal */}
      <Modal
        open={privacyOpen}
        title={t('auth.privacy_policy')}
        onClose={() => setPrivacyOpen(false)}
      >
        {locale.startsWith('tr') ? <PrivacyPolicyTR /> : <PrivacyPolicyEN />}
        <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: '1.5rem' }}>
          <button
            type="button"
            className="btn btn-primary"
            onClick={() => {
              setAgree(true)
              setPrivacyOpen(false)
            }}
            style={{ width: 'auto' }}
          >
            {t('common.confirm_ok') || 'OK'}
          </button>
        </div>
      </Modal>
    </div>
  )
}
