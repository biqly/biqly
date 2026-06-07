import { useEffect, useRef, useState } from 'react'

import { useConfirm } from '../../hooks/useConfirm'
import type { TFunction } from '../../i18n'
import type { Conversation } from '../../types/ai'

interface SidebarConversationItemProps {
  conv: Conversation
  isActive: boolean
  onSelect: () => void
  onRename: (id: string, newTitle: string) => void
  onDelete: (id: string) => void
  t: TFunction
}

export function SidebarConversationItem({
  conv,
  isActive,
  onSelect,
  onRename,
  onDelete,
  t,
}: SidebarConversationItemProps) {
  const [isEditing, setIsEditing] = useState(false)
  const [editTitle, setEditTitle] = useState(conv.title ?? '')
  const inputRef = useRef<HTMLInputElement>(null)
  const confirm = useConfirm()

  useEffect(() => {
    if (isEditing) {
      inputRef.current?.focus()
      inputRef.current?.select()
    }
  }, [isEditing])

  const handleStartEdit = (e: React.MouseEvent) => {
    e.stopPropagation()
    setEditTitle(conv.title ?? '')
    setIsEditing(true)
  }

  const handleSave = () => {
    setIsEditing(false)
    const trimmed = editTitle.trim()
    if (trimmed && trimmed !== conv.title) {
      onRename(conv.id, trimmed)
    }
  }

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') {
      handleSave()
    } else if (e.key === 'Escape') {
      setIsEditing(false)
    }
  }

  const handleDelete = async (e: React.MouseEvent) => {
    e.stopPropagation()
    const ok = await confirm({
      title: t('ai_query.delete_conv_confirm'),
      variant: 'danger',
    })
    if (ok) {
      onDelete(conv.id)
    }
  }

  return (
    <div className={`conversation-item ${isActive ? 'active' : ''}`} onClick={onSelect}>
      {isEditing ? (
        <input
          ref={inputRef}
          value={editTitle}
          onChange={(e) => setEditTitle(e.target.value)}
          onBlur={handleSave}
          onKeyDown={handleKeyDown}
          onClick={(e) => e.stopPropagation()}
          className="conv-edit-input"
          placeholder={t('ai_query.rename_placeholder')}
        />
      ) : (
        <div className="conv-item-content">
          <span className="conv-title" onDoubleClick={handleStartEdit}>
            {conv.title ?? t('ai_query.conv_current')}
          </span>
          <span className="conv-time">
            {t('ai_query.conv_messages', { count: conv.messages.length })}
          </span>
        </div>
      )}
      {!isEditing && (
        <div className="conv-actions">
          <button
            type="button"
            className="btn-conv-action edit-btn"
            onClick={handleStartEdit}
            title={t('ai_query.rename_btn')}
          >
            ✏️
          </button>
          <button
            type="button"
            className="btn-conv-action delete-btn"
            onClick={(e) => {
              void handleDelete(e)
            }}
            title={t('ai_query.delete_btn')}
          >
            🗑️
          </button>
        </div>
      )}
    </div>
  )
}
