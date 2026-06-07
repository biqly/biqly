import { type SubmitEvent, useCallback, useEffect, useRef, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'

import {
  apiChangePassword,
  apiGenerateMFABypassSelf,
  apiRequestEmailChange,
  apiUpdateProfile,
} from '../../api/auth'
import { useT } from '../../i18n'
import { useAuth } from '../auth/AuthProvider'
import PasswordStrengthMeter from '../auth/PasswordStrengthMeter'
import { AvatarCropModal } from './AvatarCropModal'

interface ProfileMessage {
  type: 'success' | 'error'
  text: string
}

export function AccountProfileSection() {
  const t = useT()
  const navigate = useNavigate()
  const { user, accessToken, roles, refreshUser, logout } = useAuth()
  const isSuperAdmin = roles.includes('super_admin')

  const [displayName, setDisplayName] = useState('')
  const [profileSaving, setProfileSaving] = useState(false)
  const [profileMessage, setProfileMessage] = useState<ProfileMessage | null>(null)

  const [cropImageSrc, setCropImageSrc] = useState<string | null>(null)
  const fileInputRef = useRef<HTMLInputElement>(null)

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
    if (!accessToken || profileSaving) {
      return
    }
    setProfileSaving(true)
    setProfileMessage(null)
    try {
      await apiUpdateProfile(accessToken, displayName.trim())
      await refreshUser()
      setProfileMessage({ type: 'success', text: t('settings.profile_saved') })
    } catch (err: unknown) {
      setProfileMessage({
        type: 'error',
        text: err instanceof Error ? err.message : t('common.error'),
      })
    } finally {
      setProfileSaving(false)
    }
  }

  const handleAvatarClick = () => {
    if (fileInputRef.current) {
      fileInputRef.current.value = ''
      fileInputRef.current.click()
    }
  }

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0]
    if (!file) {
      return
    }
    const reader = new FileReader()
    reader.onload = () => {
      if (typeof reader.result === 'string') {
        setCropImageSrc(reader.result)
      }
    }
    reader.readAsDataURL(file)
  }

  const handleCropSave = async (croppedBase64: string) => {
    setCropImageSrc(null)
    if (!accessToken || profileSaving) {
      return
    }
    setProfileSaving(true)
    setProfileMessage(null)
    try {
      await apiUpdateProfile(accessToken, displayName.trim(), croppedBase64)
      await refreshUser()
      setProfileMessage({ type: 'success', text: t('settings.profile_saved') })
    } catch (err: unknown) {
      setProfileMessage({
        type: 'error',
        text: err instanceof Error ? err.message : t('common.error'),
      })
    } finally {
      setProfileSaving(false)
    }
  }

  const handleAvatarRemove = async () => {
    if (!accessToken || profileSaving) {
      return
    }
    setProfileSaving(true)
    setProfileMessage(null)
    try {
      await apiUpdateProfile(accessToken, displayName.trim(), '')
      await refreshUser()
      setProfileMessage({ type: 'success', text: t('settings.profile_saved') })
    } catch (err: unknown) {
      setProfileMessage({
        type: 'error',
        text: err instanceof Error ? err.message : t('common.error'),
      })
    } finally {
      setProfileSaving(false)
    }
  }

  const handleEmailSubmit = async (e: SubmitEvent<HTMLFormElement>) => {
    e.preventDefault()
    if (!accessToken || emailSaving || !newEmail.trim()) {
      return
    }
    setEmailSaving(true)
    setEmailMessage(null)
    try {
      await apiRequestEmailChange(accessToken, newEmail.trim())
      setEmailMessage({ type: 'success', text: t('settings.profile_email_sent') })
      setNewEmail('')
    } catch (err: unknown) {
      setEmailMessage({
        type: 'error',
        text: err instanceof Error ? err.message : t('common.error'),
      })
    } finally {
      setEmailSaving(false)
    }
  }

  const handlePasswordSubmit = async (e: SubmitEvent<HTMLFormElement>) => {
    e.preventDefault()
    if (!accessToken || passwordSaving || !hasPassword) {
      return
    }
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
      setPasswordMessage({
        type: 'error',
        text: err instanceof Error ? err.message : t('common.error'),
      })
    } finally {
      setPasswordSaving(false)
    }
  }

  const handleGenerateBypass = async () => {
    if (!accessToken || bypassGenerating) {
      return
    }
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

  if (!user) {
    return null
  }

  const initials = (user.displayName ?? user.email).slice(0, 2).toUpperCase()

  return (
    <>
      <section
        className="card card--elevated settings-profile-card"
        aria-labelledby="settings-profile-heading"
      >
        <div className="settings-profile-card__hero">
          <div className="settings-profile-avatar-container">
            <button
              type="button"
              className="settings-profile-avatar-button"
              onClick={handleAvatarClick}
              title={t('settings.profile_picture_change')}
            >
              {user.avatarUrl ? (
                <img src={user.avatarUrl} alt="" className="settings-profile-avatar-img" />
              ) : (
                <span className="settings-profile-avatar-initials">{initials}</span>
              )}
              <div className="settings-profile-avatar-overlay">
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  width="16"
                  height="16"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2.5"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  className="lucide lucide-camera"
                >
                  <path d="M14.5 4h-5L7 7H4a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2V9a2 2 0 0 0-2-2h-3l-2.5-3z" />
                  <circle cx="12" cy="13" r="3" />
                </svg>
              </div>
            </button>
            {user.avatarUrl && (
              <button
                type="button"
                className="settings-profile-avatar-remove"
                onClick={() => {
                  void handleAvatarRemove()
                }}
                title={t('settings.profile_picture_remove')}
              >
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  width="10"
                  height="10"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2.5"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                  className="lucide lucide-trash-2"
                >
                  <path d="M3 6h18" />
                  <path d="M19 6v14c0 1-1 2-2 2H7c-1 0-2-1-2-2V6" />
                  <path d="M8 6V4c0-1 1-2 2-2h4c1 0 2 1 2 2v2" />
                  <line x1="10" x2="10" y1="11" y2="17" />
                  <line x1="14" x2="14" y1="11" y2="17" />
                </svg>
              </button>
            )}
            <input
              ref={fileInputRef}
              type="file"
              accept="image/*"
              style={{ display: 'none' }}
              onChange={handleFileChange}
            />
          </div>
          <div>
            <h2 id="settings-profile-heading" className="settings-profile-card__title">
              {t('settings.profile_section')}
            </h2>
            <p className="settings-profile-card__subtitle">{t('settings.profile_hint')}</p>
          </div>
        </div>

        <form
          onSubmit={(e) => {
            void handleProfileSubmit(e)
          }}
          className="settings-profile-block"
          style={{ borderTop: 'none', paddingTop: 0 }}
        >
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
            <input
              id="settings-email-readonly"
              type="email"
              value={user.email}
              readOnly
              disabled
              className="input-readonly"
            />
          </div>
          {profileMessage && (
            <p
              className={
                profileMessage.type === 'success'
                  ? 'settings-inline-success'
                  : 'settings-inline-error'
              }
            >
              {profileMessage.text}
            </p>
          )}
          <button
            type="submit"
            className="btn btn-primary btn-sm btn-auto-width"
            disabled={profileSaving}
          >
            {profileSaving ? '...' : t('settings.profile_save')}
          </button>
        </form>
      </section>

      <section
        className="card card--elevated settings-profile-card"
        aria-labelledby="settings-email-change-heading"
      >
        <form
          onSubmit={(e) => {
            void handleEmailSubmit(e)
          }}
          className="settings-profile-block"
          style={{ borderTop: 'none', paddingTop: 0 }}
        >
          <h3 id="settings-email-change-heading" className="settings-profile-block__title">
            {t('settings.profile_email_change_title')}
          </h3>
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
            <p
              className={
                emailMessage.type === 'success'
                  ? 'settings-inline-success'
                  : 'settings-inline-error'
              }
            >
              {emailMessage.text}
            </p>
          )}
          <button
            type="submit"
            className="btn btn-secondary btn-sm btn-auto-width"
            disabled={emailSaving || !newEmail.trim()}
          >
            {emailSaving ? '...' : t('settings.profile_email_request')}
          </button>
        </form>
      </section>

      <section
        className="card card--elevated settings-profile-card"
        aria-labelledby="settings-password-heading"
      >
        <form
          onSubmit={(e) => {
            void handlePasswordSubmit(e)
          }}
          className="settings-profile-block"
          style={{ borderTop: 'none', paddingTop: 0 }}
        >
          <h3 id="settings-password-heading" className="settings-profile-block__title">
            {t('settings.profile_password_title')}
          </h3>
          {!hasPassword ? (
            <p className="settings-profile-block__hint">{t('settings.profile_no_password')}</p>
          ) : (
            <>
              <div className="form-group">
                <label htmlFor="settings-current-password">
                  {t('settings.profile_current_password')}
                </label>
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
                <label htmlFor="settings-confirm-password">
                  {t('settings.profile_confirm_password')}
                </label>
                <input
                  id="settings-confirm-password"
                  type="password"
                  value={confirmPassword}
                  onChange={(e) => setConfirmPassword(e.target.value)}
                  autoComplete="new-password"
                />
              </div>
              {passwordMessage && (
                <p
                  className={
                    passwordMessage.type === 'success'
                      ? 'settings-inline-success'
                      : 'settings-inline-error'
                  }
                >
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
                <Link
                  to="/auth/forgot-password"
                  className="settings-profile-link"
                  onClick={(e) => {
                    e.preventDefault()
                    void (async () => {
                      await logout()
                      void navigate('/auth/forgot-password')
                    })()
                  }}
                >
                  {t('settings.profile_forgot_password')}
                </Link>
              </div>
            </>
          )}
        </form>
      </section>

      {isSuperAdmin && (
        <section
          className="card card--elevated settings-profile-card settings-profile-card--support-card"
          aria-labelledby="settings-support-heading"
        >
          <div className="settings-profile-block" style={{ borderTop: 'none', paddingTop: 0 }}>
            <h3 id="settings-support-heading" className="settings-profile-block__title">
              {t('settings.profile_mfa_bypass_title')}
            </h3>
            <p className="settings-profile-block__hint">{t('settings.profile_mfa_bypass_hint')}</p>
            <button
              type="button"
              className="btn btn-secondary btn-sm btn-auto-width"
              onClick={() => {
                void handleGenerateBypass()
              }}
              disabled={bypassGenerating}
              style={{ display: 'inline-flex', alignItems: 'center', gap: '6px' }}
            >
              <svg
                xmlns="http://www.w3.org/2000/svg"
                width="14"
                height="14"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
                strokeLinecap="round"
                strokeLinejoin="round"
                className="lucide lucide-shield-alert"
              >
                <path d="M12 22s8-4 8-10V5l-8-3-8 3v7c0 6 8 10 8 10z" />
                <line x1="12" x2="12" y1="9" y2="13" />
                <line x1="12" x2="12.01" y1="17" y2="17" />
              </svg>
              {t('settings.profile_mfa_bypass_btn')}
            </button>
            {bypassError && <p className="settings-inline-error">{bypassError}</p>}
            {bypassCode && (
              <div className="settings-bypass-code">
                <code>{bypassCode}</code>
                <button
                  type="button"
                  className="btn btn-secondary btn-sm btn-auto-width"
                  style={{ display: 'inline-flex', alignItems: 'center', gap: '6px' }}
                  onClick={() => {
                    void navigator.clipboard.writeText(bypassCode)
                  }}
                >
                  <svg
                    xmlns="http://www.w3.org/2000/svg"
                    width="14"
                    height="14"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    className="lucide lucide-copy"
                  >
                    <rect width="14" height="14" x="8" y="8" rx="2" ry="2" />
                    <path d="M4 16c-1.1 0-2-.9-2-2V4c0-1.1.9-2 2-2h10c1.1 0 2 .9 2 2" />
                  </svg>
                  {t('admin.user_detail.copy')}
                </button>
              </div>
            )}
          </div>
        </section>
      )}

      {cropImageSrc && (
        <AvatarCropModal
          imageSrc={cropImageSrc}
          onClose={() => setCropImageSrc(null)}
          onSave={handleCropSave}
        />
      )}
    </>
  )
}
