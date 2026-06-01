import type { ReactNode } from 'react'

type SettingsLinkCardProps = {
  title: string
  description: string
  action: ReactNode
  icon?: ReactNode
}

export function SettingsLinkCard({ title, description, action, icon }: SettingsLinkCardProps) {
  return (
    <article className="card card--elevated settings-link-card">
      <div className="settings-link-card__header">
        {icon && <div className="settings-link-card__icon-wrapper">{icon}</div>}
        <h2>{title}</h2>
      </div>
      <p>{description}</p>
      {action}
    </article>
  );
}
