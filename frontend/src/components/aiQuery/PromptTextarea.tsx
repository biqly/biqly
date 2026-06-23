import { useCallback, useMemo, useRef, useState } from 'react'

import type { CatalogEntry, CatalogEntryType } from '../../hooks/useSemanticCatalog'
import type { TranslationKey } from '../../i18n'
import { cn } from '../../lib/cn'
import {
  chatComposerInputClass,
  mentionGroupClass,
  mentionGroupLabelClass,
  mentionItemActiveClass,
  mentionItemBaseClass,
  mentionItemHintClass,
  mentionItemLabelClass,
  mentionItemTypeClass,
  mentionListClass,
  mentionWrapClass,
} from './aiQueryClasses'
import { type ActiveMention, findActiveMention, score } from './mentionUtils'

interface PromptTextareaProps {
  value: string
  onChange: (value: string) => void
  onSubmit: () => void
  onAbort: () => void
  disabled: boolean
  loading: boolean
  placeholder: string
  items: CatalogEntry[]
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
}

const TYPE_LABEL_KEY: Record<CatalogEntryType, TranslationKey> = {
  dimension: 'ai_query.mention_type_dimension',
  metric: 'ai_query.mention_type_metric',
  table: 'ai_query.mention_type_table',
}

export function PromptTextarea({
  value,
  onChange,
  onSubmit,
  onAbort,
  disabled,
  loading,
  placeholder,
  items,
  t,
}: PromptTextareaProps) {
  const ref = useRef<HTMLTextAreaElement | null>(null)
  const [mention, setMention] = useState<ActiveMention | null>(null)
  const [activeIdx, setActiveIdx] = useState(0)

  const results = useMemo(() => {
    if (!mention) {
      return [] as { entry: CatalogEntry; s: number }[]
    }
    const q = mention.query.trim()
    const scored = items
      .map((entry) => ({ entry, s: score(entry, q) }))
      .filter((r) => r.s > 0)
      .sort((a, b) => b.s - a.s || a.entry.label.localeCompare(b.entry.label))
    return scored.slice(0, 8)
  }, [mention, items])

  const recomputeMention = useCallback(() => {
    const el = ref.current
    if (!el) {
      return
    }
    setMention(findActiveMention(value, el.selectionStart))
    setActiveIdx(0)
  }, [value])

  const selectEntry = useCallback(
    (entry: CatalogEntry) => {
      const el = ref.current
      const cursor = el ? el.selectionStart : value.length
      const active = mention ?? findActiveMention(value, cursor)
      if (!active) {
        return
      }
      const insert = `${entry.name} `
      const next = value.slice(0, active.at) + insert + value.slice(cursor)
      onChange(next)
      setMention(null)
      const caret = active.at + insert.length
      requestAnimationFrame(() => {
        el?.focus()
        el?.setSelectionRange(caret, caret)
      })
    },
    [mention, value, onChange],
  )

  const onKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (mention && results.length > 0) {
      if (e.key === 'ArrowDown') {
        e.preventDefault()
        setActiveIdx((i) => (i + 1) % results.length)
        return
      }
      if (e.key === 'ArrowUp') {
        e.preventDefault()
        setActiveIdx((i) => (i - 1 + results.length) % results.length)
        return
      }
      if (e.key === 'Enter' || e.key === 'Tab') {
        e.preventDefault()
        const picked = results[activeIdx]?.entry
        if (picked) {
          void selectEntry(picked)
        }
        return
      }
      if (e.key === 'Escape') {
        e.preventDefault()
        setMention(null)
        return
      }
    }
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault()
      if (!loading && value.trim()) {
        onSubmit()
      }
      return
    }
    if (e.key === 'Escape' && loading) {
      onAbort()
    }
  }

  const grouped = useMemo(() => {
    const map = new Map<string, CatalogEntry[]>()
    for (const r of results) {
      const arr = map.get(r.entry.group) ?? []
      arr.push(r.entry)
      map.set(r.entry.group, arr)
    }
    return [...map.entries()]
  }, [results])

  // Flat index for keyboard nav across grouped results (matches DOM order).
  const flatEntries = useMemo(() => results.map((r) => r.entry), [results])

  const showPopup = mention && results.length > 0

  return (
    <div className={mentionWrapClass}>
      <textarea
        ref={ref}
        id="ai-question"
        className={chatComposerInputClass}
        value={value}
        onChange={(e) => {
          onChange(e.target.value)
          requestAnimationFrame(recomputeMention)
        }}
        onKeyUp={recomputeMention}
        onClick={recomputeMention}
        onBlur={() => window.setTimeout(() => setMention(null), 120)}
        onKeyDown={onKeyDown}
        placeholder={placeholder}
        rows={2}
        autoComplete="off"
        disabled={disabled}
      />
      {showPopup && (
        <div className={mentionListClass} role="listbox" aria-label={t('ai_query.mention_aria')}>
          {grouped.map(([group, entries]) => (
            <div key={group} className={mentionGroupClass}>
              <div className={mentionGroupLabelClass}>{group}</div>
              {entries.map((entry) => {
                const flatIdx = flatEntries.indexOf(entry)
                const active = flatIdx === activeIdx
                return (
                  <button
                    key={`${entry.type}:${entry.name}`}
                    type="button"
                    role="option"
                    aria-selected={active}
                    className={cn(mentionItemBaseClass, active && mentionItemActiveClass)}
                    onMouseDown={(e) => {
                      // Prevent the textarea blur before the click registers.
                      e.preventDefault()
                      void selectEntry(entry)
                    }}
                    onMouseEnter={() => setActiveIdx(flatIdx)}
                  >
                    <span className={mentionItemTypeClass}>{t(TYPE_LABEL_KEY[entry.type])}</span>
                    <span className="min-w-0 flex-1">
                      <span className={mentionItemLabelClass}>{entry.label}</span>
                      {entry.hint && <span className={mentionItemHintClass}>{entry.hint}</span>}
                    </span>
                    <span
                      className={cn(
                        'text-[0.72rem]',
                        active ? 'text-white' : 'text-foreground-faint',
                      )}
                    >
                      {entry.name}
                    </span>
                  </button>
                )
              })}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
