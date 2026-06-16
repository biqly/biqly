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
      className="pointer-events-none fixed right-5 bottom-5 z-1100 flex w-[min(100%,22rem)] max-w-[calc(100vw-2rem)] flex-col gap-[0.6rem] max-[520px]:right-4 max-[520px]:bottom-4 max-[520px]:left-4 max-[520px]:w-auto"
      role="region"
      aria-label={t('common.notifications')}
    >
      {toasts.map((toast) => (
        <div
          key={toast.id}
          className={clsx(
            'border-border-strong bg-card text-foreground pointer-events-auto grid grid-cols-[auto_1fr_auto] items-start gap-[0.65rem] rounded-[0.6rem] border border-l-3 px-[0.85rem] py-3 shadow-[0_10px_32px_rgba(0,0,0,0.4)] motion-safe:animate-[toast-in_180ms_ease]',
            VARIANT_BORDER[toast.variant],
          )}
          role={toast.variant === 'error' ? 'alert' : 'status'}
          aria-live={toast.variant === 'error' ? 'assertive' : 'polite'}
        >
          <span
            className={clsx(
              'inline-grid h-[1.4rem] w-[1.4rem] place-items-center rounded-full text-[0.8rem] leading-none font-bold text-white',
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
              <p className="text-foreground-muted mx-0 mt-[0.15rem] mb-0 text-[0.85rem] leading-[1.45] wrap-anywhere">
                {toast.message}
              </p>
            )}
          </div>
          <button
            type="button"
            className={legacyCardClass(
              'text-foreground-muted hover:bg-card-raised hover:text-foreground inline-grid h-6 w-6 cursor-pointer place-items-center rounded-[0.35rem] border-0 bg-transparent text-[0.85rem] leading-none',
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
