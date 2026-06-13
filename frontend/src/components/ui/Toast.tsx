import clsx from 'clsx'

import type { ToastItem } from '../../hooks/useToast'
import { useT } from '../../i18n'
import { legacyCardClass } from '../../lib/cardClasses'

const VARIANT_ICON: Record<ToastItem['variant'], string> = {
  success: '✓',
  error: '✕',
  info: 'i',
  warning: '!',
}

const VARIANT_BORDER: Record<ToastItem['variant'], string> = {
  success: 'border-l-success',
  error: 'border-l-error',
  warning: 'border-l-warning',
  info: 'border-l-accent',
}

const VARIANT_BG: Record<ToastItem['variant'], string> = {
  success: 'bg-success',
  error: 'bg-error',
  warning: 'bg-warning',
  info: 'bg-accent',
}

interface ToastViewportProps {
  toasts: ToastItem[]
  onDismiss: (id: number) => void
}

export function ToastViewport({ toasts, onDismiss }: ToastViewportProps) {
  const t = useT()
  if (toasts.length === 0) {
    return null
  }
  return (
    <div
      className="fixed bottom-5 right-5 z-[1100] flex flex-col gap-[0.6rem] w-[min(100%,22rem)] max-w-[calc(100vw-2rem)] pointer-events-none max-[520px]:left-4 max-[520px]:right-4 max-[520px]:bottom-4 max-[520px]:w-auto"
      role="region"
      aria-label={t('common.notifications')}
    >
      {toasts.map((toast) => (
        <div
          key={toast.id}
          className={clsx(
            'grid grid-cols-[auto_1fr_auto] items-start gap-[0.65rem] border border-border-strong border-l-3 rounded-[0.6rem] bg-card shadow-[0_10px_32px_rgba(0,0,0,0.4)] text-foreground py-3 px-[0.85rem] pointer-events-auto motion-safe:animate-[toast-in_180ms_ease]',
            VARIANT_BORDER[toast.variant],
          )}
          role={toast.variant === 'error' ? 'alert' : 'status'}
          aria-live={toast.variant === 'error' ? 'assertive' : 'polite'}
        >
          <span
            className={clsx(
              'inline-grid w-[1.4rem] h-[1.4rem] place-items-center rounded-full text-[0.8rem] font-bold leading-none text-white',
              VARIANT_BG[toast.variant],
            )}
            aria-hidden="true"
          >
            {VARIANT_ICON[toast.variant]}
          </span>
          <div className="min-w-0">
            {toast.title && (
              <p className="m-0 text-[0.9rem] font-semibold tracking-[-0.01em]">{toast.title}</p>
            )}
            {toast.message && (
              <p className="mt-[0.15rem] mb-0 mx-0 text-[0.85rem] leading-[1.45] text-foreground-muted [overflow-wrap:anywhere]">
                {toast.message}
              </p>
            )}
          </div>
          <button
            type="button"
            className={legacyCardClass(
              'inline-grid w-6 h-6 place-items-center border-0 rounded-[0.35rem] bg-transparent text-foreground-muted cursor-pointer text-[0.85rem] leading-none hover:bg-card-raised hover:text-foreground',
            )}
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
