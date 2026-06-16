import type { ReactNode } from 'react'

import { cn } from '../../lib/cn'
import {
  uiEmptyStateActionClass,
  uiEmptyStateClass,
  uiEmptyStateDescClass,
  uiEmptyStateIconClass,
  uiEmptyStateInlineClass,
  uiEmptyStateSlotClass,
  uiEmptyStateTitleClass,
} from '../../lib/feedbackClasses'
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

  const rootCls = cn(
    uiEmptyStateClass,
    className === 'ui-empty-state--inline' ? uiEmptyStateInlineClass : className,
  )

  return (
    <div className={rootCls}>
      {icon ? (
        <div className={uiEmptyStateIconClass} aria-hidden="true">
          {icon}
        </div>
      ) : null}
      {title ? <h3 className={uiEmptyStateTitleClass}>{title}</h3> : null}
      {description ? <p className={uiEmptyStateDescClass}>{description}</p> : null}
      {action ? (
        <Button
          variant={action.variant ?? 'primary'}
          className={uiEmptyStateActionClass}
          onClick={action.onClick}
        >
          {action.label}
        </Button>
      ) : null}
      {children ? <div className={uiEmptyStateSlotClass}>{children}</div> : null}
    </div>
  )
}
