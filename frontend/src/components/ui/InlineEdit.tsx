import type { KeyboardEvent } from 'react'

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

  return (
    <td
      className={editing ? `${className} ${className}--editing` : className}
      onDoubleClick={onStart}
    >
      {editing ? (
        <textarea
          className="metadata-inline-field metadata-inline-field--fit-rows"
          title="Kaydetmek için Cmd/Ctrl+Enter veya dışarı tıklayın. İptal: Escape."
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
