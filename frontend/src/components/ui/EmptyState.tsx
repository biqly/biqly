import type { ReactNode } from 'react'

import { Button } from './Button'

export interface EmptyStateAction {
  label: string
  onClick: () => void
  variant?: 'primary' | 'secondary'
}

interface Props {
  /** Primary line (optional if description is enough) */
  title?: string
  description?: string
  /** Decorative illustration/icon shown above the title. */
  icon?: ReactNode
  /** Optional call-to-action button. */
  action?: EmptyStateAction
  children?: ReactNode
  /** Extra class on root (e.g. card padding context) */
  className?: string
}

export function EmptyState({ title, description, icon, action, children, className = '' }: Props) {
  if (!title && !description && icon == null && action == null && children == null) {
    return null
  }

  const rootCls = ['ui-empty-state', className].filter(Boolean).join(' ')

  return (
    <div className={rootCls} role="status">
      {icon ? (
        <div className="ui-empty-state__icon" aria-hidden="true">
          {icon}
        </div>
      ) : null}
      {title ? <h3 className="ui-empty-state__title">{title}</h3> : null}
      {description ? <p className="ui-empty-state__desc">{description}</p> : null}
      {action ? (
        <Button
          variant={action.variant ?? 'primary'}
          className="ui-empty-state__action"
          onClick={action.onClick}
        >
          {action.label}
        </Button>
      ) : null}
      {children ? <div className="ui-empty-state__slot">{children}</div> : null}
    </div>
  )
}
