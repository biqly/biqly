import React from 'react'

interface NotebookStepProps {
  label: string
  themeClass: string
  onClose?: () => void
  closeTitle?: string
  children: React.ReactNode
}

export function NotebookStep({
  label,
  themeClass,
  onClose,
  closeTitle,
  children,
}: NotebookStepProps) {
  return (
    <div className="notebook-step">
      <div className={`notebook-step-label notebook-step-label--${themeClass}`}>{label}</div>
      <div className={`notebook-step-card notebook-step-card--${themeClass}`}>
        {children}
        {onClose && (
          <button
            type="button"
            className="notebook-step-close"
            onClick={onClose}
            title={closeTitle}
          >
            ×
          </button>
        )}
      </div>
    </div>
  )
}
