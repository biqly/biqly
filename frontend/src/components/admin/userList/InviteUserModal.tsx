import React, { useState } from 'react'
import { apiInviteUser } from '../../../api/auth'

import type { TranslationKey } from '../../../i18n'

const backdropStyle: React.CSSProperties = {
  position: 'fixed',
  top: 0,
  left: 0,
  right: 0,
  bottom: 0,
  backgroundColor: 'rgba(0,0,0,0.5)',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  zIndex: 1000,
}
const cardStyle: React.CSSProperties = {
  background: 'var(--bg-card, #ffffff)',
  border: '1px solid var(--border, rgba(255, 255, 255, 0.06))',
  borderRadius: 8,
  width: '100%',
  maxWidth: 400,
  padding: 24,
  boxShadow: '0 20px 25px -5px rgba(0,0,0,0.1), 0 10px 10px -5px rgba(0,0,0,0.04)',
}
const headerStyle: React.CSSProperties = {
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'center',
  marginBottom: 16,
}
const titleStyle: React.CSSProperties = { margin: 0, fontSize: 18 }
const closeBtnStyle: React.CSSProperties = {
  border: 0,
  background: 'transparent',
  fontSize: 20,
  cursor: 'pointer',
  color: 'var(--text-secondary)',
}
const labelStyle: React.CSSProperties = { fontSize: 13, fontWeight: 500 }
const gap16: React.CSSProperties = { gap: 16 }
const gap6: React.CSSProperties = { gap: 6 }
const footerStyle: React.CSSProperties = { display: 'flex', justifyContent: 'flex-end', gap: 12, marginTop: 8 }
const spinnerStyle: React.CSSProperties = { marginRight: 6, display: 'inline-block', width: 12, height: 12 }

interface InviteUserModalProps {
  open: boolean
  onClose: () => void
  token: string
  onSuccess: () => void
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
}

export function InviteUserModal({
  open,
  onClose,
  token,
  onSuccess,
  t,
}: InviteUserModalProps) {
  const [inviteEmail, setInviteEmail] = useState('')
  const [inviteRole, setInviteRole] = useState('viewer')
  const [inviteLoading, setInviteLoading] = useState(false)
  const [inviteSuccess, setInviteSuccess] = useState(false)
  const [inviteError, setInviteError] = useState<string | null>(null)

  if (!open) return null

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setInviteLoading(true)
    setInviteError(null)
    try {
      await apiInviteUser(token, inviteEmail, inviteRole)
      setInviteSuccess(true)
      onSuccess()
    } catch (err: unknown) {
      setInviteError(err instanceof Error ? err.message : 'Invitation failed')
    } finally {
      setInviteLoading(false)
    }
  }

  return (
    <div
      className="modal-backdrop"
      role="presentation"
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose()
      }}
      style={backdropStyle}
    >
      <section
        className="modal-card"
        role="dialog"
        aria-modal="true"
        style={cardStyle}
      >
        <header style={headerStyle}>
          <h2 style={titleStyle}>{t('auth.invite_user_modal_title')}</h2>
          <button
            type="button"
            onClick={onClose}
            style={closeBtnStyle}
          >
            ×
          </button>
        </header>

        {inviteSuccess ? (
          <div className="page-stack" style={gap16}>
            <div className="admin-success-box">
              {t('auth.invite_user_success', { email: inviteEmail })}
            </div>
            <button
              type="button"
              className="admin-btn-primary"
              onClick={onClose}
            >
              {t('common.close')}
            </button>
          </div>
        ) : (
          <form onSubmit={handleSubmit} className="page-stack" style={gap16}>
            {inviteError && (
              <div className="admin-err-box">
                {t('auth.invite_user_failed', { error: inviteError })}
              </div>
            )}

            <div className="page-stack" style={gap6}>
              <label style={labelStyle} htmlFor="invite-email-input">
                {t('auth.invite_user_email')}
              </label>
              <input
                id="invite-email-input"
                type="email"
                required
                value={inviteEmail}
                onChange={(e) => setInviteEmail(e.target.value)}
                className="admin-input-wide"
              />
            </div>

            <div className="page-stack" style={gap6}>
              <label style={labelStyle} htmlFor="invite-role-input">
                {t('auth.invite_user_role')}
              </label>
              <select
                id="invite-role-input"
                value={inviteRole}
                onChange={(e) => setInviteRole(e.target.value)}
                className="admin-select-wide"
              >
                <option value="viewer">Viewer</option>
                <option value="analyst">Analyst</option>
                <option value="developer">Developer</option>
                <option value="admin">Admin</option>
                <option value="super_admin">Super Admin</option>
              </select>
            </div>

            <div style={footerStyle}>
              <button
                type="button"
                className="admin-btn-ghost"
                onClick={onClose}
                disabled={inviteLoading}
              >
                {t('common.cancel')}
              </button>
              <button
                type="submit"
                className="admin-btn-primary"
                disabled={inviteLoading || !inviteEmail}
              >
                {inviteLoading && (
                  <span
                    className="spinner"
                    style={spinnerStyle}
                  />
                )}
                {t('auth.btn_invite_user')}
              </button>
            </div>
          </form>
        )}
      </section>
    </div>
  )
}
