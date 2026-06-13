import { buttonClass } from './buttonClasses'
import { cn } from './cn'

export const lockedStateOverlayClass =
  'flex items-center justify-center py-12 px-6 min-h-[450px] w-full'

export const lockedStateCardClass = cn(
  'flex flex-col items-center text-center max-w-md w-full',
  'bg-card border border-border-strong rounded-2xl py-10 px-8 shadow-card',
  'backdrop-blur-[8px] animate-locked-card-appear',
)

export const lockedStateIconClass = cn(
  'flex items-center justify-center w-16 h-16 rounded-full mb-6',
  'bg-gradient-to-br from-[rgba(239,68,68,0.1)] to-[rgba(239,68,68,0.03)]',
  'text-error border border-[rgba(239,68,68,0.2)]',
  '[&_svg]:w-7 [&_svg]:h-7',
)

export const lockedStateTitleClass = cn(
  "font-[family-name:var(--font-display),'Plus Jakarta Sans',sans-serif]",
  'text-[1.4rem] font-extrabold text-foreground mb-3 tracking-[-0.025em]',
)

export const lockedStateDescClass = 'text-[0.875rem] text-foreground-muted leading-normal mb-6'

export const lockedStateAlertClass = 'w-full mb-5 text-left'

export const lockedStateSuccessAlertClass = cn(
  lockedStateAlertClass,
  'border border-[color-mix(in_srgb,var(--success)_22%,transparent)] rounded-lg',
  'bg-[color-mix(in_srgb,var(--success)_8%,transparent)] text-success',
  'px-[0.85rem] py-[0.7rem] text-[0.85rem]',
)

export const lockedStateBtnClass = cn(buttonClass('primary'), 'w-auto min-w-[12rem]')
