import type { ReactNode } from 'react'

import { cardClass } from '../../lib/cardClasses'

interface SettingsLinkCardProps {
  title: string
  description: string
  action: ReactNode
  icon?: ReactNode
}

export function SettingsLinkCard({ title, description, action, icon }: SettingsLinkCardProps) {
  return (
    <article
      className={cardClass({
        elevated: true,
        className:
          'hover:border-accent mb-0 flex min-h-full flex-col gap-[0.85rem] p-5 transition-all duration-220 ease-in-out hover:-translate-y-0.75 hover:shadow-[0_12px_30px_var(--accent-glow)]',
      })}
    >
      <div className="flex items-center gap-3">
        {icon && (
          <div className="bg-card-raised border-border flex h-9 w-9 shrink-0 items-center justify-center rounded-lg border">
            {icon}
          </div>
        )}
        <h2 className="m-0 text-[1rem] font-semibold">{title}</h2>
      </div>
      <p className="text-foreground-muted text-caption m-0 flex-1 leading-[1.45]">{description}</p>
      {action}
    </article>
  )
}
