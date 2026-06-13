import clsx from 'clsx'
import { type ReactNode, useEffect, useRef } from 'react'

import { useT } from '../../i18n'

interface ModalProps {
  open: boolean
  title: ReactNode
  subtitle?: ReactNode
  children: ReactNode
  onClose: () => void
  labelledBy?: string
  className?: string
  bodyClassName?: string
  closeOnBackdrop?: boolean
}

const FOCUSABLE_SELECTOR = [
  'a[href]',
  'button:not([disabled])',
  'textarea:not([disabled])',
  'input:not([disabled])',
  'select:not([disabled])',
  '[tabindex]:not([tabindex="-1"])',
].join(',')

export function Modal({
  open,
  title,
  subtitle,
  children,
  onClose,
  labelledBy = 'modal-title',
  className,
  bodyClassName,
  closeOnBackdrop = true,
}: ModalProps) {
  const t = useT()
  const dialogRef = useRef<HTMLElement>(null)

  useEffect(() => {
    if (!open) {
      return
    }
    const previousFocus =
      document.activeElement instanceof HTMLElement ? document.activeElement : null
    const dialog = dialogRef.current

    // Prioritize [autoFocus] or elements in .modal-body over the close button in the header
    const autoFocusEl = dialog?.querySelector<HTMLElement>('[autofocus], [autoFocus]')
    if (autoFocusEl) {
      autoFocusEl.focus()
    } else {
      const bodySelector = FOCUSABLE_SELECTOR.split(',')
        .map((s) => `.modal-body ${s.trim()}`)
        .join(',')
      const bodyFocusable = dialog?.querySelector<HTMLElement>(bodySelector)
      if (bodyFocusable) {
        bodyFocusable.focus()
      } else {
        const focusable = dialog?.querySelector<HTMLElement>(FOCUSABLE_SELECTOR)
        focusable?.focus()
      }
    }

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault()
        onClose()
        return
      }
      if (event.key !== 'Tab' || !dialog) {
        return
      }

      const focusableElements = Array.from(dialog.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR))
      if (focusableElements.length === 0) {
        event.preventDefault()
        dialog.focus()
        return
      }

      const first = focusableElements[0]
      const last = focusableElements[focusableElements.length - 1]
      if (!first || !last) {
        return
      }
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault()
        last.focus()
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault()
        first.focus()
      }
    }

    document.addEventListener('keydown', handleKeyDown)
    return () => {
      document.removeEventListener('keydown', handleKeyDown)
      previousFocus?.focus()
    }
  }, [onClose, open])

  if (!open) {
    return null
  }

  return (
    <div
      className="fixed inset-0 z-[var(--z-modal,1000)] grid [place-items:start_center] overflow-y-auto bg-black/55 backdrop-blur-[4px] p-[3rem_1rem] max-[680px]:p-4 animate-modal-fade"
      role="presentation"
      onClick={(event) => {
        if (closeOnBackdrop && event.target === event.currentTarget) {
          onClose()
        }
      }}
    >
      <section
        ref={dialogRef}
        className={clsx(
          'w-[min(100%,44rem)] max-[680px]:rounded-lg border border-border-strong rounded-xl bg-card shadow-[0_16px_48px_rgba(0,0,0,0.5)] text-foreground animate-modal-pop',
          className,
        )}
        role="dialog"
        aria-modal="true"
        aria-labelledby={labelledBy}
        tabIndex={-1}
      >
        <header
          className={`flex items-center justify-between gap-4 border-b border-border p-[1rem_1.25rem] max-[680px]:p-[0.85rem_1rem]`}
        >
          <div className="flex flex-col min-w-0">
            <h3 id={labelledBy} className="m-0 text-[1.05rem] tracking-tight">
              {title}
            </h3>
            {subtitle && (
              <p className="m-0 mt-[0.2rem] text-foreground-muted text-[0.86rem] leading-[1.45] [overflow-wrap:anywhere]">
                {subtitle}
              </p>
            )}
          </div>
          <button
            type="button"
            className={`inline-grid w-[1.85rem] h-[1.85rem] place-items-center border border-border rounded-[0.4rem] bg-transparent text-foreground-muted cursor-pointer text-[1.05rem] leading-none transition-colors duration-150 hover:border-rose-400/40 hover:bg-rose-400/12 hover:text-rose-200`}
            aria-label={t('common.modal_close_aria')}
            onClick={onClose}
          >
            ×
          </button>
        </header>
        <div
          className={clsx(
            'modal-body grid gap-[0.85rem] max-[680px]:gap-3 p-[1.1rem_1.25rem_1.25rem] max-[680px]:p-[0.85rem_1rem_1rem]',
            bodyClassName,
          )}
        >
          {children}
        </div>
      </section>
    </div>
  )
}
