import type { ButtonHTMLAttributes } from 'react'

export type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  /** Maps to the existing `btn btn-*` CSS family in index.css — no new styles. */
  variant?: ButtonVariant
  size?: 'sm' | 'md'
  autoWidth?: boolean
}

/**
 * Standard button (Faz 7.1, tasks/frontend-table-pagination-standardization.md).
 * Emits the exact `btn btn-*` class strings the screens already use, so
 * adopting it is markup-neutral. New code should use this instead of raw
 * className strings; existing call sites migrate opportunistically.
 * The separate `admin-btn-*` family is a distinct visual language and is
 * intentionally NOT covered here.
 */
export function Button({
  variant = 'primary',
  size = 'md',
  autoWidth = false,
  type = 'button',
  className,
  children,
  ...rest
}: ButtonProps) {
  const cls = [
    'btn',
    `btn-${variant}`,
    size === 'sm' ? 'btn-sm' : '',
    autoWidth ? 'btn-auto-width' : '',
    className ?? '',
  ]
    .filter(Boolean)
    .join(' ')

  return (
    <button type={type} className={cls} {...rest}>
      {children}
    </button>
  )
}
