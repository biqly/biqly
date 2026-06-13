import type { ReactNode } from 'react'

interface SettingsLinkCardProps {
  title: string
  description: string
  action: ReactNode
  icon?: ReactNode
}

export function SettingsLinkCard({ title, description, action, icon }: SettingsLinkCardProps) {
  return (
    <article className="card card--elevated flex flex-col gap-[0.85rem] min-h-full mb-0 p-5 transition-all duration-[220ms] ease-[cubic-bezier(0.4,0,0.2,1)] hover:-translate-y-[3px] hover:border-accent hover:shadow-[0_12px_30px_var(--accent-glow)]">
      <div className="flex items-center gap-3">
        {icon && (
          <div
            className={`flex items-center justify-center w-9 h-9 rounded-lg bg-[var(--bg-card-raised)] border border-border shrink-0`}
          >
            {icon}
          </div>
        )}
        <h2 className="m-0 text-[1rem] font-semibold">{title}</h2>
      </div>
      <p className="m-0 flex-1 text-foreground-muted text-[0.8125rem] leading-[1.45]">
        {description}
      </p>
      {action}
    </article>
  )
}
