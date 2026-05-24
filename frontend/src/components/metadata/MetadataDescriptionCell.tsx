import { InlineEdit } from '../ui/InlineEdit'
import type { MetadataEditingKind, MetadataEditingState } from './utils'
import { textareaRowsForDescription } from './utils'

interface MetadataDescriptionCellProps {
  kind: MetadataEditingKind
  entityId: string
  description: string | null
  editing: MetadataEditingState | null
  placeholder: string
  onStartEdit: () => void
  onChange: (value: string) => void
  onSave: () => void
  onCancel: () => void
}

export function MetadataDescriptionCell({
  kind,
  entityId,
  description,
  editing,
  placeholder,
  onStartEdit,
  onChange,
  onSave,
  onCancel,
}: MetadataDescriptionCellProps) {
  const isEditing = editing?.kind === kind && editing.id === entityId
  const displayValue = isEditing ? editing.value : description ?? ''
  return (
    <InlineEdit
      editing={isEditing}
      value={displayValue}
      placeholder={placeholder}
      rows={textareaRowsForDescription(displayValue)}
      onStart={onStartEdit}
      onChange={onChange}
      onSave={onSave}
      onCancel={onCancel}
    />
  )
}
