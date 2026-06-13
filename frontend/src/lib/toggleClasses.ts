import { cn } from './cn'

export function toggleGroupClass(...extra: (string | false | undefined)[]): string {
  return cn(
    'inline-flex shrink-0 border border-border-strong rounded-lg p-[0.2rem] bg-card-raised gap-[0.2rem]',
    ...extra,
  )
}

export const metricModeToggleClass = 'w-full mt-[0.35rem] mb-[0.15rem]'

export function toggleBtnClass(active: boolean, extra?: string): string {
  return cn(
    'flex-[1_1_auto] min-w-[4.5rem] px-[0.85rem] py-[0.4rem] bg-transparent border-0 rounded-[0.35rem] text-foreground-muted cursor-pointer font-[family-name:var(--font-display,"Plus_Jakarta_Sans",sans-serif)] text-[0.78rem] font-semibold leading-[1.2] transition-[background-color,color,box-shadow] duration-180 ease-[cubic-bezier(0.4,0,0.2,1)] disabled:opacity-45 disabled:cursor-not-allowed',
    !active && 'hover:bg-[var(--toggle-idle-hover-bg)] hover:text-foreground',
    active &&
      'bg-card text-foreground shadow-[0_1px_3px_rgba(0,0,0,0.12),0_0_0_1px_var(--border-strong)]',
    extra,
  )
}
