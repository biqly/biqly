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

export const authLabelClass = 'text-caption font-medium text-foreground-muted'

export const authInputClass = cn(
  'w-full rounded-lg border border-border bg-input px-[14px] py-[10px]',
  'text-sm text-foreground transition-[border-color,box-shadow] duration-200',
  'placeholder:text-foreground-faint',
  'focus:border-accent focus:shadow-[0_0_0_3px_var(--control-focus-ring)] focus:outline-none',
  'disabled:cursor-not-allowed disabled:opacity-60',
)

export const authInputErrorClass = cn(
  'border-error focus:border-error',
  'focus:shadow-[0_0_0_3px_color-mix(in_srgb,var(--error)_22%,transparent)]',
)

export const authFieldHintClass = 'text-xs leading-snug'

export const authFieldHintErrorClass = cn(authFieldHintClass, 'text-error')

export const authFieldHintSuccessClass = cn(authFieldHintClass, 'text-success')

export const authIconBoxClass = cn(
  'mb-4 flex h-12 w-12 items-center justify-center rounded-xl',
  'bg-gradient-to-br from-accent to-accent-strong text-white shadow-[var(--accent-shadow-md)]',
)

export const authAvatarClass = cn(
  'flex h-9 w-9 items-center justify-center overflow-hidden rounded-full',
  'bg-gradient-to-br from-accent to-accent-strong text-sm font-semibold text-white',
)

export const authOAuthBtnClass = cn(
  'border-border bg-card text-foreground-muted hover:border-foreground-muted',
  'flex w-full cursor-pointer items-center justify-center gap-2.5 rounded-lg border px-4 py-2.5',
  'text-sm font-medium transition-colors duration-200 hover:bg-[var(--control-hover-bg)]',
  'dark:border-white/8 dark:bg-white/3 dark:text-foreground dark:hover:border-white/20 dark:hover:bg-white/8',
)

export const authSpinnerClass =
  'h-8 w-8 animate-spin rounded-full border-2 border-border border-t-accent'

// Delegate to the shared ui/Button primary styling so auth submit renders
// the same gradient-system look as <Button>. mt-0 (form uses gap-based
// layout, buttonBase's mt-2 would add unwanted margin) + gap-2 (icon spacing
// for spinner+text) override buttonBase via tailwind-merge.
export const authSubmitBtnClass = cn(buttonClass('primary'), 'mt-0 gap-2')

export const authCheckboxRowClass =
  'flex items-start gap-2.5 text-caption leading-snug text-foreground-muted'

export const authInlineLinkClass = 'font-medium text-accent no-underline hover:underline'

export const authLinkBtnClass = cn(
  'inline border-0 bg-transparent p-0 font-inherit text-accent underline',
  'cursor-pointer hover:text-accent-strong',
)

export const authBtnClass = buttonClass('primary')
