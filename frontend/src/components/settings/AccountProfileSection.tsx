import { type SubmitEvent, useCallback, useEffect, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import {
  apiChangePassword,
  apiGenerateMFABypassSelf,
  apiRequestEmailChange,
  apiUpdateProfile,
} from '../../api/auth'
import { useT } from '../../i18n'
import { useAuth } from '../auth/AuthProvider'
import {
  AccountEmailChangeSection,
  AccountMfaBypassSection,
  AccountPasswordSection,
  AccountProfileHero,
} from './AccountProfileSections'
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
    // eslint-disable-next-line react-hooks/set-state-in-effect
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

  return (
    <>
      <AccountProfileHero
        t={t}
        user={user}
        displayName={displayName}
        profileSaving={profileSaving}
        profileMessage={profileMessage}
        onDisplayNameChange={setDisplayName}
        onProfileSubmit={(event) => {
          void handleProfileSubmit(event)
        }}
        onAvatarClick={handleAvatarClick}
        onAvatarRemove={() => void handleAvatarRemove()}
        fileInputRef={fileInputRef}
        onFileChange={handleFileChange}
      />
      <AccountEmailChangeSection
        t={t}
        newEmail={newEmail}
        emailSaving={emailSaving}
        emailMessage={emailMessage}
        onNewEmailChange={setNewEmail}
        onSubmit={(event) => {
          void handleEmailSubmit(event)
        }}
      />
      <AccountPasswordSection
        t={t}
        hasPassword={hasPassword}
        currentPassword={currentPassword}
        newPassword={newPassword}
        confirmPassword={confirmPassword}
        passwordSaving={passwordSaving}
        passwordMessage={passwordMessage}
        onCurrentPasswordChange={setCurrentPassword}
        onNewPasswordChange={setNewPassword}
        onConfirmPasswordChange={setConfirmPassword}
        onValidityChange={handleValidity}
        onSubmit={(event) => {
          void handlePasswordSubmit(event)
        }}
        onForgotPassword={() => {
          void logout()
          void navigate('/auth/forgot-password')
        }}
      />
      {isSuperAdmin && (
        <AccountMfaBypassSection
          t={t}
          bypassGenerating={bypassGenerating}
          bypassCode={bypassCode}
          bypassError={bypassError}
          onGenerate={() => void handleGenerateBypass()}
        />
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
