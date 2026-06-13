import type { ReactNode } from 'react'
import { useId } from 'react'

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
 * tasks/frontend-table-pagination-standardization.md). Uses the existing
 * .form-field/.form-label/.input class family; replaces ad-hoc inline-styled
 * label+input pairs. Selects keep using ui/Select directly.
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
    <div className="form-field">
      <label className="form-label" htmlFor={id}>
        {label}
      </label>
      <input
        id={id}
        className="input"
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
        <p className="mt-1 text-[0.8rem] text-error" id={errorId} role="alert">
          {error}
        </p>
      ) : null}
    </div>
  )
}
