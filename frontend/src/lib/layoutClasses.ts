import { cn } from './cn'

export const pageStackClass = 'flex flex-col gap-5 min-w-0'

export const mainClass = cn(
  'w-[min(100%,1800px)] min-w-0 mx-auto',
  'px-[clamp(1.25rem,4vw,2.75rem)] pt-9 pb-14',
  'focus:outline-none',
  'max-[980px]:pt-14',
  'max-[680px]:px-[0.9rem]',
)

export const skipLinkClass = cn(
  'fixed top-4 left-4 z-[100] -translate-y-[180%]',
  'rounded-full bg-accent text-white font-bold px-4 py-[0.65rem]',
  'transition-transform duration-[160ms] ease-in',
  'focus-visible:translate-y-0',
)

export const pageHeaderClass = cn(
  'grid gap-[0.35rem] mb-8',
  '[&>p]:text-accent [&>p]:font-[family-name:var(--font-display)]',
  '[&>p]:text-[0.72rem] [&>p]:font-bold [&>p]:tracking-[0.12em] [&>p]:uppercase',
  '[&_h1]:text-foreground [&_h1]:font-[family-name:var(--font-display)]',
  '[&_h1]:text-[clamp(1.8rem,2.4vw,2.1rem)] [&_h1]:font-extrabold',
  '[&_h1]:tracking-[-0.035em] [&_h1]:leading-[1.12] [&_h1]:text-balance',
  '[&>div]:flex [&>div]:flex-col [&>div]:gap-[0.6rem]',
  '[&_span]:block [&_span]:max-w-[min(96rem,100%)] [&_span]:mt-0',
  '[&_span]:text-foreground-muted [&_span]:text-[0.9rem] [&_span]:leading-[1.45] [&_span]:text-pretty',
)

export const navLinkClass = cn(
  'group flex items-center gap-[0.7rem] min-w-0 rounded-lg text-foreground-muted py-2 px-3',
  'font-[family-name:var(--font-display)] text-[0.86rem] font-medium',
  'border-l-2 border-transparent',
  'transition-all duration-180 ease-in-out',
  'hover:bg-card hover:text-foreground hover:translate-x-[2px]',
  'aria-[current=page]:bg-(--nav-link-active-bg) aria-[current=page]:border-l-accent',
  'aria-[current=page]:text-foreground aria-[current=page]:font-semibold',
  'aria-[current=page]:shadow-[inset_0_1px_0_rgba(255,255,255,0.01)]',
)

export const navLinkIconClass = cn(
  'inline-flex items-center justify-center w-[1.05rem] h-[1.05rem] shrink-0 text-foreground-muted',
  'transition-all duration-180 group-hover:text-accent group-hover:scale-105',
  'group-aria-[current=page]:text-accent',
)

export const settingsPrefsCardClass = 'mb-0'

export const settingsFootnoteClass = cn(
  'm-0 px-[0.15rem] max-w-none',
  'text-[0.74rem] leading-[1.45] text-foreground-faint',
)

export const mobileNavScrimClass = cn(
  'hidden fixed inset-0 z-40',
  'bg-[var(--mobile-nav-scrim)] backdrop-blur-[length:var(--mobile-nav-scrim-blur)]',
  '[-webkit-backdrop-filter:blur(var(--mobile-nav-scrim-blur))]',
)

export const mobileNavScrimVisibleClass = 'max-[980px]:block'

export function mobileNavSidebarClass(open: boolean): string {
  return cn(
    'sticky top-0 flex flex-col gap-[0.85rem] h-screen border-r border-border bg-bg-secondary py-6 px-4 min-w-0',
    'max-[980px]:fixed max-[980px]:left-0 max-[980px]:top-0 max-[980px]:z-50',
    'max-[980px]:w-[min(18rem,86vw)] max-[980px]:border-r max-[980px]:border-[color:var(--mobile-nav-panel-edge)]',
    'max-[980px]:bg-[var(--mobile-nav-panel-bg)] max-[980px]:shadow-[var(--mobile-nav-panel-shadow)]',
    'max-[980px]:transition-transform max-[980px]:duration-200 motion-reduce:transition-none',
    open ? 'max-[980px]:translate-x-0' : 'max-[980px]:translate-x-[-105%]',
  )
}

export function mobileNavToggleClass(open: boolean): string {
  return cn(
    'hidden max-[980px]:inline-flex fixed top-3 z-60 w-10 h-10 items-center justify-center rounded-[0.65rem] text-[1.2rem] cursor-pointer',
    'focus-visible:outline-2 focus-visible:outline-accent focus-visible:outline-offset-2 transition-all duration-200',
    open
      ? 'left-[calc(min(18rem,86vw)-3.25rem)] bg-[var(--mobile-nav-toggle-open-bg)] border border-[color:var(--mobile-nav-toggle-open-border)] text-foreground shadow-[var(--mobile-nav-toggle-open-shadow)] hover:bg-[var(--mobile-nav-toggle-open-hover-bg)]'
      : 'left-3 bg-bg-secondary text-foreground border border-border shadow-[0_4px_14px_rgba(0,0,0,0.25)]',
  )
}

/** Maps legacy layout class strings to Tailwind for gradual migration. */
export function legacyLayoutClass(className: string): string {
  const parts = className.split(/\s+/).filter(Boolean)
  let extra = ''

  for (const part of parts) {
    if (part === 'page-stack') {
      extra = cn(extra, pageStackClass)
    } else if (part === 'main') {
      extra = cn(extra, mainClass)
    } else if (part === 'page-header') {
      extra = cn(extra, pageHeaderClass)
    } else if (part === 'skip-link') {
      extra = cn(extra, skipLinkClass)
    } else if (part === 'settings-prefs-card') {
      extra = cn(extra, settingsPrefsCardClass)
    } else if (part === 'settings-footnote') {
      extra = cn(extra, settingsFootnoteClass)
    } else {
      extra = cn(extra, part)
    }
  }

  return extra
}
