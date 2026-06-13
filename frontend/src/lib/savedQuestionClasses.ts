import { cn } from './cn'

export function savedQuestionListClass(): string {
  return 'grid grid-cols-[1fr_2fr] gap-6'
}

export function savedQuestionItemClass(selected: boolean): string {
  return cn(
    'block w-full cursor-pointer rounded-lg border border-border bg-transparent p-[0.85rem_1rem] text-left text-inherit transition-[background-color,border-color] duration-[120ms]',
    'hover:border-[var(--border-strong)] hover:bg-[var(--bg-card-raised)]',
    selected && 'border-[var(--accent-strong)] bg-[var(--bg-card-raised)]',
  )
}

export function savedQuestionItemTopClass(): string {
  return 'mb-1 flex items-start justify-between gap-[0.65rem]'
}

export function savedQuestionItemTitleClass(): string {
  return 'm-0 min-w-0 flex-1 text-base leading-[1.35] text-foreground'
}

export function savedQuestionItemMetaPClass(): string {
  return 'm-0 text-[0.8rem] text-foreground-muted'
}

export function savedQuestionRowClass(): string {
  return 'relative'
}

export function fewshotCheckboxClass(inline = false, active = false): string {
  return cn(
    'flex cursor-pointer items-center gap-[0.3rem] rounded-[0.3rem] bg-transparent text-[0.72rem] text-foreground-muted',
    inline
      ? 'static max-w-[42%] shrink-0 text-right [&_span]:whitespace-nowrap'
      : 'absolute top-[0.6rem] right-[0.6rem] px-[0.4rem] py-[0.15rem]',
    active && 'bg-[rgba(96,165,250,0.1)] text-[var(--accent)]',
  )
}

export function savedQuestionFavClass(active = false): string {
  return cn(
    'absolute right-[0.6rem] bottom-[0.6rem] cursor-pointer rounded-[0.35rem] border-0 bg-transparent p-[0.2rem_0.3rem] text-[1.1rem] leading-none text-foreground-muted',
    'hover:bg-card hover:text-warning',
    active && 'text-warning',
  )
}

export function savedQuestionDescriptionClass(): string {
  return 'm-0 text-[0.875rem] leading-[1.5] whitespace-pre-wrap text-foreground-muted'
}

export function savedQuestionActionsClass(): string {
  return 'mt-4 flex flex-wrap gap-2'
}
