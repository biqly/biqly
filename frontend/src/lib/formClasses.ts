import { cn } from './cn'

/** Shared text/select/textarea surface (not checkbox/radio). */
export const formControlClass = cn(
  'w-full border border-border rounded-lg bg-card-raised text-foreground text-[0.82rem] leading-[1.4] px-3 py-2',
  'transition-all duration-180 ease-[cubic-bezier(0.4,0,0.2,1)] shadow-[inset_0_1px_2px_rgba(0,0,0,0.08)]',
  'focus-visible:border-[var(--control-focus-border)] focus-visible:shadow-[0_0_0_1px_var(--bg-primary),0_0_0_3px_var(--control-focus-ring)] focus-visible:outline-none',
)

const formControlDescendant = cn(
  '[&_select]:w-full',
  '[&_input:not([type=checkbox]):not([type=radio])]:w-full',
  '[&_textarea]:w-full',
  '[&_select]:border [&_select]:border-border [&_select]:rounded-lg [&_select]:bg-card-raised [&_select]:text-foreground',
  '[&_select]:text-[0.82rem] [&_select]:leading-[1.4] [&_select]:px-3 [&_select]:py-2',
  '[&_input:not([type=checkbox]):not([type=radio])]:border [&_input:not([type=checkbox]):not([type=radio])]:border-border',
  '[&_input:not([type=checkbox]):not([type=radio])]:rounded-lg [&_input:not([type=checkbox]):not([type=radio])]:bg-card-raised',
  '[&_input:not([type=checkbox]):not([type=radio])]:text-foreground [&_input:not([type=checkbox]):not([type=radio])]:text-[0.82rem]',
  '[&_input:not([type=checkbox]):not([type=radio])]:leading-[1.4] [&_input:not([type=checkbox]):not([type=radio])]:px-3',
  '[&_input:not([type=checkbox]):not([type=radio])]:py-2',
  '[&_textarea]:border [&_textarea]:border-border [&_textarea]:rounded-lg [&_textarea]:bg-card-raised',
  '[&_textarea]:text-foreground [&_textarea]:text-[0.82rem] [&_textarea]:leading-[1.4] [&_textarea]:px-3 [&_textarea]:py-2',
  '[&_select]:transition-all [&_select]:duration-180 [&_select]:ease-[cubic-bezier(0.4,0,0.2,1)]',
  '[&_input:not([type=checkbox]):not([type=radio])]:transition-all',
  '[&_input:not([type=checkbox]):not([type=radio])]:duration-180',
  '[&_input:not([type=checkbox]):not([type=radio])]:ease-[cubic-bezier(0.4,0,0.2,1)]',
  '[&_textarea]:transition-all [&_textarea]:duration-180 [&_textarea]:ease-[cubic-bezier(0.4,0,0.2,1)]',
  '[&_select]:shadow-[inset_0_1px_2px_rgba(0,0,0,0.08)]',
  '[&_input:not([type=checkbox]):not([type=radio])]:shadow-[inset_0_1px_2px_rgba(0,0,0,0.08)]',
  '[&_textarea]:shadow-[inset_0_1px_2px_rgba(0,0,0,0.08)]',
  '[&_select]:focus-visible:border-[var(--control-focus-border)]',
  '[&_select]:focus-visible:shadow-[0_0_0_1px_var(--bg-primary),0_0_0_3px_var(--control-focus-ring)]',
  '[&_select]:focus-visible:outline-none',
  '[&_input:not([type=checkbox]):not([type=radio])]:focus-visible:border-[var(--control-focus-border)]',
  '[&_input:not([type=checkbox]):not([type=radio])]:focus-visible:shadow-[0_0_0_1px_var(--bg-primary),0_0_0_3px_var(--control-focus-ring)]',
  '[&_input:not([type=checkbox]):not([type=radio])]:focus-visible:outline-none',
  '[&_textarea]:focus-visible:border-[var(--control-focus-border)]',
  '[&_textarea]:focus-visible:shadow-[0_0_0_1px_var(--bg-primary),0_0_0_3px_var(--control-focus-ring)]',
  '[&_textarea]:focus-visible:outline-none',
)

const formLabelDescendant = cn(
  '[&_label]:block [&_label]:mb-[0.45rem] [&_label]:text-foreground-muted',
  '[&_label]:font-[family-name:var(--font-display)] [&_label]:text-[0.8rem] [&_label]:font-semibold',
)

export const formLabelClass = cn(
  'block mb-[0.45rem] text-foreground-muted font-[family-name:var(--font-display)] text-[0.8rem] font-semibold',
)

export const formHintClass = 'block mt-2 text-foreground-faint text-[0.76rem]'

export const formHintWarningClass = cn(formHintClass, 'text-warning')

/** Horizontal field/action row (legacy `form-row`; never had CSS — layout via flex). */
export const formRowClass = cn('flex min-w-0 flex-wrap items-end gap-4')

/** Vertical stack of form sections (legacy `form-stack`). */
export const formStackClass = cn('flex min-w-0 flex-col gap-[0.85rem]')

export const formTextareaClass = cn(formControlClass, 'min-h-[7.5rem] resize-y')

/** Standard vertical field group (label + control + hint). */
export const formGroupClass = cn(
  'form-group min-w-0 mb-[1.15rem]',
  formLabelDescendant,
  formControlDescendant,
  '[&>.ui-select]:mt-0',
  '[&>.ui-multiselect--inline]:mt-[0.35rem]',
  '[&_textarea]:min-h-[7.5rem] [&_textarea]:resize-y',
  '[&_small]:block [&_small]:mt-2 [&_small]:text-foreground-faint [&_small]:text-[0.76rem]',
)

/** Filter/toolbar field wrapper (label + control, no group label margin). */
export const formFieldClass = cn('form-field min-w-0', formControlDescendant)

/** Modeling toolbar / side panel compact groups. */
export const modelingFormGroupClass = cn(
  formGroupClass,
  'flex flex-col gap-[0.38rem] mb-0',
  '[&_label]:text-foreground-muted [&_label]:text-[0.78rem] [&_label]:font-bold [&_label]:mb-0',
)

/** Maps legacy BEM form class strings to Tailwind for gradual migration. */
export function legacyFormClass(className: string): string {
  const parts = className.split(/\s+/).filter(Boolean)
  let extra = ''

  for (const part of parts) {
    if (part === 'form-group') {
      continue
    }
    if (part === 'form-field') {
      continue
    }
    if (part === 'form-label') {
      extra = cn(extra, formLabelClass)
    } else if (part === 'form-row') {
      extra = cn(extra, formRowClass)
    } else if (part === 'form-stack') {
      extra = cn(extra, formStackClass)
    } else if (part === 'form-hint') {
      extra = cn(extra, formHintClass)
    } else if (part === 'form-hint--warning') {
      extra = cn(extra, formHintWarningClass)
    } else if (part === 'input') {
      extra = cn(extra, formControlClass)
    } else {
      extra = cn(extra, part)
    }
  }

  if (parts.includes('form-group')) {
    return cn(formGroupClass, extra)
  }
  if (parts.includes('form-field')) {
    return cn(formFieldClass, extra)
  }
  if (parts.includes('form-label')) {
    return cn(formLabelClass, extra)
  }
  if (parts.includes('input')) {
    return cn(formControlClass, extra)
  }

  return extra
}
