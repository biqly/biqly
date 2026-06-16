import { useEffect, useRef, useState } from 'react'

import { useConfirm } from '../../hooks/useConfirm'
import type { TFunction } from '../../i18n'
import { cn } from '../../lib/cn'
import type { Conversation } from '../../types/ai'
import {
  btnConvActionClass,
  convActionsClass,
  convEditInputClass,
  conversationItemClass,
  convItemContentClass,
  convTimeClass,
  convTitleClass,
} from './aiQueryClasses'

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

  const handleStartEdit = (e: React.SyntheticEvent) => {
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
    <div className={conversationItemClass(isActive)}>
      {isEditing ? (
        <div className="p-[0.65rem_0.8rem] w-full">
          <input
            ref={inputRef}
            value={editTitle}
            onChange={(e) => setEditTitle(e.target.value)}
            onBlur={handleSave}
            onKeyDown={handleKeyDown}
            onClick={(e) => e.stopPropagation()}
            className={convEditInputClass}
            placeholder={t('ai_query.rename_placeholder')}
            aria-label={t('ai_query.rename_placeholder')}
          />
        </div>
      ) : (
        <button
          type="button"
          className={cn(
            convItemContentClass,
            'w-full text-left p-[0.65rem_0.8rem] pr-16 bg-transparent border-0 cursor-pointer text-foreground rounded-[0.6rem] outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-1',
          )}
          onClick={(e) => {
            e.stopPropagation()
            onSelect()
          }}
          onDoubleClick={handleStartEdit}
          onKeyDown={(e) => {
            if (e.key === 'F2') {
              e.preventDefault()
              handleStartEdit(e)
            }
          }}
          aria-label={conv.title ?? t('ai_query.conv_current')}
        >
          <span className={convTitleClass}>{conv.title ?? t('ai_query.conv_current')}</span>
          <span className={convTimeClass}>
            {t('ai_query.conv_messages', { count: conv.messages.length })}
          </span>
        </button>
      )}
      {!isEditing && (
        <div className={convActionsClass}>
          <button
            type="button"
            className={btnConvActionClass}
            onClick={handleStartEdit}
            aria-label={t('ai_query.rename_btn')}
            title={t('ai_query.rename_btn')}
          >
            <svg width="15" height="15" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
              <path
                fill="currentColor"
                d="M3 17.25V21h3.75L17.81 9.94l-3.75-3.75L3 17.25Zm2.92 1.83-.01-.01L5.84 19h-.01v-.08Zm15.71-15a1.207 1.207 0 0 0 0-1.71l-.92-.92a1.207 1.207 0 0 0-1.71 0l-1.52 1.52 3.63 3.63 1.52-1.52Z"
              />
            </svg>
          </button>
          <button
            type="button"
            className={btnConvActionClass}
            onClick={(e) => {
              void handleDelete(e)
            }}
            aria-label={t('ai_query.delete_btn')}
            title={t('ai_query.delete_btn')}
          >
            <svg width="15" height="15" viewBox="0 0 24 24" aria-hidden="true" focusable="false">
              <path
                fill="currentColor"
                d="M9 3a1 1 0 0 0-1 1v1H5.5a1 1 0 0 0 0 2H6v12a2 2 0 0 0 2 2h8a2 2 0 0 0 2-2V7h.5a1 1 0 1 0 0-2H16V4a1 1 0 0 0-1-1H9Zm2 4a1 1 0 0 0-1 1v9a1 1 0 1 0 2 0V8a1 1 0 0 0-1-1Zm4 0a1 1 0 0 0-1 1v9a1 1 0 1 0 2 0V8a1 1 0 0 0-1-1ZM10 5h4V4h-4v1Z"
              />
            </svg>
          </button>
        </div>
      )}
    </div>
  )
}
