import type { KeyboardEvent } from 'react'
import clsx from 'clsx'
import { useT } from '../../i18n'

interface InlineEditProps {
  editing: boolean
  value: string
  placeholder: string
  rows?: number
  className?: string
  onStart: () => void
  onChange: (value: string) => void
  onSave: () => void
  onCancel: () => void
}

export function InlineEdit({
  editing,
  value,
  placeholder,
  rows = 1,
  className = 'metadata-desc-cell',
  onStart,
  onChange,
  onSave,
  onCancel,
}: InlineEditProps) {
  const t = useT()
  const handleKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key === 'Escape') {
      event.preventDefault()
      onCancel()
    }
    if (event.key === 'Enter' && (event.ctrlKey || event.metaKey)) {
      event.preventDefault()
      onSave()
    }
  }

  const handleDisplayKeyDown = (event: KeyboardEvent<HTMLTableCellElement>) => {
    if (event.key !== 'Enter' && event.key !== ' ') return
    event.preventDefault()
    onStart()
  }

  return (
    <td
      className={clsx(className, editing && `${className}--editing`)}
      onDoubleClick={onStart}
      onKeyDown={editing ? undefined : handleDisplayKeyDown}
      role={editing ? undefined : 'button'}
      tabIndex={editing ? undefined : 0}
      aria-label={editing ? undefined : value || placeholder}
    >
      {editing ? (
        <textarea
          className="metadata-inline-field metadata-inline-field--fit-rows"
          title={t('common.inline_edit_save_hint')}
          autoFocus
          rows={rows}
          value={value}
          onChange={(event) => onChange(event.target.value)}
          onBlur={onSave}
          onKeyDown={handleKeyDown}
        />
      ) : (
        <span style={{ color: value ? 'var(--text-primary)' : 'var(--text-secondary)', fontStyle: value ? 'normal' : 'italic' }}>
          {value || placeholder}
        </span>
      )}
    </td>
  )
}
