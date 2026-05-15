import type { ReactNode } from 'react'

type Props = {
  /** Primary line (optional if description is enough) */
  title?: string
  description?: string
  children?: ReactNode
  /** Extra class on root (e.g. card padding context) */
  className?: string
}

export function EmptyState({ title, description, children, className = '' }: Props) {
  if (!title && !description && children == null) return null

  const rootCls = ['ui-empty-state', className].filter(Boolean).join(' ')

  return (
    <div className={rootCls} role="status">
      {title ? <h3 className="ui-empty-state__title">{title}</h3> : null}
      {description ? <p className="ui-empty-state__desc">{description}</p> : null}
      {children ? <div className="ui-empty-state__slot">{children}</div> : null}
    </div>
  )
}
