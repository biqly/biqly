import { Fragment } from 'react'

export interface Crumb {
  label: string
  onClick?: () => void
}

interface BreadcrumbsProps {
  items: Crumb[]
  ariaLabel?: string
}

export function Breadcrumbs({ items, ariaLabel = 'Breadcrumb' }: BreadcrumbsProps) {
  if (items.length === 0) {
    return null
  }
  return (
    <nav className="mb-[0.35rem]" aria-label={ariaLabel}>
      <ol className="m-0 flex list-none flex-wrap items-center gap-[0.4rem] p-0 text-[0.8125rem]">
        {items.map((item, index) => {
          const isLast = index === items.length - 1
          return (
            <Fragment key={index}>
              <li className="inline-flex min-w-0">
                {item.onClick && !isLast ? (
                  <button
                    type="button"
                    className="cursor-pointer border-0 bg-transparent p-0 font-medium text-foreground [font-family:inherit] [font-size:inherit] [line-height:inherit] hover:text-accent hover:underline"
                    onClick={item.onClick}
                  >
                    {item.label}
                  </button>
                ) : (
                  <span
                    className="font-semibold text-foreground-muted"
                    aria-current={isLast ? 'page' : undefined}
                  >
                    {item.label}
                  </span>
                )}
              </li>
              {!isLast && (
                <li className="text-foreground-faint" aria-hidden="true">
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
