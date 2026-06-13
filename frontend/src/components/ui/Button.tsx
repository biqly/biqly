import type { ButtonHTMLAttributes } from 'react'

import { buttonClass, type ButtonSize, type ButtonVariant } from '../../lib/buttonClasses'
import { cn } from '../../lib/cn'
import { adminBtnAutoWidthClass } from '../admin/adminClasses'

export type { ButtonSize, ButtonVariant }

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: ButtonVariant
  size?: ButtonSize
  autoWidth?: boolean
}

export function Button({
  variant = 'primary',
  size = 'md',
  autoWidth = false,
  type = 'button',
  className,
  children,
  ...rest
}: ButtonProps) {
  return (
    <button
      type={type}
      className={cn(
        buttonClass(variant, { size, autoWidth }),
        autoWidth && adminBtnAutoWidthClass,
        className,
      )}
      {...rest}
    >
      {children}
    </button>
  )
}
