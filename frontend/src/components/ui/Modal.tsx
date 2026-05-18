import type { ReactNode } from 'react'
import { useT } from '../../i18n'

interface ModalProps {
  open: boolean
  title: ReactNode
  children: ReactNode
  onClose: () => void
  labelledBy?: string
}

export function Modal({ open, title, children, onClose, labelledBy = 'modal-title' }: ModalProps) {
  const t = useT()
  if (!open) return null

  return (
    <div
      className="modal-backdrop"
      role="presentation"
      onClick={(event) => { if (event.target === event.currentTarget) onClose() }}
    >
      <section className="modal-card modal-content" role="dialog" aria-modal="true" aria-labelledby={labelledBy}>
        <header className="modal-header">
          <h3 id={labelledBy}>{title}</h3>
          <button type="button" className="modal-close" aria-label={t('common.modal_close_aria')} onClick={onClose}>
            ×
          </button>
        </header>
        <div className="modal-body">
          {children}
        </div>
      </section>
    </div>
  )
}
