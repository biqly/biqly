import type { KeyboardEvent } from 'react'

import { useAutofocus } from '../../hooks/useAutofocus'
import { useT } from '../../i18n'
import { cn } from '../../lib/cn'
import {
  metadataDescCellClass,
  metadataDescDisplayPlaceholderClass,
  metadataDescDisplayValueClass,
  metadataInlineFieldFitRowsClass,
} from '../../lib/tableClasses'

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
  className,
  onStart,
  onChange,
  onSave,
  onCancel,
}: InlineEditProps) {
  const t = useT()
  const editInputRef = useAutofocus<HTMLTextAreaElement>(editing)
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
    if (event.key !== 'Enter' && event.key !== ' ') {
      return
    }
    event.preventDefault()
    onStart()
  }

  return (
    <td
      className={cn(metadataDescCellClass(editing), className)}
      onDoubleClick={onStart}
      onKeyDown={editing ? undefined : handleDisplayKeyDown}
      role={editing ? undefined : 'button'}
      tabIndex={editing ? undefined : 0}
      aria-label={editing ? undefined : value || placeholder}
    >
      {editing ? (
        <textarea
          ref={editInputRef}
          className={metadataInlineFieldFitRowsClass}
          title={t('common.inline_edit_save_hint')}
          rows={rows}
          value={value}
          onChange={(event) => onChange(event.target.value)}
          onBlur={onSave}
          onKeyDown={handleKeyDown}
        />
      ) : (
        <span
          className={value ? metadataDescDisplayValueClass : metadataDescDisplayPlaceholderClass}
        >
          {value || placeholder}
        </span>
      )}
    </td>
  )
}
