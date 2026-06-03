import type { ReactNode } from 'react'

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
    <div className={`error${className ? ` ${className}` : ''}`} role="alert">
      {error}
      {children}
    </div>
  )
}
