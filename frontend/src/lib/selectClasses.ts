import { cn } from './cn'

/** Root wrapper — keeps `ui-select` marker for parent `[&_.ui-select]` overrides. */
export const selectRootClass = cn('ui-select relative block w-full min-w-0')

export const selectMultiselectClass = 'ui-multiselect'

export const selectMultiselectInlineClass = 'ui-multiselect--inline'

export const selectMultiselectInlineRootClass = cn(
  selectRootClass,
  selectMultiselectClass,
  selectMultiselectInlineClass,
)

export function selectTriggerClass(options?: {
  size?: 'sm' | 'md'
  open?: boolean
  empty?: boolean
  stacked?: boolean
  className?: string
}): string {
  return cn(
    'ui-select-trigger group/trigger',
    'flex items-center justify-between gap-2 w-full min-h-[2.1rem]',
    'py-[0.35rem] pl-[0.7rem] pr-[0.6rem] border border-border rounded-[0.4rem]',
    'bg-card-raised text-foreground text-[0.8rem] leading-[1.3] text-left cursor-pointer font-inherit',
    'shadow-[inset_0_1px_0_var(--control-surface-highlight)]',
    'transition-[background-color,border-color,box-shadow] duration-120 ease-in-out',
    'hover:not(:disabled):border-(--control-hover-border) hover:not(:disabled):bg-(--control-hover-bg)',
    'focus-visible:outline-none focus-visible:border-(--control-focus-border)',
    'focus-visible:shadow-[inset_0_1px_0_var(--control-surface-highlight),0_0_0_3px_var(--control-focus-ring)]',
    'disabled:cursor-not-allowed disabled:opacity-45',
    options?.open && 'is-open border-(--control-active-border) bg-(--control-open-bg)',
    options?.stacked && 'ui-select-trigger--stacked',
    options?.size === 'sm' &&
      'ui-select-trigger--sm min-h-[1.85rem] py-[0.3rem] pl-[0.6rem] pr-[0.5rem] text-[0.76rem] rounded-[0.35rem]',
    options?.empty && 'is-empty',
    options?.className,
  )
}

export function selectValueClass(options?: {
  placeholder?: boolean
  stacked?: boolean
  className?: string
}): string {
  return cn(
    'ui-select-value flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap',
    options?.placeholder && 'is-placeholder text-foreground-faint',
    options?.stacked &&
      'ui-select-value--stacked flex flex-col gap-[0.12rem] items-start whitespace-normal overflow-visible text-clip',
    options?.className,
  )
}

export const selectValuePrimaryClass = cn(
  'ui-select-value-primary block w-full line-clamp-2 leading-[1.3] break-words',
)

export const selectValueHintClass = cn(
  'ui-select-value-hint text-[0.62rem] font-bold tracking-[0.04em] uppercase text-foreground-faint leading-[1.2]',
)

export function selectChevronClass(open?: boolean): string {
  return cn(
    'ui-select-chevron shrink-0 text-foreground-faint transition-[transform,color] duration-140 ease-in-out',
    open && 'rotate-180 text-foreground-muted',
  )
}

export function selectPopoverClass(placement: 'down' | 'up'): string {
  return cn(
    'ui-select-popover z-[5000] overflow-hidden border border-border-strong rounded-[0.55rem]',
    'bg-canvas-subtle shadow-[0_1px_0_rgba(255,255,255,0.03)_inset,0_18px_40px_rgba(0,0,0,0.55),0_4px_14px_rgba(0,0,0,0.35)]',
    'animate-ui-select-pop',
    placement === 'up' ? 'ui-select-popover--up origin-bottom' : 'origin-top',
  )
}

export const selectHeaderClass = cn(
  'ui-select-header px-[0.7rem] pt-2 pb-1 text-foreground-faint text-[0.6rem] font-semibold tracking-[0.09em] uppercase',
)

export const selectSearchWrapClass = cn(
  'ui-select-search px-2 pt-[0.35rem] pb-1 border-b border-border',
)

export const selectSearchInputClass = cn(
  'ui-select-search-input box-border w-full px-2 py-[0.4rem] border border-border-strong rounded-[0.35rem]',
  'text-[0.78rem] bg-canvas text-foreground',
  'focus:outline-none focus:border-accent',
)

/** List shell — scrollbar styling stays in index.css @layer components. */
export const selectListClass = 'ui-select-list list-none m-0 p-1 overflow-y-auto'

export function selectOptionClass(options: {
  selected?: boolean
  active?: boolean
  disabled?: boolean
}): string {
  return cn(
    'ui-select-option flex items-center gap-[0.45rem] py-[0.32rem] pl-[0.4rem] pr-[0.5rem] rounded-[0.35rem]',
    'text-foreground-muted text-[0.78rem] cursor-pointer',
    'transition-[background-color,color] duration-100 ease-in-out',
    options.active && 'is-active bg-(--control-option-active-bg) text-foreground',
    options.selected && 'is-selected text-foreground',
    options.disabled && 'is-disabled opacity-40 cursor-not-allowed',
  )
}

export const selectCheckClass = cn(
  'ui-select-check inline-grid place-items-center shrink-0 w-[0.85rem] h-[0.85rem] text-foreground',
)

export const selectLabelClass = cn(
  'ui-select-label flex-1 min-w-0 overflow-hidden text-ellipsis whitespace-nowrap',
)

/** Inside popover, labels may wrap (replaces `.ui-select-popover .ui-select-label`). */
export const selectPopoverLabelClass = cn(selectLabelClass, 'overflow-visible text-clip')

export const selectHintClass = cn(
  'ui-select-hint ml-[0.35rem] text-foreground-faint text-[0.7rem] font-normal',
)

export const selectCountClass = cn(
  'ui-select-count shrink-0 ml-[0.35rem] text-foreground-faint text-[0.72rem] tabular-nums',
)

export const selectEmptyClass = cn(
  'ui-select-empty px-[0.65rem] py-[0.45rem] text-foreground-faint text-[0.76rem]',
)
