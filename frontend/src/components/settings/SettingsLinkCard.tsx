import type { ReactNode } from 'react'

type SettingsLinkCardProps = {
  title: string
  description: string
  action: ReactNode
}

export function SettingsLinkCard({ title, description, action }: SettingsLinkCardProps) {
  return (
    <article className="card card--elevated settings-link-card">
      <h2>{title}</h2>
      <p>{description}</p>
      {action}
    </article>
  )
}
