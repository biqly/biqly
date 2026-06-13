import React, { useId, useState } from 'react'

import {
  qbStepCardClass,
  qbStepChevronClass,
  qbStepCloseClass,
  qbStepLabelClass,
  qbStepRootClass,
  qbStepSummaryCardClass,
} from './queryBuilderClasses'

interface NotebookStepProps {
  label: string
  themeClass: string
  onClose?: () => void
  closeTitle?: string
  /** When true, the step body can be collapsed to a one-line summary. */
  collapsible?: boolean
  /** Shown in place of the body when collapsed (e.g. a count of configured rows). */
  summary?: React.ReactNode
  /** Start collapsed — used for advanced steps to reduce initial clutter. */
  defaultCollapsed?: boolean
  children: React.ReactNode
}

export function NotebookStep({
  label,
  themeClass,
  onClose,
  closeTitle,
  collapsible = false,
  summary,
  defaultCollapsed = false,
  children,
}: NotebookStepProps) {
  const [collapsed, setCollapsed] = useState(defaultCollapsed)
  const bodyId = useId()
  const isCollapsed = collapsible && collapsed
  const toggle = () => setCollapsed((v) => !v)

  return (
    <div className={qbStepRootClass}>
      {collapsible ? (
        <button
          type="button"
          className={qbStepLabelClass(themeClass, true)}
          onClick={toggle}
          aria-expanded={!collapsed}
          aria-controls={bodyId}
        >
          <span className={qbStepChevronClass} aria-hidden="true">
            {isCollapsed ? '▸' : '▾'}
          </span>
          <span>{label}</span>
        </button>
      ) : (
        <div className={qbStepLabelClass(themeClass)}>{label}</div>
      )}
      {isCollapsed ? (
        <button type="button" className={qbStepSummaryCardClass} onClick={toggle}>
          {summary}
        </button>
      ) : (
        <div id={bodyId} className={qbStepCardClass(themeClass)}>
          {children}
          {onClose && (
            <button type="button" className={qbStepCloseClass} onClick={onClose} title={closeTitle}>
              ×
            </button>
          )}
        </div>
      )}
    </div>
  )
}
