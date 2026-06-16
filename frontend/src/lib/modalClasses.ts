import { cn } from './cn'

export function modalBackdropClass(): string {
  return cn(
    'fixed inset-0 z-[1000] flex items-center justify-center p-4',
    'bg-black/60 backdrop-blur-[4px]',
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

export function modalEnumValuesCardClass(): string {
  return cn(modalCardClass(), 'w-[min(100%,42rem)] max-w-none flex min-h-0 flex-col')
}

export const modalEnumValuesHelpClass =
  'm-0 rounded-lg border border-border bg-card-raised px-3 py-2.5 text-[0.8rem] leading-snug text-foreground-muted'

export const modalEnumValuesGridClass =
  'grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_minmax(0,1.4fr)_auto] gap-2 items-center max-[640px]:grid-cols-1'

export const modalEnumValuesHeaderClass = cn(
  modalEnumValuesGridClass,
  'max-[640px]:hidden px-1 pb-0.5 text-[0.68rem] font-bold uppercase tracking-[0.06em] text-foreground-faint',
)

export const modalEnumValuesRowClass = cn(
  modalEnumValuesGridClass,
  'rounded-lg border border-border bg-card-raised p-2',
  '[&_input]:w-full [&_input]:min-h-[2rem] [&_input]:rounded-md [&_input]:border [&_input]:border-border',
  '[&_input]:bg-canvas [&_input]:px-2.5 [&_input]:py-1.5 [&_input]:text-[0.82rem] [&_input]:text-foreground',
  '[&_input]:outline-none [&_input:focus-visible]:border-accent/55',
  'max-[640px]:[&_input]:mb-0',
)

export const modalEnumValuesListClass =
  'flex flex-col gap-2 max-h-[min(42vh,20rem)] overflow-y-auto pr-0.5'

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

export function modalDescribeCardClass(hasResults?: boolean): string {
  return cn(
    modalCardClass(),
    hasResults ? 'w-[min(100%,52rem)] max-h-[min(calc(100vh-1.5rem),90vh)]' : 'w-[min(100%,40rem)]',
    'max-w-none flex min-h-0 flex-col',
  )
}

export const modalDescribeHeaderClass = cn(
  modalHeaderClass(),
  'items-start gap-3 px-4 py-3 sm:px-5 sm:py-4',
)

export const modalDescribeTitleClass =
  'm-0 text-[0.95rem] font-[650] tracking-[-0.02em] leading-tight text-foreground'

export const modalDescribeFqnClass =
  'mt-[0.2rem] mb-0 text-[0.72rem] font-mono text-foreground-faint leading-snug'

export const modalDescribeIntroClass = 'm-0 text-[0.78rem] leading-[1.48] text-foreground-muted'

export const modalDescribeSectionTitleClass =
  'm-0 mb-2 text-[0.86rem] font-[650] text-foreground tracking-[-0.01em]'

export const modalDescribeFieldsetClass =
  'm-0 min-w-0 rounded-lg border border-border bg-card-raised p-[0.65rem_0.75rem]'

export const modalDescribeLegendClass =
  'px-[0.25rem] text-[0.62rem] font-[800] uppercase tracking-[0.07em] text-foreground-faint'

export const modalDescribeSampleInputClass =
  'w-[5rem] text-[0.82rem] px-[0.45rem] py-[0.3rem] border border-border rounded-lg bg-card text-foreground'

export function modalDescribeBodyClass(hasResults?: boolean): string {
  return cn(modalBodyClass(), hasResults ? 'gap-4 overflow-y-auto min-h-0 flex-1' : 'gap-[0.65rem]')
}

export function modalDescribeStatusBannerClass(applied?: boolean): string {
  return cn(
    'rounded-lg border px-3 py-2 text-[0.78rem] leading-snug',
    applied
      ? 'border-[color-mix(in_srgb,var(--success)_25%,transparent)] bg-[color-mix(in_srgb,var(--success)_8%,transparent)] text-success'
      : 'border-border bg-card-raised text-foreground-muted',
  )
}

export const modalDescribeMetaLineClass =
  'm-0 text-[0.76rem] text-foreground-faint [&_code]:font-mono [&_code]:text-[0.74rem] [&_code]:text-foreground-muted'

export const modalDescribeEmptyEmClass = 'text-foreground-muted italic'

export const modalDescribeResultsScrollClass =
  'max-w-full overflow-x-auto overflow-y-auto max-h-[min(40vh,18rem)] rounded-lg border border-border'

export const modalDescribeResultsTableClass = cn(
  'w-full min-w-[28rem] border-collapse text-caption table-fixed',
  '[&_th]:px-3 [&_td]:px-3 [&_th]:py-2 [&_td]:py-2',
  '[&_thead_th]:text-left [&_thead_th]:text-[0.68rem] [&_thead_th]:font-bold [&_thead_th]:uppercase',
  '[&_thead_th]:tracking-[0.06em] [&_thead_th]:text-foreground-muted [&_thead_th]:align-middle',
  '[&_thead_th]:border-b [&_thead_th]:border-border-strong [&_thead_th]:bg-[var(--table-header-bg)]',
  '[&_thead_th:last-child]:text-right',
  '[&_tbody_td]:border-b [&_tbody_td]:border-border [&_tbody_td]:align-top [&_tbody_td]:text-[0.82rem]',
  '[&_tbody_tr:last-child_td]:border-b-0',
  '[&_tbody_td:first-child]:w-[22%] [&_tbody_td:first-child_code]:text-[0.76rem] [&_tbody_td:first-child_code]:break-all',
  '[&_tbody_td:nth-child(2)]:break-words [&_tbody_td:nth-child(2)]:[overflow-wrap:anywhere]',
  '[&_td.actions]:text-right [&_td.actions]:whitespace-nowrap [&_td.actions]:align-middle',
)
