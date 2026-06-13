import { cn } from './cn'

export function modalBackdropClass(): string {
  return cn(
    'fixed inset-0 z-[1000] flex items-center justify-center p-4',
    'bg-[rgba(15,23,42,0.55)] backdrop-blur-[2px]',
  )
}

export function modalCardClass(): string {
  return cn(
    'w-full max-w-[min(100%,28rem)] max-h-[min(90vh,40rem)]',
    'flex flex-col overflow-hidden rounded-[var(--radius)] border border-border bg-card shadow-[var(--shadow)]',
  )
}

export function modalAvatarCardClass(): string {
  return cn(modalCardClass(), 'w-[min(100%,25rem)] max-w-none')
}

export function modalHeaderClass(): string {
  return cn('flex shrink-0 items-center justify-between gap-3 border-b border-border px-5 py-4')
}

export function modalTitleClass(): string {
  return 'm-0 text-[1.05rem] font-semibold text-foreground'
}

export function modalCloseClass(): string {
  return cn(
    'inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-[var(--radius)]',
    'border-0 bg-transparent p-0 text-[1.25rem] leading-none text-foreground-muted',
    'cursor-pointer transition-colors duration-150',
    'hover:bg-[var(--bg-hover)] hover:text-foreground',
  )
}

export function modalBodyClass(): string {
  return 'flex min-h-0 flex-1 flex-col gap-3 overflow-y-auto px-5 py-4'
}

export function modalActionsClass(): string {
  return 'mt-2 flex flex-wrap justify-end gap-2'
}

export function modalActionsBorderedClass(): string {
  return cn(modalActionsClass(), 'mt-0 border-t border-border pt-[0.85rem]')
}

export function modalBulkCardClass(): string {
  return cn(
    modalCardClass(),
    'w-[min(100%,52rem)] max-h-[min(calc(100vh-1.5rem),90vh)] max-w-none flex min-h-0 flex-col',
  )
}

export function modalBulkHeaderClass(): string {
  return cn(modalHeaderClass(), 'items-start gap-3 p-[0.65rem_1rem]')
}

export function modalBodyScrollClass(): string {
  return cn(
    modalBodyClass(),
    'flex min-h-0 flex-1 flex-col gap-[0.65rem] overflow-hidden pb-4 pt-[0.85rem]',
  )
}

export function modalBodyCompactClass(): string {
  return cn(modalBodyClass(), 'gap-[0.65rem] p-[0.85rem_1rem_1rem]')
}

export function modalContentClass(): string {
  return cn(modalCardClass(), 'w-[90%] max-w-[min(100%,37.5rem)]')
}

export function modalSubtitleClass(): string {
  return 'm-0 text-[0.875rem] leading-snug text-foreground-muted'
}

export function modalFormRowClass(): string {
  return cn('grid grid-cols-2 gap-[0.85rem]', 'max-[680px]:grid-cols-1')
}

export function modalInviteUserCardClass(): string {
  return 'w-[min(100%,25rem)]'
}

export function modalModelingCardClass(): string {
  return 'w-[min(100%,30rem)]'
}

export function modalDashboardCardClass(): string {
  return 'w-[min(100%,28rem)]'
}

export function modalModelingBodyClass(): string {
  return 'gap-3'
}

export function checkboxRowClass(): string {
  return cn(
    'flex items-center gap-2 py-[0.4rem]',
    '[&_label]:m-0 [&_label]:font-medium [&_label]:text-foreground',
  )
}

export const confirmDialogMessageClass =
  'm-0 mb-2 text-[0.9rem] leading-normal text-foreground-muted'

export const confirmDialogActionsClass = cn(
  'mt-3 flex justify-end gap-2',
  '[&_button]:mt-0 [&_button]:min-h-[2.25rem] [&_button]:w-auto [&_button]:min-w-[5.5rem]',
)
