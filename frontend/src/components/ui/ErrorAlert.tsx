import type { ReactNode } from 'react'

import { cn } from '../../lib/cn'
import { errorAlertClass } from '../../lib/feedbackClasses'

interface ErrorAlertProps {
  error: string | null | undefined
  className?: string
  children?: ReactNode
}

export function ErrorAlert({ error, className, children }: ErrorAlertProps) {
  if (!error && !children) {
    return null
  }
  return (
    <div className={cn(errorAlertClass, className)} role="alert">
      {error}
      {children}
    </div>
  )
}
