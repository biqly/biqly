import clsx from 'clsx'
import { useT } from '../../i18n'
import type { ToastItem } from '../../hooks/useToast'
import '../../styles/toast.css'

const VARIANT_ICON: Record<ToastItem['variant'], string> = {
  success: '✓',
  error: '✕',
  info: 'i',
  warning: '!',
}

interface ToastViewportProps {
  toasts: ToastItem[]
  onDismiss: (id: number) => void
}

export function ToastViewport({ toasts, onDismiss }: ToastViewportProps) {
  const t = useT()
  if (toasts.length === 0) return null
  return (
    <div className="toast-viewport" role="region" aria-label={t('common.notifications')}>
      {toasts.map((toast) => (
        <div
          key={toast.id}
          className={clsx('toast', `toast--${toast.variant}`)}
          role={toast.variant === 'error' ? 'alert' : 'status'}
          aria-live={toast.variant === 'error' ? 'assertive' : 'polite'}
        >
          <span className="toast__icon" aria-hidden="true">
            {VARIANT_ICON[toast.variant]}
          </span>
          <div className="toast__body">
            {toast.title && <p className="toast__title">{toast.title}</p>}
            {toast.message && <p className="toast__message">{toast.message}</p>}
          </div>
          <button
            type="button"
            className="toast__close"
            onClick={() => onDismiss(toast.id)}
            aria-label={t('common.close')}
          >
            ✕
          </button>
        </div>
      ))}
    </div>
  )
}
