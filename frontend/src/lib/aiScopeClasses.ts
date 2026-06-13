import { cn } from './cn'

export function aiScopeLabelClass(): string {
  return 'mb-[0.35rem] block text-[0.875rem] font-semibold text-foreground'
}

export function aiScopeTypeFiltersClass(): string {
  return 'mb-[0.45rem] flex flex-wrap gap-x-4 gap-y-[0.65rem]'
}

export function aiScopeTypeOptionClass(): string {
  return cn(
    'm-0 inline-flex cursor-pointer items-center gap-[0.4rem] text-[0.8rem] text-foreground-muted',
    '[&_input[type=checkbox]]:shrink-0',
  )
}

export function aiScopeMultiselectClass(): string {
  return cn(
    'mt-[0.45rem] box-border block w-full text-[0.8rem]',
    '[&_.ui-select-option]:text-[0.8rem]',
  )
}
