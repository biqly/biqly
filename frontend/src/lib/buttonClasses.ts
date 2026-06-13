import { cn } from './cn'

export type ButtonVariant =
  | 'primary'
  | 'secondary'
  | 'ghost'
  | 'danger'
  | 'danger-outline'
  | 'back'
  | 'neutral'
  | 'destructive'

export type ButtonSize = 'sm' | 'md'

const buttonBase =
  'inline-flex items-center justify-center w-full min-h-[2.25rem] mt-2 px-4 py-2 border border-border-strong rounded-lg bg-card-raised text-foreground font-[family-name:var(--font-display,"Plus_Jakarta_Sans",sans-serif)] text-[0.8rem] font-semibold tracking-normal cursor-pointer shadow-[0_1px_2px_rgba(0,0,0,0.05)] transition-all duration-180 ease-[cubic-bezier(0.4,0,0.2,1)] hover:bg-[var(--control-hover-bg)] hover:border-[var(--control-hover-border)] hover:-translate-y-[0.5px] active:scale-[0.98] disabled:cursor-not-allowed disabled:opacity-45 disabled:transform-none! disabled:shadow-none!'

const variantClasses: Record<ButtonVariant, string> = {
  primary:
    'border-accent-strong bg-[linear-gradient(135deg,var(--accent)_0%,var(--accent-strong)_100%)] text-white shadow-[0_2px_8px_var(--accent-glow)] hover:bg-[linear-gradient(135deg,var(--accent-hover)_0%,var(--accent-strong)_100%)] hover:border-accent-hover hover:shadow-[0_4px_14px_var(--accent-glow)]',
  secondary:
    'border-border bg-card-raised text-foreground hover:bg-[color-mix(in_srgb,var(--accent)_10%,var(--bg-card-raised))] hover:border-border-strong',
  ghost:
    'border-transparent bg-transparent text-foreground-muted shadow-none hover:bg-card-raised hover:text-foreground hover:border-border',
  danger:
    'border-error bg-error text-white font-semibold hover:bg-[color-mix(in_srgb,var(--error)_90%,#000)] hover:border-[color-mix(in_srgb,var(--error)_90%,#000)]',
  'danger-outline':
    'border-[color-mix(in_srgb,var(--error)_40%,var(--border))] bg-transparent text-error shadow-none hover:bg-[color-mix(in_srgb,var(--error)_8%,transparent)] hover:border-error',
  back: 'inline-flex items-center gap-[0.35rem] w-auto mt-0 min-h-[1.85rem] rounded-[0.4rem] px-3 py-[0.3rem] text-[0.76rem] border-transparent bg-transparent text-foreground-muted shadow-none hover:bg-card-raised hover:text-foreground hover:border-border',
  neutral: 'bg-card text-foreground-muted shadow-none',
  destructive: 'bg-error text-white shadow-none',
}

const sizeClasses: Record<ButtonSize, string> = {
  md: '',
  sm: 'min-h-[1.85rem] rounded-[0.4rem] text-[0.76rem] px-3 py-[0.3rem]',
}

export function buttonClass(
  variant: ButtonVariant = 'primary',
  options?: { size?: ButtonSize; autoWidth?: boolean; className?: string },
): string {
  const size = options?.size ?? 'md'
  return cn(
    variant === 'back' ? variantClasses.back : buttonBase,
    variant !== 'back' && variantClasses[variant],
    sizeClasses[size],
    options?.autoWidth && 'w-auto mt-0',
    options?.className,
  )
}

export const rowActionsClass =
  'inline-flex gap-[0.4rem] items-center justify-end flex-nowrap [&_button]:w-auto [&_button]:mt-0'

export const removeBtnClass =
  'inline-flex items-center justify-center min-w-8 min-h-[2.25rem] mt-2 px-[0.55rem] py-[0.3rem] border border-[rgba(251,113,133,0.38)] rounded-lg bg-[rgba(251,113,133,0.14)] text-[#fecdd3] font-[family-name:var(--font-display,"Plus_Jakarta_Sans",sans-serif)] text-[0.8rem] font-semibold cursor-pointer transition-all duration-180 hover:bg-[rgba(251,113,133,0.24)] active:scale-[0.98] disabled:cursor-not-allowed disabled:opacity-45'

export const removeBtnCompactClass =
  'inline-flex items-center justify-center min-w-7 min-h-[1.85rem] mt-2 px-[0.42rem] py-1 border border-border-strong rounded-lg bg-card-raised text-foreground-muted text-[0.8rem] font-semibold cursor-pointer transition-all duration-180 hover:border-[rgba(251,113,133,0.42)] hover:bg-[rgba(251,113,133,0.12)] hover:text-[#fecdd3] active:scale-[0.98] disabled:cursor-not-allowed disabled:opacity-45'

export const addBtnClass =
  'inline-flex items-center justify-center w-full min-h-[2.25rem] mt-2 px-4 py-2 border border-dashed border-border-strong rounded-lg bg-transparent text-foreground-muted font-[family-name:var(--font-display,"Plus_Jakarta_Sans",sans-serif)] text-[0.8rem] font-semibold cursor-pointer transition-all duration-180 hover:border-foreground-muted hover:bg-card-raised hover:text-foreground active:scale-[0.98] disabled:cursor-not-allowed disabled:opacity-45'

export const iconBtnClass =
  'inline-flex items-center gap-[0.4rem] border-0 bg-transparent text-foreground cursor-pointer text-[0.88rem] py-[0.2rem] px-0 hover:text-accent'

/** Maps legacy BEM class strings to Tailwind for gradual migration. */
export function legacyButtonClass(className: string): string {
  const trimmed = className.trim()
  if (trimmed === 'icon-btn') {
    return iconBtnClass
  }
  if (trimmed === 'remove-btn') {
    return removeBtnClass
  }
  if (trimmed === 'remove-btn--compact') {
    return removeBtnCompactClass
  }
  if (trimmed === 'add-btn') {
    return addBtnClass
  }

  const parts = className.split(/\s+/).filter(Boolean)
  let variant: ButtonVariant = 'secondary'
  let size: ButtonSize = 'md'
  let extra = ''

  for (const part of parts) {
    if (part === 'btn') {
      continue
    }
    if (part === 'btn-sm') {
      size = 'sm'
    } else if (part === 'btn-primary') {
      variant = 'primary'
    } else if (part === 'btn-secondary') {
      variant = 'secondary'
    } else if (part === 'btn-ghost') {
      variant = 'ghost'
    } else if (part === 'btn-danger') {
      variant = 'danger'
    } else if (part === 'btn-danger-outline') {
      variant = 'danger-outline'
    } else if (part === 'btn-back') {
      variant = 'back'
    } else if (part === 'btn--neutral') {
      variant = 'neutral'
    } else if (part === 'btn--destructive') {
      variant = 'destructive'
    } else {
      extra = cn(extra, part)
    }
  }

  return buttonClass(variant, { size, className: extra })
}
