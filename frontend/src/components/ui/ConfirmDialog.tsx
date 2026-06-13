import type { ReactNode } from 'react'

import { useAutofocus } from '../../hooks/useAutofocus'
import { useT } from '../../i18n'
import { buttonClass, legacyButtonClass } from '../../lib/buttonClasses'
import { confirmDialogActionsClass, confirmDialogMessageClass } from '../../lib/modalClasses'
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
      <div>
        {message && <p className={confirmDialogMessageClass}>{message}</p>}
        <div className={confirmDialogActionsClass}>
          <button
            type="button"
            className={legacyButtonClass('btn btn-secondary')}
            onClick={onCancel}
          >
            {cancelText}
          </button>
          <button
            type="button"
            ref={confirmButtonRef}
            className={buttonClass(
              variant === 'danger'
                ? 'danger'
                : variant === 'warning'
                  ? 'danger-outline'
                  : 'primary',
            )}
            onClick={onConfirm}
          >
            {confirmText}
          </button>
        </div>
      </div>
    </Modal>
  )
}
