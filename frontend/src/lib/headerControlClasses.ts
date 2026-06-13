import { cn } from './cn'

export const sidebarFooterControlsClass = 'flex w-full min-w-0 flex-nowrap items-center gap-1.5'

export const segmentedControlShellClass =
  'inline-flex shrink-0 items-center bg-card-raised border border-border rounded-full p-[0.18rem] gap-[0.1rem]'

export function segmentedControlBtnClass(
  active: boolean,
  options?: { theme?: boolean; compact?: boolean },
): string {
  return cn(
    'inline-flex items-center justify-center h-[1.65rem] border-0 rounded-full bg-transparent text-foreground-muted cursor-pointer transition-[background,color] duration-140 ease-in-out hover:text-foreground',
    options?.compact
      ? 'min-w-7 px-2 text-[0.68rem] font-semibold tracking-wide'
      : 'min-w-8 px-[0.55rem] text-[0.72rem] font-semibold tracking-wide',
    options?.theme &&
      (options.compact
        ? 'min-w-[1.55rem] px-[0.3rem] [&_svg]:w-[0.85rem] [&_svg]:h-[0.85rem]'
        : 'min-w-[1.85rem] px-[0.4rem] [&_svg]:w-[0.95rem] [&_svg]:h-[0.95rem]'),
    active && 'bg-[var(--bg-primary)] text-foreground shadow-[0_0_0_1px_var(--border-strong)]',
  )
}

export const sidebarLogoutBtnClass = cn(
  'inline-flex shrink-0 items-center justify-center w-8 h-8 rounded-full border border-border bg-transparent text-foreground-muted cursor-pointer transition-[background,color,border-color] duration-140 ease-in-out ml-auto',
  'hover:border-[rgba(248,113,113,0.55)] hover:text-[#f87171] hover:bg-[rgba(239,68,68,0.1)]',
  '[&_svg]:w-[0.95rem] [&_svg]:h-[0.95rem]',
)
