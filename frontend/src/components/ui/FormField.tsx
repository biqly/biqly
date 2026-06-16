import type { ReactNode } from 'react'
import { useId } from 'react'

import { legacyFeedbackClass } from '../../lib/feedbackClasses'
import { formControlClass, formFieldClass, formLabelClass } from '../../lib/formClasses'
interface FormFieldProps {
  label: ReactNode
  value: string
  onChange: (value: string) => void
  /** Field-level validation error; wired to the input via aria-describedby. */
  error?: string | null
  required?: boolean
  placeholder?: string
  type?: 'text' | 'email' | 'password' | 'url' | 'number'
  disabled?: boolean
  autoComplete?: string
}

/**
 * Labeled text input with accessible error wiring (Faz 7.2,
 * tasks/frontend-table-pagination-standardization.md). Uses formClasses
 * tokens; replaces ad-hoc inline-styled label+input pairs. Selects keep
 * using ui/Select directly.
 */
export function FormField({
  label,
  value,
  onChange,
  error,
  required = false,
  placeholder,
  type = 'text',
  disabled = false,
  autoComplete,
}: FormFieldProps) {
  const id = useId()
  const errorId = error ? `${id}-error` : undefined

  return (
    <div className={formFieldClass}>
      <label className={formLabelClass} htmlFor={id}>
        {label}
      </label>
      <input
        id={id}
        className={formControlClass}
        type={type}
        value={value}
        onChange={(e) => onChange(e.target.value)}
        required={required}
        placeholder={placeholder}
        disabled={disabled}
        autoComplete={autoComplete}
        aria-invalid={error ? true : undefined}
        aria-describedby={errorId}
      />
      {error ? (
        <p
          className={legacyFeedbackClass('text-error mt-1 text-[0.8rem]')}
          id={errorId}
          role="alert"
        >
          {error}
        </p>
      ) : null}
    </div>
  )
}
