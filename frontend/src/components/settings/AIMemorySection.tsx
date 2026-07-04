import { useCallback, useEffect, useState } from 'react'

import {
  createMemoryEntry,
  deleteMemoryEntry,
  listMemoryEntries,
  type MemoryEntry,
  updateMemoryEntry,
} from '../../api/aiMemory'
import { useToast } from '../../hooks/useToast'
import { useT } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
import { cardClass } from '../../lib/cardClasses'
import { friendlyErrorMessage } from '../../utils/error'
import { useAuth } from '../auth/AuthProvider'
import { LoadingOverlay } from '../ui/LoadingOverlay'

const MAX_CONTENT_LENGTH = 500

export function AIMemorySection() {
  const t = useT()
  const toast = useToast()
  const { accessToken } = useAuth()
  const [entries, setEntries] = useState<MemoryEntry[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [draft, setDraft] = useState('')
  const [editingID, setEditingID] = useState<string | null>(null)
  const [editingContent, setEditingContent] = useState('')

  const load = useCallback(async () => {
    setLoading(true)
    try {
      setEntries(await listMemoryEntries(accessToken ?? undefined))
    } catch (e) {
      toast.error(friendlyErrorMessage(t, e))
    } finally {
      setLoading(false)
    }
  }, [accessToken, toast, t])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load()
  }, [load])

  const handleAdd = async () => {
    const content = draft.trim()
    if (!content) {
      return
    }
    setSaving(true)
    try {
      await createMemoryEntry(content, accessToken ?? undefined)
      setDraft('')
      toast.success(t('settings.ai_memory.added'))
      await load()
    } catch (e) {
      toast.error(friendlyErrorMessage(t, e))
    } finally {
      setSaving(false)
    }
  }

  const handleSaveEdit = async () => {
    if (!editingID) {
      return
    }
    const content = editingContent.trim()
    if (!content) {
      return
    }
    setSaving(true)
    try {
      await updateMemoryEntry(editingID, content, accessToken ?? undefined)
      setEditingID(null)
      toast.success(t('settings.ai_memory.updated'))
      await load()
    } catch (e) {
      toast.error(friendlyErrorMessage(t, e))
    } finally {
      setSaving(false)
    }
  }

  const handleDelete = async (id: string) => {
    setSaving(true)
    try {
      await deleteMemoryEntry(id, accessToken ?? undefined)
      toast.success(t('settings.ai_memory.deleted'))
      await load()
    } catch (e) {
      toast.error(friendlyErrorMessage(t, e))
    } finally {
      setSaving(false)
    }
  }

  return (
    <section
      className={cardClass({ className: 'mb-0', elevated: true })}
      aria-labelledby="ai-memory-heading"
    >
      <div className="mb-4">
        <h2 id="ai-memory-heading" className="m-0">
          {t('settings.ai_memory.section')}
        </h2>
        <p className="text-foreground-muted mt-[0.35rem] mb-0 max-w-2xl text-[0.875rem] leading-[1.45]">
          {t('settings.ai_memory.hint')}
        </p>
      </div>

      <LoadingOverlay loading={loading}>
        <div className="flex flex-col gap-3">
          <div className="flex flex-wrap items-start gap-2">
            <input
              type="text"
              className="border-border bg-card-raised text-foreground min-w-60 flex-1 rounded-lg border px-3 py-2 text-[0.875rem]"
              value={draft}
              maxLength={MAX_CONTENT_LENGTH}
              placeholder={t('settings.ai_memory.placeholder')}
              aria-label={t('settings.ai_memory.placeholder')}
              onChange={(e) => {
                setDraft(e.target.value)
              }}
              onKeyDown={(e) => {
                if (e.key === 'Enter') {
                  void handleAdd()
                }
              }}
            />
            <button
              type="button"
              className={buttonClass('primary', { autoWidth: true })}
              disabled={saving || !draft.trim()}
              onClick={() => {
                void handleAdd()
              }}
            >
              {t('settings.ai_memory.add_btn')}
            </button>
          </div>

          {entries.length === 0 && !loading ? (
            <p className="text-foreground-muted m-0 text-[0.82rem]">
              {t('settings.ai_memory.empty')}
            </p>
          ) : (
            <ul className="custom-scrollbar m-0 flex max-h-64 list-none flex-col gap-2 overflow-y-auto p-0 pr-1">
              {entries.map((entry) => (
                <li
                  key={entry.id}
                  className="border-border bg-card-raised flex items-center gap-2 rounded-[10px] border px-3.5 py-2.5"
                >
                  {editingID === entry.id ? (
                    <>
                      <input
                        type="text"
                        className="border-border bg-card text-foreground flex-1 rounded-lg border px-2.5 py-1.5 text-[0.85rem]"
                        value={editingContent}
                        maxLength={MAX_CONTENT_LENGTH}
                        aria-label={t('settings.ai_memory.edit_aria')}
                        onChange={(e) => {
                          setEditingContent(e.target.value)
                        }}
                        onKeyDown={(e) => {
                          if (e.key === 'Enter') {
                            void handleSaveEdit()
                          }
                          if (e.key === 'Escape') {
                            setEditingID(null)
                          }
                        }}
                      />
                      <button
                        type="button"
                        className={buttonClass('primary', { size: 'sm', autoWidth: true })}
                        disabled={saving || !editingContent.trim()}
                        onClick={() => {
                          void handleSaveEdit()
                        }}
                      >
                        {t('common.save')}
                      </button>
                      <button
                        type="button"
                        className={buttonClass('ghost', { size: 'sm', autoWidth: true })}
                        onClick={() => {
                          setEditingID(null)
                        }}
                      >
                        {t('common.cancel')}
                      </button>
                    </>
                  ) : (
                    <>
                      <span className="text-foreground flex-1 text-[0.85rem] leading-[1.4] wrap-break-word">
                        {entry.content}
                      </span>
                      <button
                        type="button"
                        className={buttonClass('ghost', { size: 'sm', autoWidth: true })}
                        disabled={saving}
                        onClick={() => {
                          setEditingID(entry.id)
                          setEditingContent(entry.content)
                        }}
                      >
                        {t('common.edit')}
                      </button>
                      <button
                        type="button"
                        className={buttonClass('danger-outline', { size: 'sm', autoWidth: true })}
                        disabled={saving}
                        onClick={() => {
                          void handleDelete(entry.id)
                        }}
                      >
                        {t('common.delete')}
                      </button>
                    </>
                  )}
                </li>
              ))}
            </ul>
          )}
        </div>
      </LoadingOverlay>
    </section>
  )
}
