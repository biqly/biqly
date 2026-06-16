import type { ReactNode } from 'react'

import { legacyCardClass } from '../../lib/cardClasses'

interface SettingsLinkCardProps {
  title: string
  description: string
  action: ReactNode
  icon?: ReactNode
}

export function SettingsLinkCard({ title, description, action, icon }: SettingsLinkCardProps) {
  return (
    <article
      className={legacyCardClass(
        'card card--elevated flex flex-col gap-[0.85rem] min-h-full mb-0 p-5 transition-all duration-220 ease-in-out hover:translate-y-[-3px] hover:border-accent hover:shadow-[0_12px_30px_var(--accent-glow)]',
      )}
    >
      <div className="flex items-center gap-3">
        {icon && (
          <div
            className={legacyCardClass(
              'flex items-center justify-center w-9 h-9 rounded-lg bg-card-raised border border-border shrink-0',
            )}
          >
            {icon}
          </div>
        )}
        <h2 className="m-0 text-[1rem] font-semibold">{title}</h2>
      </div>
      <p className="m-0 flex-1 text-[0.8125rem] leading-[1.45] text-foreground-muted">
        {description}
      </p>
      {action}
    </article>
  )
}
