import type { ReactNode } from 'react'

import { cn } from '../../lib/cn'

interface AdminFormSectionProps {
  title?: string
  children: ReactNode
  disabled?: boolean
  className?: string
}

export function AdminFormSection({ title, children, disabled, className }: AdminFormSectionProps) {
  return (
    <fieldset
      className={cn(
        'border-border bg-card-raised m-0 flex flex-col gap-4 rounded-[10px] border p-4 disabled:opacity-60 md:p-[16px_18px_18px]',
        '[&>legend]:text-foreground-muted [&>legend]:px-1.5 [&>legend]:text-xs [&>legend]:font-semibold [&>legend]:tracking-[0.4px] [&>legend]:uppercase',
        className,
      )}
      disabled={disabled}
    >
      {title && <legend>{title}</legend>}
      {children}
    </fieldset>
  )
}
