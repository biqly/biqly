import { useState } from 'react'

import { apiInviteUser } from '../../../api/auth'
import { useT } from '../../../i18n'
import { cn } from '../../../lib/cn'
import { legacyLayoutClass } from '../../../lib/layoutClasses'
import { modalInviteUserCardClass } from '../../../lib/modalClasses'
import { Modal } from '../../ui/Modal'
import { Select } from '../../ui/Select'
import {
  adminBtnGhostClass,
  adminBtnPrimaryClass,
  adminErrBoxClass,
  adminFormLabelClass,
  adminInputWideClass,
  adminSuccessBoxClass,
} from '../adminClasses'
import { securityRoleOptions } from '../adminSelectOptions'

const labelStyle: React.CSSProperties = { fontSize: 13, fontWeight: 500 }
const gap16: React.CSSProperties = { gap: 16 }
const gap6: React.CSSProperties = { gap: 6 }
const footerStyle: React.CSSProperties = {
  display: 'flex',
  justifyContent: 'flex-end',
  gap: 12,
  marginTop: 8,
}
const spinnerStyle: React.CSSProperties = {
  marginRight: 6,
  display: 'inline-block',
  width: 12,
  height: 12,
}

interface InviteUserModalProps {
  open: boolean
  onClose: () => void
  token: string
  onSuccess: () => void
}

export function InviteUserModal({ open, onClose, token, onSuccess }: InviteUserModalProps) {
  const t = useT()
  const [inviteEmail, setInviteEmail] = useState('')
  const [inviteRole, setInviteRole] = useState('viewer')
  const [inviteLoading, setInviteLoading] = useState(false)
  const [inviteSuccess, setInviteSuccess] = useState(false)
  const [inviteError, setInviteError] = useState<string | null>(null)

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
    <Modal
      open={open}
      title={t('auth.invite_user_modal_title')}
      onClose={onClose}
      labelledBy="invite-user-modal-title"
      className={modalInviteUserCardClass()}
    >
      {inviteSuccess ? (
        <div className={legacyLayoutClass('page-stack')} style={gap16}>
          <div className={adminSuccessBoxClass}>
            {t('auth.invite_user_success', { email: inviteEmail })}
          </div>
          <button type="button" className={adminBtnPrimaryClass} onClick={onClose}>
            {t('common.close')}
          </button>
        </div>
      ) : (
        <form
          onSubmit={(e) => {
            void handleSubmit(e)
          }}
          className={legacyLayoutClass('page-stack')}
          style={gap16}
        >
          {inviteError && (
            <div className={adminErrBoxClass}>
              {t('auth.invite_user_failed', { error: inviteError })}
            </div>
          )}

          <div className={legacyLayoutClass('page-stack')} style={gap6}>
            <label style={labelStyle} htmlFor="invite-email-input">
              {t('auth.invite_user_email')}
            </label>
            <input
              id="invite-email-input"
              type="email"
              required
              value={inviteEmail}
              onChange={(e) => setInviteEmail(e.target.value)}
              className={adminInputWideClass}
            />
          </div>

          <div className={cn(legacyLayoutClass('page-stack'), adminFormLabelClass)} style={gap6}>
            <label style={labelStyle} htmlFor="invite-role-input">
              {t('auth.invite_user_role')}
            </label>
            <Select
              id="invite-role-input"
              value={inviteRole}
              options={securityRoleOptions()}
              onChange={setInviteRole}
            />
          </div>

          <div style={footerStyle}>
            <button
              type="button"
              className={adminBtnGhostClass}
              onClick={onClose}
              disabled={inviteLoading}
            >
              {t('common.cancel')}
            </button>
            <button
              type="submit"
              className={adminBtnPrimaryClass}
              disabled={inviteLoading || !inviteEmail}
            >
              {inviteLoading && <span className="spinner" style={spinnerStyle} />}
              {t('auth.btn_invite_user')}
            </button>
          </div>
        </form>
      )}
    </Modal>
  )
}
