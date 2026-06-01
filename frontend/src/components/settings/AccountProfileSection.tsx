import { useCallback, useEffect, useState, type SubmitEvent } from 'react'
import { Link } from 'react-router-dom'
import { useT } from '../../i18n'
import { useAuth } from '../auth/AuthProvider'
import {
  apiChangePassword,
  apiGenerateMFABypassSelf,
  apiRequestEmailChange,
  apiUpdateProfile,
} from '../../api/auth'
import PasswordStrengthMeter from '../auth/PasswordStrengthMeter'

type ProfileMessage = { type: 'success' | 'error'; text: string }

export function AccountProfileSection() {
  const t = useT()
  const { user, accessToken, roles, refreshUser } = useAuth()
  const isSuperAdmin = roles.includes('super_admin')

  const [displayName, setDisplayName] = useState('')
  const [profileSaving, setProfileSaving] = useState(false)
  const [profileMessage, setProfileMessage] = useState<ProfileMessage | null>(null)

  const [newEmail, setNewEmail] = useState('')
  const [emailSaving, setEmailSaving] = useState(false)
  const [emailMessage, setEmailMessage] = useState<ProfileMessage | null>(null)

  const [currentPassword, setCurrentPassword] = useState('')
  const [newPassword, setNewPassword] = useState('')
  const [confirmPassword, setConfirmPassword] = useState('')
  const [passwordValid, setPasswordValid] = useState(false)
  const [passwordSaving, setPasswordSaving] = useState(false)
  const [passwordMessage, setPasswordMessage] = useState<ProfileMessage | null>(null)

  const [bypassGenerating, setBypassGenerating] = useState(false)
  const [bypassCode, setBypassCode] = useState<string | null>(null)
  const [bypassError, setBypassError] = useState<string | null>(null)

  const hasPassword = user?.hasPassword !== false

  useEffect(() => {
    setDisplayName(user?.displayName ?? '')
  }, [user?.displayName])

  const handleValidity = useCallback((info: { valid: boolean }) => {
    setPasswordValid(info.valid)
  }, [])

  const handleProfileSubmit = async (e: SubmitEvent<HTMLFormElement>) => {
    e.preventDefault()
    if (!accessToken || profileSaving) return
    setProfileSaving(true)
    setProfileMessage(null)
    try {
      await apiUpdateProfile(accessToken, displayName.trim())
      await refreshUser()
      setProfileMessage({ type: 'success', text: t('settings.profile_saved') })
    } catch (err: unknown) {
      setProfileMessage({ type: 'error', text: err instanceof Error ? err.message : t('common.error') })
    } finally {
      setProfileSaving(false)
    }
  }

  const handleEmailSubmit = async (e: SubmitEvent<HTMLFormElement>) => {
    e.preventDefault()
    if (!accessToken || emailSaving || !newEmail.trim()) return
    setEmailSaving(true)
    setEmailMessage(null)
    try {
      await apiRequestEmailChange(accessToken, newEmail.trim())
      setEmailMessage({ type: 'success', text: t('settings.profile_email_sent') })
      setNewEmail('')
    } catch (err: unknown) {
      setEmailMessage({ type: 'error', text: err instanceof Error ? err.message : t('common.error') })
    } finally {
      setEmailSaving(false)
    }
  }

  const handlePasswordSubmit = async (e: SubmitEvent<HTMLFormElement>) => {
    e.preventDefault()
    if (!accessToken || passwordSaving || !hasPassword) return
    if (newPassword !== confirmPassword) {
      setPasswordMessage({ type: 'error', text: t('auth.passwords_dont_match') })
      return
    }
    if (!passwordValid) {
      setPasswordMessage({ type: 'error', text: t('auth.password_requirements_failed') })
      return
    }
    setPasswordSaving(true)
    setPasswordMessage(null)
    try {
      await apiChangePassword(accessToken, currentPassword, newPassword)
      setCurrentPassword('')
      setNewPassword('')
      setConfirmPassword('')
      setPasswordMessage({ type: 'success', text: t('settings.profile_password_saved') })
    } catch (err: unknown) {
      setPasswordMessage({ type: 'error', text: err instanceof Error ? err.message : t('common.error') })
    } finally {
      setPasswordSaving(false)
    }
  }

  const handleGenerateBypass = async () => {
    if (!accessToken || bypassGenerating) return
    setBypassGenerating(true)
    setBypassError(null)
    setBypassCode(null)
    try {
      const resp = await apiGenerateMFABypassSelf(accessToken)
      setBypassCode(resp.bypass_code)
    } catch (err: unknown) {
      setBypassError(err instanceof Error ? err.message : t('common.error'))
    } finally {
      setBypassGenerating(false)
    }
  }

  if (!user) return null

  const initials = (user.displayName || user.email).slice(0, 2).toUpperCase()

  return (
    <section className="card card--elevated settings-profile-card" aria-labelledby="settings-profile-heading">
      <div className="settings-profile-card__hero">
        <div className="settings-profile-avatar" aria-hidden>
          {initials}
        </div>
        <div>
          <h2 id="settings-profile-heading" className="settings-profile-card__title">
            {t('settings.profile_section')}
          </h2>
          <p className="settings-profile-card__subtitle">{t('settings.profile_hint')}</p>
        </div>
      </div>

      <form onSubmit={handleProfileSubmit} className="settings-profile-block">
        <h3 className="settings-profile-block__title">{t('settings.profile_name_title')}</h3>
        <div className="form-group">
          <label htmlFor="settings-display-name">{t('settings.profile_display_name')}</label>
          <input
            id="settings-display-name"
            type="text"
            value={displayName}
            onChange={(e) => setDisplayName(e.target.value)}
            autoComplete="name"
            maxLength={120}
          />
        </div>
        <div className="form-group">
          <label htmlFor="settings-email-readonly">{t('settings.profile_email')}</label>
          <input id="settings-email-readonly" type="email" value={user.email} readOnly disabled className="input-readonly" />
        </div>
        {profileMessage && (
          <p className={profileMessage.type === 'success' ? 'settings-inline-success' : 'settings-inline-error'}>
            {profileMessage.text}
          </p>
        )}
        <button type="submit" className="btn btn-primary btn-sm btn-auto-width" disabled={profileSaving}>
          {profileSaving ? '...' : t('settings.profile_save')}
        </button>
      </form>

      <form onSubmit={handleEmailSubmit} className="settings-profile-block">
        <h3 className="settings-profile-block__title">{t('settings.profile_email_change_title')}</h3>
        <p className="settings-profile-block__hint">{t('settings.profile_email_change_hint')}</p>
        <div className="form-group">
          <label htmlFor="settings-new-email">{t('settings.profile_new_email')}</label>
          <input
            id="settings-new-email"
            type="email"
            value={newEmail}
            onChange={(e) => setNewEmail(e.target.value)}
            autoComplete="email"
            placeholder={t('settings.profile_new_email_placeholder')}
          />
        </div>
        {emailMessage && (
          <p className={emailMessage.type === 'success' ? 'settings-inline-success' : 'settings-inline-error'}>
            {emailMessage.text}
          </p>
        )}
        <button type="submit" className="btn btn-secondary btn-sm btn-auto-width" disabled={emailSaving || !newEmail.trim()}>
          {emailSaving ? '...' : t('settings.profile_email_request')}
        </button>
      </form>

      <form onSubmit={handlePasswordSubmit} className="settings-profile-block">
        <h3 className="settings-profile-block__title">{t('settings.profile_password_title')}</h3>
        {!hasPassword ? (
          <p className="settings-profile-block__hint">{t('settings.profile_no_password')}</p>
        ) : (
          <>
            <div className="form-group">
              <label htmlFor="settings-current-password">{t('settings.profile_current_password')}</label>
              <input
                id="settings-current-password"
                type="password"
                value={currentPassword}
                onChange={(e) => setCurrentPassword(e.target.value)}
                autoComplete="current-password"
              />
            </div>
            <div className="form-group">
              <label htmlFor="settings-new-password">{t('settings.profile_new_password')}</label>
              <input
                id="settings-new-password"
                type="password"
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                autoComplete="new-password"
              />
              <PasswordStrengthMeter password={newPassword} onValidityChange={handleValidity} />
            </div>
            <div className="form-group">
              <label htmlFor="settings-confirm-password">{t('settings.profile_confirm_password')}</label>
              <input
                id="settings-confirm-password"
                type="password"
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                autoComplete="new-password"
              />
            </div>
            {passwordMessage && (
              <p className={passwordMessage.type === 'success' ? 'settings-inline-success' : 'settings-inline-error'}>
                {passwordMessage.text}
              </p>
            )}
            <div className="settings-profile-actions">
              <button
                type="submit"
                className="btn btn-primary btn-sm btn-auto-width"
                disabled={passwordSaving || !currentPassword || !newPassword || !confirmPassword}
              >
                {passwordSaving ? '...' : t('settings.profile_password_save')}
              </button>
              <Link to="/auth/forgot-password" className="settings-profile-link">
                {t('settings.profile_forgot_password')}
              </Link>
            </div>
          </>
        )}
      </form>

      {isSuperAdmin && (
        <div className="settings-profile-block settings-profile-block--support">
          <h3 className="settings-profile-block__title">{t('settings.profile_mfa_bypass_title')}</h3>
          <p className="settings-profile-block__hint">{t('settings.profile_mfa_bypass_hint')}</p>
          <button
            type="button"
            className="btn btn-secondary btn-sm btn-auto-width"
            onClick={handleGenerateBypass}
            disabled={bypassGenerating}
          >
            {bypassGenerating ? '...' : t('settings.profile_mfa_bypass_btn')}
          </button>
          {bypassError && <p className="settings-inline-error">{bypassError}</p>}
          {bypassCode && (
            <div className="settings-bypass-code">
              <code>{bypassCode}</code>
              <button
                type="button"
                className="btn btn-secondary btn-sm"
                onClick={() => {
                  void navigator.clipboard.writeText(bypassCode)
                }}
              >
                {t('admin.user_detail.copy')}
              </button>
            </div>
          )}
        </div>
      )}
    </section>
  )
}
