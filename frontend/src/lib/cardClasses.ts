import { cn } from './cn'

const cardHeading =
  '[&_h2]:m-0 [&_h2]:mb-4 [&_h2]:text-foreground [&_h2]:font-[family-name:var(--font-display)] [&_h2]:text-[1.15rem] [&_h2]:font-bold [&_h2]:tracking-[-0.015em] [&_h3]:text-foreground [&_h3]:font-[family-name:var(--font-display)] [&_h3]:text-[1.02rem] [&_h3]:font-bold [&_h3]:tracking-[-0.01em]'

export function cardClass(options?: { elevated?: boolean; className?: string }): string {
  return cn(
    'min-w-0 overflow-x-auto border border-border rounded-[0.85rem] bg-card p-6 mb-5 shadow-card',
    'max-[680px]:p-4 max-[680px]:rounded-[0.65rem]',
    'transition-[transform,border-color,box-shadow] duration-[220ms] ease-[cubic-bezier(0.4,0,0.2,1)]',
    'hover:border-border-strong hover:shadow-[var(--shadow-card-hover)]',
    cardHeading,
    options?.elevated && 'border-border-strong',
    options?.className,
  )
}

export const cardIntroClass = cn('flex flex-col gap-[0.65rem] mb-[1.35rem]', '[&_h2]:m-0 [&>p]:m-0')

export const cardIntroCompactClass = 'mb-4'

export const cardHeaderRowClass = cn(
  'flex justify-between items-center flex-wrap gap-y-[0.65rem] gap-x-4 mb-0',
  '[&_h2]:m-0 [&_button]:w-auto [&_button]:mt-0',
)

export const cardLeadClass =
  'm-0 mb-5 text-foreground-muted text-[0.86rem] leading-[1.45] max-w-full'

export const cardLeadSingleLineClass = 'xl:whitespace-nowrap xl:overflow-hidden xl:text-ellipsis'

export const cardSubtitleClass = cardLeadClass

export const savedQuestionIntroClass = cardLeadClass

/** Maps legacy BEM card class strings to Tailwind for gradual migration. */
export function legacyCardClass(className: string): string {
  const parts = className.split(/\s+/).filter(Boolean)
  let elevated = false
  let extra = ''

  for (const part of parts) {
    if (part === 'card') {
      continue
    }
    if (part === 'card--elevated') {
      elevated = true
    } else if (part === 'card-intro') {
      extra = cn(extra, cardIntroClass)
    } else if (part === 'card-intro--compact') {
      extra = cn(extra, cardIntroCompactClass)
    } else if (part === 'card-header-row' || part === 'card-header-row--spaced') {
      extra = cn(extra, cardHeaderRowClass)
    } else if (
      part === 'card-lead' ||
      part === 'card-subtitle' ||
      part === 'saved-question-intro'
    ) {
      extra = cn(extra, cardLeadClass)
    } else if (part === 'card-lead--single-line') {
      extra = cn(extra, cardLeadSingleLineClass)
    } else {
      extra = cn(extra, part)
    }
  }

  const hasCard = parts.includes('card')
  if (hasCard) {
    return cardClass({ elevated, className: extra })
  }

  return extra
}
