import { useEffect, useRef, type ReactNode } from 'react'
import clsx from 'clsx'
import { useT } from '../../i18n'

interface ModalProps {
  open: boolean
  title: ReactNode
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
    if (!open) return
    const previousFocus = document.activeElement instanceof HTMLElement ? document.activeElement : null
    const dialog = dialogRef.current
    const focusable = dialog?.querySelector<HTMLElement>(FOCUSABLE_SELECTOR)
    focusable?.focus()

    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        event.preventDefault()
        onClose()
        return
      }
      if (event.key !== 'Tab' || !dialog) return

      const focusableElements = Array.from(dialog.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR))
      if (focusableElements.length === 0) {
        event.preventDefault()
        dialog.focus()
        return
      }

      const first = focusableElements[0]
      const last = focusableElements[focusableElements.length - 1]
      if (!first || !last) return
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

  if (!open) return null

  return (
    <div
      className="modal-backdrop"
      role="presentation"
      onClick={(event) => {
        if (closeOnBackdrop && event.target === event.currentTarget) onClose()
      }}
    >
      <section
        ref={dialogRef}
        className={clsx('modal-card', 'modal-content', className)}
        role="dialog"
        aria-modal="true"
        aria-labelledby={labelledBy}
        tabIndex={-1}
      >
        <header className="modal-header">
          <h3 id={labelledBy}>{title}</h3>
          <button type="button" className="modal-close" aria-label={t('common.modal_close_aria')} onClick={onClose}>
            ×
          </button>
        </header>
        <div className={clsx('modal-body', bodyClassName)}>
          {children}
        </div>
      </section>
    </div>
  )
}
