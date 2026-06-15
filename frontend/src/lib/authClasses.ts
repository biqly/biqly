import { buttonClass } from './buttonClasses'
import { cn } from './cn'
import { errorAlertClass } from './feedbackClasses'

/** Hook class — radial gradient stays in index.css */
export const authPageClass = 'auth-page'

export const authCardClass = cn(
  'w-full max-w-[440px] bg-card border border-border rounded-2xl p-8',
  'shadow-[0_10px_25px_-5px_rgba(0,0,0,0.05),0_8px_10px_-6px_rgba(0,0,0,0.05)]',
  'backdrop-blur-[10px] transition-[transform,box-shadow] duration-200 ease',
  '[data-theme=dark]:bg-[rgba(26,32,44,0.75)] [data-theme=dark]:border-white/[0.08]',
  '[data-theme=dark]:shadow-[0_20px_25px_-5px_rgba(0,0,0,0.3),0_10px_10px_-5px_rgba(0,0,0,0.3)]',
)

export const authHeaderClass = 'flex flex-col items-center gap-4 mb-6'

export const authLogoClass = 'flex items-center justify-center'

export const authTitleClass = cn(
  "font-[family-name:var(--font-display),'Plus Jakarta Sans',sans-serif]",
  'text-[1.35rem] font-extrabold tracking-[-0.025em] text-foreground m-0',
)

export const authErrorClass = errorAlertClass

export const authFormClass = 'flex flex-col gap-5'

export const authFieldClass = 'flex flex-col gap-1.5'

export const authLabelClass = 'text-[13px] font-medium text-foreground-muted'

export const authInputClass = cn(
  'w-full rounded-lg border border-border bg-input px-[14px] py-[10px]',
  'text-[14px] text-foreground transition-[border-color,box-shadow] duration-200',
  'placeholder:text-foreground-faint',
  'focus:border-accent focus:shadow-[0_0_0_3px_var(--control-focus-ring)] focus:outline-none',
  'disabled:cursor-not-allowed disabled:opacity-60',
)

export const authInputErrorClass = cn(
  'border-error focus:border-error',
  'focus:shadow-[0_0_0_3px_color-mix(in_srgb,var(--error)_22%,transparent)]',
)

export const authFieldHintClass = 'text-[12px] leading-snug'

export const authFieldHintErrorClass = cn(authFieldHintClass, 'text-error')

export const authFieldHintSuccessClass = cn(authFieldHintClass, 'text-success')

export const authSubmitBtnClass = cn(
  'flex w-full items-center justify-center gap-2 rounded-lg border-0',
  'bg-gradient-to-br from-accent to-[var(--accent-strong)] px-4 py-[11px]',
  'text-[14px] font-semibold text-white shadow-[0_4px_10px_rgba(99,102,241,0.2)]',
  'cursor-pointer transition-all duration-150 hover:-translate-y-px hover:opacity-95 active:translate-y-0',
  'disabled:cursor-not-allowed disabled:opacity-60 disabled:transform-none',
)

export const authCheckboxRowClass =
  'flex items-start gap-2.5 text-[13px] leading-snug text-foreground-muted'

export const authInlineLinkClass = 'font-medium text-accent no-underline hover:underline'

export const authLinkBtnClass = cn(
  'inline border-0 bg-transparent p-0 font-inherit text-accent underline',
  'cursor-pointer hover:text-accent-strong',
)

export const authBtnClass = buttonClass('primary')
