import type { ReactNode } from 'react'

import { useAutofocus } from '../../hooks/useAutofocus'
import { useT } from '../../i18n'
import { Modal } from './Modal'

interface ConfirmDialogProps {
  open: boolean
  title: ReactNode
  message?: ReactNode
  confirmLabel?: string
  cancelLabel?: string
  variant?: 'danger' | 'warning' | 'default'
  onConfirm: () => void
  onCancel: () => void
}

export function ConfirmDialog({
  open,
  title,
  message,
  confirmLabel,
  cancelLabel,
  variant = 'danger',
  onConfirm,
  onCancel,
}: ConfirmDialogProps) {
  const t = useT()
  const confirmButtonRef = useAutofocus<HTMLButtonElement>(open)

  const confirmText = confirmLabel ?? t('common.confirm_ok')
  const cancelText = cancelLabel ?? t('common.confirm_cancel')

  return (
    <Modal open={open} title={title} onClose={onCancel} closeOnBackdrop={false}>
      <div className="confirm-dialog">
        {message && <p className="confirm-dialog__message">{message}</p>}
        <div className="confirm-dialog__actions">
          <button type="button" className="btn btn-secondary" onClick={onCancel}>
            {cancelText}
          </button>
          <button
            type="button"
            ref={confirmButtonRef}
            className={`btn ${variant === 'danger' ? 'btn-danger' : variant === 'warning' ? 'btn-danger-outline' : 'btn-primary'}`}
            onClick={onConfirm}
          >
            {confirmText}
          </button>
        </div>
      </div>
    </Modal>
  )
}
