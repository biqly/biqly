import { Fragment } from 'react'
import '../../styles/breadcrumbs.css'

export interface Crumb {
  label: string
  onClick?: () => void
}

interface BreadcrumbsProps {
  items: Crumb[]
  ariaLabel?: string
}

export function Breadcrumbs({ items, ariaLabel = 'Breadcrumb' }: BreadcrumbsProps) {
  if (items.length === 0) return null
  return (
    <nav className="breadcrumbs" aria-label={ariaLabel}>
      <ol className="breadcrumbs__list">
        {items.map((item, index) => {
          const isLast = index === items.length - 1
          return (
            <Fragment key={index}>
              <li className="breadcrumbs__item">
                {item.onClick && !isLast ? (
                  <button type="button" className="breadcrumbs__link" onClick={item.onClick}>
                    {item.label}
                  </button>
                ) : (
                  <span className="breadcrumbs__current" aria-current={isLast ? 'page' : undefined}>
                    {item.label}
                  </span>
                )}
              </li>
              {!isLast && (
                <li className="breadcrumbs__sep" aria-hidden="true">
                  ›
                </li>
              )}
            </Fragment>
          )
        })}
      </ol>
    </nav>
  )
}
