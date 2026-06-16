import { type MouseEvent, type SubmitEvent, useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import {
  apiGetPasswordPolicy,
  firstUserSetupRequiredFromPolicy,
  selfSignupEnabledFromPolicy,
} from '../../api/auth'
import abiLogo from '../../assets/abi-logo.png'
import { useLocale, useT } from '../../i18n'
import {
  authCardClass,
  authCheckboxRowClass,
  authFieldClass,
  authFieldHintErrorClass,
  authFieldHintSuccessClass,
  authFormClass,
  authIconBoxClass,
  authInlineLinkClass,
  authInputClass,
  authInputErrorClass,
  authLabelClass,
  authLinkBtnClass,
  authPageClass,
  authSubmitBtnClass,
} from '../../lib/authClasses'
import { legacyButtonClass } from '../../lib/buttonClasses'
import { cn } from '../../lib/cn'
import { isValidEmailFormat } from '../../lib/emailValidation'
import { legacyFeedbackClass } from '../../lib/feedbackClasses'
import { Modal } from '../ui/Modal'
import { useAuth } from './AuthProvider'
import PasswordStrengthMeter from './PasswordStrengthMeter'
import { PrivacyPolicyEN, PrivacyPolicyTR, TermsOfUseEN, TermsOfUseTR } from './PolicyContent'

function confirmPasswordMismatch(password: string, confirmPassword: string): boolean {
  return confirmPassword.length > 0 && password !== confirmPassword
}

interface SubmitDisabledState {
  loading: boolean
  displayName: string
  email: string
  showEmailInvalid: boolean
  password: string
  confirmPassword: string
  agree: boolean
  showPasswordMismatch: boolean
  passwordValid: boolean
}

function isSubmitDisabled(state: SubmitDisabledState): boolean {
  return (
    state.loading ||
    !state.displayName ||
    !state.email ||
    state.showEmailInvalid ||
    !state.password ||
    !state.confirmPassword ||
    !state.agree ||
    state.showPasswordMismatch ||
    !state.passwordValid
  )
}

interface EmailFieldProps {
  email: string
  onEmailChange: (value: string) => void
  disabled?: boolean
}

function EmailField({ email, onEmailChange, disabled }: EmailFieldProps) {
  const t = useT()
  const showInvalid = email.length > 0 && !isValidEmailFormat(email)

  return (
    <div className={authFieldClass}>
      <label className={authLabelClass} htmlFor="email-input">
        {t('auth.email')}
      </label>
      <input
        id="email-input"
        type="email"
        className={cn(authInputClass, showInvalid && authInputErrorClass)}
        value={email}
        onChange={(e) => onEmailChange(e.target.value)}
        required
        disabled={disabled}
        autoComplete="email"
        spellCheck={false}
        aria-invalid={showInvalid || undefined}
        aria-describedby={showInvalid ? 'email-hint' : undefined}
      />
      {showInvalid && (
        <p id="email-hint" className={authFieldHintErrorClass} role="alert">
          {t('auth.invalid_email')}
        </p>
      )}
    </div>
  )
}

interface ConfirmPasswordFieldProps {
  password: string
  confirmPassword: string
  onConfirmChange: (value: string) => void
  disabled?: boolean
}

function ConfirmPasswordField({
  password,
  confirmPassword,
  onConfirmChange,
  disabled,
}: ConfirmPasswordFieldProps) {
  const t = useT()
  const showMismatch = confirmPasswordMismatch(password, confirmPassword)

  return (
    <div className={authFieldClass}>
      <label className={authLabelClass} htmlFor="confirm-password-input">
        {t('auth.confirm_password')}
      </label>
      <input
        id="confirm-password-input"
        type="password"
        className={cn(authInputClass, showMismatch && authInputErrorClass)}
        value={confirmPassword}
        onChange={(e) => onConfirmChange(e.target.value)}
        required
        disabled={disabled}
        autoComplete="new-password"
        aria-invalid={showMismatch || undefined}
        aria-describedby={confirmPassword.length > 0 ? 'confirm-password-hint' : undefined}
      />
      {confirmPassword.length > 0 && (
        <p
          id="confirm-password-hint"
          className={showMismatch ? authFieldHintErrorClass : authFieldHintSuccessClass}
          role={showMismatch ? 'alert' : 'status'}
        >
          {showMismatch ? t('auth.passwords_dont_match') : t('auth.passwords_match')}
        </p>
      )}
    </div>
  )
}

interface SignUpHeaderProps {
  firstUserSetupRequired: boolean
  onSignInClick: (event: MouseEvent<HTMLAnchorElement>) => void
}

function SignUpHeader({ firstUserSetupRequired, onSignInClick }: SignUpHeaderProps) {
  const t = useT()

  return (
    <div className="mb-6 flex flex-col items-center text-center">
      <div className={authIconBoxClass}>
        <img src={abiLogo} alt="" className="h-8.5 w-8.5 object-contain" />
      </div>
      <h1 className="text-foreground mb-1 text-2xl font-bold tracking-tight">
        {firstUserSetupRequired ? t('auth.first_setup_title') : t('auth.title_signup')}
      </h1>
      {firstUserSetupRequired ? (
        <p className="text-foreground-muted text-sm">{t('auth.first_setup_signup_body')}</p>
      ) : (
        <p className="text-foreground-muted text-sm">
          {t('auth.already_account')}{' '}
          <a href="/auth/signin" className={authInlineLinkClass} onClick={onSignInClick}>
            {t('auth.btn_signin')}
          </a>
        </p>
      )}
    </div>
  )
}

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
  const [success, setSuccess] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [passwordValid, setPasswordValid] = useState(false)
  const [termsOpen, setTermsOpen] = useState(false)
  const [privacyOpen, setPrivacyOpen] = useState(false)
  const [policyLoading, setPolicyLoading] = useState(true)
  const [signupAllowed, setSignupAllowed] = useState(true)
  const [firstUserSetupRequired, setFirstUserSetupRequired] = useState(false)

  useEffect(() => {
    let cancelled = false
    void apiGetPasswordPolicy()
      .then((policy) => {
        if (!cancelled) {
          setSignupAllowed(selfSignupEnabledFromPolicy(policy))
          setFirstUserSetupRequired(firstUserSetupRequiredFromPolicy(policy))
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

  const showPasswordMismatch = confirmPasswordMismatch(password, confirmPassword)
  const showEmailInvalid = email.length > 0 && !isValidEmailFormat(email)

  const clearPasswordMismatchError = useCallback(() => {
    setError((current) => (current === t('auth.passwords_dont_match') ? null : current))
  }, [t])

  const clearEmailFormatError = useCallback(() => {
    setError((current) => (current === t('auth.invalid_email') ? null : current))
  }, [t])

  const handleSignInClick = useCallback(
    (e: MouseEvent<HTMLAnchorElement>) => {
      e.preventDefault()
      void navigate('/auth/signin')
    },
    [navigate],
  )

  const handleSubmit = async (e: SubmitEvent<HTMLFormElement>) => {
    e.preventDefault()
    if (!displayName || !email || !password || !confirmPassword) {
      return
    }

    if (!isValidEmailFormat(email)) {
      setError(t('auth.invalid_email'))
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
    setSuccess(null)

    try {
      const result = await register(email, password, displayName)
      if (!result.authenticated) {
        setSuccess(t('auth.register_success'))
        setPassword('')
        setConfirmPassword('')
        setAgree(false)
        return
      }
      void navigate('/datasources')
    } catch (err: unknown) {
      setSuccess(null)
      setError(err instanceof Error ? err.message : 'Registration failed')
    } finally {
      setLoading(false)
    }
  }

  if (policyLoading) {
    return (
      <div className={authPageClass}>
        <div className={authCardClass}>
          <p className="text-foreground-muted text-sm" style={{ textAlign: 'center', margin: 0 }}>
            {t('common.loading')}
          </p>
        </div>
      </div>
    )
  }

  if (!signupAllowed) {
    return (
      <div className={authPageClass}>
        <div className={authCardClass}>
          <div className="mb-6 flex flex-col items-center text-center">
            <div className={authIconBoxClass}>
              <img src={abiLogo} alt="" className="h-8.5 w-8.5 object-contain" />
            </div>
            <h1 className="text-foreground mb-1 text-2xl font-bold tracking-tight">
              {t('auth.signup_closed_title')}
            </h1>
            <p className="text-foreground-muted text-sm">{t('auth.signup_closed_body')}</p>
            <p className="text-foreground-muted text-sm">{t('auth.signup_closed_contact')}</p>
          </div>
          <p className="text-foreground-muted text-sm" style={{ textAlign: 'center' }}>
            <a
              href="/auth/signin"
              className={authInlineLinkClass}
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
    <div className={authPageClass}>
      <div className={authCardClass}>
        <SignUpHeader
          firstUserSetupRequired={firstUserSetupRequired}
          onSignInClick={handleSignInClick}
        />

        <form
          onSubmit={(e) => {
            void handleSubmit(e)
          }}
          className={authFormClass}
        >
          {error && (
            <div
              className={legacyFeedbackClass(
                'border-error bg-error/8 text-error text-caption rounded border-l-[3px] p-[10px_12px]',
              )}
              role="alert"
              aria-live="assertive"
            >
              {error}
            </div>
          )}
          {success && (
            <div
              className={legacyFeedbackClass(
                'border-success bg-success/8 text-success text-caption rounded border-l-[3px] p-[10px_12px]',
              )}
              role="status"
              aria-live="polite"
            >
              {success}
            </div>
          )}

          <div className={authFieldClass}>
            <label className={authLabelClass} htmlFor="name-input">
              {t('auth.display_name')}
            </label>
            <input
              id="name-input"
              type="text"
              className={authInputClass}
              value={displayName}
              onChange={(e) => setDisplayName(e.target.value)}
              required
              disabled={loading}
              autoComplete="name"
            />
          </div>

          <EmailField
            email={email}
            onEmailChange={(value) => {
              setEmail(value)
              clearEmailFormatError()
            }}
            disabled={loading}
          />

          <div className={authFieldClass}>
            <label className={authLabelClass} htmlFor="password-input">
              {t('auth.password')}
            </label>
            <input
              id="password-input"
              type="password"
              className={authInputClass}
              value={password}
              onChange={(e) => {
                setPassword(e.target.value)
                clearPasswordMismatchError()
              }}
              required
              disabled={loading}
              autoComplete="new-password"
            />
            <PasswordStrengthMeter password={password} onValidityChange={handleValidity} />
          </div>

          <ConfirmPasswordField
            password={password}
            confirmPassword={confirmPassword}
            onConfirmChange={(value) => {
              setConfirmPassword(value)
              clearPasswordMismatchError()
            }}
            disabled={loading}
          />

          <label className={authCheckboxRowClass}>
            <input
              type="checkbox"
              className="accent-accent mt-0.5 shrink-0 cursor-pointer"
              checked={agree}
              onChange={(e) => setAgree(e.target.checked)}
              disabled={loading}
            />
            <span>
              {t('auth.agree_to')}{' '}
              <button type="button" className={authLinkBtnClass} onClick={() => setTermsOpen(true)}>
                {t('auth.terms_of_use')}
              </button>{' '}
              {t('auth.and')}{' '}
              <button
                type="button"
                className={authLinkBtnClass}
                onClick={() => setPrivacyOpen(true)}
              >
                {t('auth.privacy_policy')}
              </button>
              {t('auth.agree_suffix') && ` ${t('auth.agree_suffix')}`}
            </span>
          </label>

          <button
            type="submit"
            className={authSubmitBtnClass}
            disabled={isSubmitDisabled({
              loading,
              displayName,
              email,
              showEmailInvalid,
              password,
              confirmPassword,
              agree,
              showPasswordMismatch,
              passwordValid,
            })}
          >
            {loading && (
              <div className="h-4 w-4 animate-spin rounded-full border-2 border-white/30 border-t-white" />
            )}
            {firstUserSetupRequired ? t('auth.first_setup_submit') : t('auth.btn_signup')}
          </button>
        </form>
      </div>

      {/* Terms of Use Modal */}
      <Modal open={termsOpen} title={t('auth.terms_of_use')} onClose={() => setTermsOpen(false)}>
        {locale.startsWith('tr') ? <TermsOfUseTR /> : <TermsOfUseEN />}
        <div style={{ display: 'flex', justifyContent: 'flex-end', marginTop: '1.5rem' }}>
          <button
            type="button"
            className={legacyButtonClass('btn btn-primary')}
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
            className={legacyButtonClass('btn btn-primary')}
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
