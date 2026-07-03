import { type ReactNode, useEffect, useRef } from 'react'

import { useT } from '../../i18n'
import { cn } from '../../lib/cn'

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

  // Lock body scroll while any modal is open so the page behind the overlay
  // can't scroll (and, on mobile, can't visually detach from the backdrop).
  useEffect(() => {
    if (!open) {
      return
    }
    const previousOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      document.body.style.overflow = previousOverflow
    }
  }, [open])

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
      if (event.key === 'Escape' && closeOnBackdrop) {
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
  }, [onClose, open, closeOnBackdrop])

  if (!open) {
    return null
  }

  return (
    <div
      className="animate-modal-fade fixed inset-0 z-(--z-modal,1000) grid [place-items:start_center] overflow-y-auto bg-black/55 p-[3rem_1rem] backdrop-blur-xs max-[680px]:p-4"
      role="presentation"
      tabIndex={-1}
      onClick={(event) => {
        if (closeOnBackdrop && event.target === event.currentTarget) {
          onClose()
        }
      }}
      onKeyDown={(event) => {
        if (
          closeOnBackdrop &&
          event.target === event.currentTarget &&
          (event.key === 'Enter' || event.key === ' ' || event.key === 'Spacebar')
        ) {
          event.preventDefault()
          onClose()
        }
      }}
    >
      <section
        ref={dialogRef}
        className={cn(
          'border-border-strong bg-card text-foreground animate-modal-pop w-[min(100%,44rem)] rounded-xl border shadow-[0_16px_48px_rgba(0,0,0,0.5)] max-[680px]:rounded-lg',
          className,
        )}
        role="dialog"
        aria-modal="true"
        aria-labelledby={labelledBy}
        tabIndex={-1}
      >
        <header
          className={`border-border flex items-center justify-between gap-4 border-b p-[1rem_1.25rem] max-[680px]:p-[0.85rem_1rem]`}
        >
          <div className="flex min-w-0 flex-col">
            <h3 id={labelledBy} className="m-0 text-[1.05rem] tracking-tight">
              {title}
            </h3>
            {subtitle && (
              <p className="text-foreground-muted m-0 mt-[0.2rem] text-[0.86rem] leading-[1.45] wrap-anywhere">
                {subtitle}
              </p>
            )}
          </div>
          <button
            type="button"
            className={`border-border text-foreground-muted inline-grid h-[1.85rem] w-[1.85rem] cursor-pointer place-items-center rounded-[0.4rem] border bg-transparent text-[1.05rem] leading-none transition-colors duration-150 hover:border-rose-400/40 hover:bg-rose-400/12 hover:text-rose-200`}
            aria-label={t('common.modal_close_aria')}
            onClick={onClose}
          >
            ×
          </button>
        </header>
        <div
          className={cn(
            'modal-body grid gap-[0.85rem] p-[1.1rem_1.25rem_1.25rem] max-[680px]:gap-3 max-[680px]:p-[0.85rem_1rem_1rem]',
            bodyClassName,
          )}
        >
          {children}
        </div>
      </section>
    </div>
  )
}
