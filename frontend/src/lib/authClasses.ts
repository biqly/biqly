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

export const authBtnClass = buttonClass('primary')
