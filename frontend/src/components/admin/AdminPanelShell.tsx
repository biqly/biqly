import type { ReactNode } from 'react'

import { useT } from '../../i18n'
import { cn } from '../../lib/cn'
import { legacyLayoutClass } from '../../lib/layoutClasses'
import { ErrorAlert } from '../ui/ErrorAlert'
import { ReadOnlyNote } from './ReadOnlyNote'

interface AdminPanelShellProps {
  title: string
  description?: ReactNode
  action?: ReactNode
  readOnly?: boolean
  error?: string | null
  children: ReactNode
  maxWidth?: string | number
  className?: string
}

export function AdminPanelShell({
  title,
  description,
  action,
  readOnly,
  error,
  children,
  maxWidth = 1000,
  className,
}: AdminPanelShellProps) {
  const t = useT()
  const maxWStyle = {
    maxWidth: typeof maxWidth === 'number' ? `${maxWidth}px` : maxWidth,
  }

  return (
    <div className={cn(legacyLayoutClass('page-stack'), 'w-full', className)} style={maxWStyle}>
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div>
          <h2 className="m-0 text-xl font-bold">{title}</h2>
          {description && (
            <p className="text-foreground-muted m-0 mt-1 text-xs leading-normal">{description}</p>
          )}
        </div>
        {action && <div>{action}</div>}
      </div>

      {readOnly && <ReadOnlyNote />}

      {error && <ErrorAlert error={`${t('common.error')}: ${error}`} />}

      {children}
    </div>
  )
}
