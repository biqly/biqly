import { cn } from './cn'
import { modalCardClass } from './modalClasses'

export function fewShotModalCardClass(): string {
  return cn(
    modalCardClass(),
    'flex min-h-0 w-[min(100%,56rem)] max-w-none max-h-[min(calc(100dvh-4rem),92vh)] flex-col overflow-hidden',
  )
}

export function modalBodyTwoColClass(): string {
  return cn(
    'grid min-h-0 flex-1 grid-cols-[minmax(0,1fr)_280px] items-start gap-5 overflow-y-auto overscroll-y-contain px-5 py-4',
    'max-[720px]:grid-cols-1',
  )
}

export function fewShotMainFormClass(): string {
  return 'flex flex-col gap-[0.85rem] overflow-visible pr-[0.25rem]'
}

export function fewShotSidebarClass(): string {
  return cn(
    'flex min-h-0 flex-col gap-3 self-stretch border-l border-border pl-5',
    'max-[720px]:border-t max-[720px]:border-l-0 max-[720px]:pt-4 max-[720px]:pl-0',
  )
}

export function fewShotSidebarHeaderClass(): string {
  return 'text-[0.85rem] font-semibold text-foreground'
}

export function fewShotSidebarListClass(): string {
  return 'flex max-h-[min(42vh,22rem)] flex-col gap-[0.4rem] overflow-y-auto overscroll-y-contain pr-[0.25rem]'
}

export const fieldBadgeBtnClass = cn(
  'flex items-center justify-between text-left w-full py-[0.35rem] px-2',
  'bg-white/[0.02] border border-border rounded-[0.35rem] font-mono text-[0.75rem]',
  'text-foreground-muted cursor-pointer transition-all duration-150 ease',
  'hover:bg-[rgba(96,165,250,0.08)] hover:border-[rgba(96,165,250,0.3)] hover:text-accent',
)

export const fieldBadgeBtnTypeClass = cn(
  'text-[0.65rem] py-[0.05rem] px-1 rounded-[0.2rem]',
  'bg-white/[0.05] text-foreground-muted',
)
