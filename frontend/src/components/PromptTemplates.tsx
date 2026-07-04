import { useCallback, useEffect, useMemo, useRef, useState } from 'react'

import { useApi } from '../hooks/useApi'
import { useConfirm } from '../hooks/useConfirm'
import type { Locale, TranslationKey } from '../i18n'
import { LOCALE_OPTIONS, localeLanguageTag, SUPPORTED_LOCALES, useLocale, useT } from '../i18n'
import { buttonClass } from '../lib/buttonClasses'
import {
  cardClass,
  cardHeaderRowClass,
  cardIntroClass,
  cardLeadClass,
  cardLeadSingleLineClass,
  cardSubtitleClass,
} from '../lib/cardClasses'
import { cn } from '../lib/cn'
import { formRowClass, legacyFormClass } from '../lib/formClasses'
import { legacyLayoutClass } from '../lib/layoutClasses'
import {
  promptEditorContainerClass,
  promptEditorTextareaClass,
  promptEditorUnderlayClass,
} from '../lib/promptEditorClasses'
import { formatDateTime } from '../utils/formatters'
import { ErrorAlert } from './ui/ErrorAlert'
import { LoadingScreen } from './ui/LoadingScreen'
import { Select } from './ui/Select'
const TEMPLATE_NAMES = [
  'system_rules',
  'output_format',
  'retry',
  'clarification',
  'prompt_layout',
] as const
type TemplateName = (typeof TEMPLATE_NAMES)[number]
type EditLocale = Locale

interface PromptTemplateRow {
  name: string
  locale: string
  version: number
  content: string
  is_active: boolean
  created_at: string
  updated_at: string
}

const nameLabelKeys: Record<TemplateName, TranslationKey> = {
  system_rules: 'prompt_templates.name_system_rules',
  output_format: 'prompt_templates.name_output_format',
  retry: 'prompt_templates.name_retry',
  clarification: 'prompt_templates.name_clarification',
  prompt_layout: 'prompt_templates.name_prompt_layout',
}

const TEMPLATE_PARAMS: Record<TemplateName, string[]> = {
  system_rules: [],
  output_format: [],
  clarification: [
    '{{.Question}}',
    '{{.FailureReason}}',
    '{{.ModelName}}',
    '{{.Dimensions}}',
    '{{.Metrics}}',
  ],
  retry: [
    '{{.OriginalPrompt}}',
    '{{.LastResponse}}',
    '{{.ValidationError}}',
    '{{.ValidationErrors}}',
  ],
  prompt_layout: [
    '{{.SystemRules}}',
    '{{.CurrentDateTime}}',
    '{{.ModelName}}',
    '{{.ModelLabel}}',
    '{{.ModelDescription}}',
    '{{.BaseTable}}',
    '{{.ModelSynonyms}}',
    '{{.Dimensions}}',
    '{{.Metrics}}',
    '{{.Note}}',
    '{{.Joins}}',
    '{{.FilterOperators}}',
    '{{.Glossary}}',
    '{{.DialectGuide}}',
    '{{.FailureExamples}}',
    '{{.PlanningSteps}}',
    '{{.Question}}',
    '{{.OutputFormat}}',
    '{{.SampleData}}',
    '{{.Examples}}',
    '{{.PriorTurns}}',
  ],
}

function highlightContent(text: string) {
  if (!text) {
    return null
  }
  const parts = text.split(/(\{\{.*?\}\})/g)
  return parts.map((part, idx) => {
    if (part.startsWith('{{') && part.endsWith('}}')) {
      const inner = part.slice(2, -2) // get inner part

      // Parse keywords if any
      let keyword = ''
      let rest = inner
      if (inner.startsWith('if ')) {
        keyword = 'if '
        rest = inner.substring(3)
      } else if (inner.startsWith('else')) {
        keyword = 'else'
        rest = inner.substring(4)
      } else if (inner.startsWith('end')) {
        keyword = 'end'
        rest = inner.substring(3)
      }

      return (
        <span key={idx} className="font-bold">
          <span className="text-foreground-faint">{'{{'}</span>
          {keyword && <span className="text-error font-bold">{keyword}</span>}
          <span className="text-accent font-bold">{rest}</span>
          <span className="text-foreground-faint">{'}}'}</span>
        </span>
      )
    }
    return <span key={idx}>{part}</span>
  })
}

// Editor drag handlers live at module scope so their branches don't inflate the
// PromptTemplates component's cyclomatic complexity.
function handleEditorDragOver(e: React.DragEvent<HTMLTextAreaElement>, dragging: boolean) {
  if (!dragging) {
    return
  }
  // Allow the drop and let the browser move the caret to the pointer so the
  // parameter lands where it is dropped.
  e.preventDefault()
  e.dataTransfer.dropEffect = 'copy'
}

function handleEditorDrop(
  e: React.DragEvent<HTMLTextAreaElement>,
  insert: (param: string) => void,
  setDragging: (dragging: boolean) => void,
) {
  const param = e.dataTransfer.getData('text/plain')
  setDragging(false)
  if (!param) {
    return
  }
  e.preventDefault()
  insert(param)
}

// EditorDropOverlay draws the drop affordance (ring + hint) while a parameter is
// being dragged. Kept as its own component so the conditional stays out of the
// PromptTemplates body.
function EditorDropOverlay({ active, label }: { active: boolean; label: string }) {
  if (!active) {
    return null
  }
  return (
    <div className="ring-accent/50 pointer-events-none absolute inset-0 z-10 rounded-lg ring-2">
      <span className="text-caption text-accent bg-accent/10 border-accent/30 absolute top-2 right-2 rounded-full border px-2 py-0.5 font-medium backdrop-blur-sm">
        {label}
      </span>
    </div>
  )
}

function ParameterPalette({
  params,
  onInsert,
  onDragStateChange,
  t,
}: {
  params: string[]
  onInsert: (param: string) => void
  onDragStateChange: (dragging: boolean) => void
  t: ReturnType<typeof useT>
}) {
  return (
    <div className="border-border mb-4 rounded-lg border bg-white/1.5 p-3">
      <div className="mb-2 flex items-baseline justify-between gap-2">
        <span className="text-caption text-foreground-muted font-semibold">
          {t('prompt_templates.available_params')}
        </span>
        <span className="text-2xs text-foreground-faint">
          {t('prompt_templates.param_drag_hint')}
        </span>
      </div>
      <div className="flex flex-wrap gap-1.5">
        {params.map((param) => (
          <button
            key={param}
            type="button"
            draggable
            onDragStart={(e) => {
              e.dataTransfer.setData('text/plain', param)
              e.dataTransfer.effectAllowed = 'copy'
              onDragStateChange(true)
            }}
            onDragEnd={() => onDragStateChange(false)}
            className="text-caption border-accent/20 bg-accent/5 text-accent hover:bg-accent/12 hover:border-accent/40 mt-0 inline-flex min-h-6 w-auto cursor-grab items-center justify-center gap-1 rounded border px-2 py-0.5 font-mono transition-colors active:cursor-grabbing"
            onClick={() => onInsert(param)}
          >
            <span aria-hidden className="text-accent/50 select-none">
              ⠿
            </span>
            {param}
          </button>
        ))}
      </div>
    </div>
  )
}

function VersionHistoryTable({
  versionHistory,
  languageTag,
  t,
}: {
  versionHistory: PromptTemplateRow[]
  languageTag: string
  t: ReturnType<typeof useT>
}) {
  return (
    <div className="mt-6">
      <h3 className="text-foreground mb-3 text-sm font-semibold">
        {t('prompt_templates.version_history')}
      </h3>
      <div className="border-border overflow-hidden rounded-lg border">
        <table className="w-full border-collapse text-sm">
          <thead>
            <tr className="text-2xs text-foreground-muted bg-white/2 text-left tracking-wide uppercase">
              <th className="px-3 py-2 font-semibold">{t('prompt_templates.col_version')}</th>
              <th className="px-3 py-2 font-semibold">{t('prompt_templates.col_status')}</th>
              <th className="px-3 py-2 font-semibold">{t('prompt_templates.col_updated')}</th>
              <th className="px-3 py-2 text-right font-semibold">
                {t('prompt_templates.col_chars')}
              </th>
            </tr>
          </thead>
          <tbody>
            {versionHistory.map((row) => (
              <tr
                key={`${row.name}:${row.locale}:${row.version}`}
                className={cn(
                  'border-border border-t',
                  row.is_active ? 'bg-accent/5' : 'hover:bg-white/2',
                )}
              >
                <td className="text-foreground px-3 py-2 font-mono">v{row.version}</td>
                <td className="px-3 py-2">
                  {row.is_active ? (
                    <span className="text-2xs bg-accent/10 text-accent border-accent/20 inline-flex items-center gap-1.5 rounded-full border px-2 py-0.5 font-medium">
                      <span className="bg-accent size-1.5 rounded-full" aria-hidden />
                      {t('prompt_templates.status_active')}
                    </span>
                  ) : (
                    <span className="text-2xs text-foreground-faint">
                      {t('prompt_templates.status_inactive')}
                    </span>
                  )}
                </td>
                <td className="text-foreground-muted px-3 py-2">
                  {formatDateTime(row.updated_at, languageTag)}
                </td>
                <td className="text-foreground-muted px-3 py-2 text-right font-mono tabular-nums">
                  {row.content.length.toLocaleString(languageTag)}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}

export default function PromptTemplates() {
  const t = useT()
  const [locale] = useLocale()
  const languageTag = localeLanguageTag(locale)
  const { get, putData, postData, loading, error } = useApi()
  const confirm = useConfirm()
  const [initLoading, setInitLoading] = useState(true)
  const [rows, setRows] = useState<PromptTemplateRow[]>([])
  // Default the editor to the locale the user is browsing in, not the app's
  // fallback locale.
  const [editLocale, setEditLocale] = useState<EditLocale>(locale)
  const [selectedName, setSelectedName] = useState<TemplateName>('system_rules')
  const [draft, setDraft] = useState('')
  const [dirty, setDirty] = useState(false)
  const [saveOk, setSaveOk] = useState<string | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)

  const textareaRef = useRef<HTMLTextAreaElement | null>(null)
  const [showSuggestions, setShowSuggestions] = useState(false)
  const [suggestions, setSuggestions] = useState<string[]>([])
  const [suggestionIndex, setSuggestionIndex] = useState(0)
  const [queryStartIdx, setQueryStartIdx] = useState(-1)
  const [draggingParam, setDraggingParam] = useState(false)

  const insertParameter = useCallback(
    (param: string, customStart?: number) => {
      const textarea = textareaRef.current
      if (!textarea) {
        return
      }

      const start = customStart ?? textarea.selectionStart
      const end = textarea.selectionEnd
      const text = textarea.value

      const newValue = text.substring(0, start) + param + text.substring(end)
      setDraft(newValue)
      setDirty(true)
      setSaveOk(null)
      setShowSuggestions(false)

      setTimeout(() => {
        textarea.focus()
        const newCursorPos = start + param.length
        textarea.setSelectionRange(newCursorPos, newCursorPos)
      }, 0)
    },
    [setDraft, setDirty, setSaveOk],
  )

  const checkSuggestions = (val: string, cursorIndex: number) => {
    const activeParams = TEMPLATE_PARAMS[selectedName]
    if (activeParams.length === 0) {
      setShowSuggestions(false)
      return
    }

    const lastOpen = val.lastIndexOf('{{', cursorIndex - 1)
    if (lastOpen === -1 || lastOpen < cursorIndex - 30) {
      setShowSuggestions(false)
      return
    }

    const textBetween = val.substring(lastOpen, cursorIndex)
    if (textBetween.includes('}}')) {
      setShowSuggestions(false)
      return
    }

    const query = textBetween
    setQueryStartIdx(lastOpen)

    const filtered = activeParams.filter((p) => {
      const paramLower = p.toLowerCase()
      const queryLower = query.toLowerCase()
      return (
        paramLower.includes(queryLower) ||
        p.replace(/[{}.]/g, '').toLowerCase().includes(query.replace(/[{}.]/g, '').toLowerCase())
      )
    })

    if (filtered.length > 0) {
      setSuggestions(filtered)
      setSuggestionIndex(0)
      setShowSuggestions(true)
    } else {
      setShowSuggestions(false)
    }
  }

  const handleTextareaChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const val = e.target.value
    setDraft(val)
    setDirty(true)
    setSaveOk(null)
    checkSuggestions(val, e.target.selectionStart)
  }

  const handleTextareaSelect = (e: React.SyntheticEvent<HTMLTextAreaElement>) => {
    const target = e.target as HTMLTextAreaElement
    checkSuggestions(target.value, target.selectionStart)
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if (!showSuggestions) {
      return
    }

    if (e.key === 'ArrowDown') {
      e.preventDefault()
      setSuggestionIndex((prev) => (prev + 1) % suggestions.length)
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setSuggestionIndex((prev) => (prev - 1 + suggestions.length) % suggestions.length)
    } else if (e.key === 'Enter' || e.key === 'Tab') {
      e.preventDefault()
      const selected = suggestions[suggestionIndex]
      if (selected && queryStartIdx !== -1) {
        insertParameter(selected, queryStartIdx)
      }
    } else if (e.key === 'Escape') {
      e.preventDefault()
      setShowSuggestions(false)
    }
  }

  const handleTextareaScroll = (e: React.UIEvent<HTMLTextAreaElement>) => {
    const textarea = e.currentTarget
    const underlay = textarea.parentElement?.querySelector('.prompt-editor-underlay')
    if (underlay) {
      underlay.scrollTop = textarea.scrollTop
      underlay.scrollLeft = textarea.scrollLeft
    }
  }

  useEffect(() => {
    const textarea = textareaRef.current
    if (textarea) {
      const underlay = textarea.parentElement?.querySelector('.prompt-editor-underlay')
      if (underlay) {
        underlay.scrollTop = textarea.scrollTop
        underlay.scrollLeft = textarea.scrollLeft
      }
    }
  }, [draft])

  const load = useCallback(async () => {
    setInitLoading(true)
    try {
      const data = await get<PromptTemplateRow[]>('/api/ai/prompt-templates')
      if (data) {
        setRows(data)
      }
    } finally {
      setInitLoading(false)
    }
  }, [get])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    void load()
  }, [load])

  const currentRow = useMemo(
    () => rows.find((r) => r.name === selectedName && r.locale === editLocale && r.is_active),
    [rows, selectedName, editLocale],
  )

  const versionHistory = useMemo(
    () => rows.filter((r) => r.name === selectedName && r.locale === editLocale),
    [rows, selectedName, editLocale],
  )

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setDraft(currentRow?.content ?? '')
    setDirty(false)
    setSaveOk(null)
  }, [currentRow?.name, currentRow?.locale, currentRow?.content, currentRow?.updated_at])

  const localeOptions = useMemo(
    () =>
      SUPPORTED_LOCALES.map((loc) => ({
        value: loc,
        label: LOCALE_OPTIONS[loc].label,
      })),
    [],
  )

  const templateOptions = useMemo(
    () =>
      TEMPLATE_NAMES.map((name) => ({
        value: name,
        label: t(nameLabelKeys[name]),
      })),
    [t],
  )

  const handleSave = async () => {
    setActionError(null)
    setSaveOk(null)
    const trimmed = draft.trim()
    if (!trimmed) {
      setActionError(t('prompt_templates.err_empty'))
      return
    }
    const ok = await putData(`/api/ai/prompt-templates/${selectedName}/${editLocale}`, {
      content: trimmed,
    })
    if (!ok) {
      return
    }
    setDirty(false)
    setSaveOk(t('prompt_templates.saved'))
    await load()
  }

  const handleRestore = async () => {
    const ok = await confirm({
      title: t('prompt_templates.confirm_restore'),
      variant: 'warning',
    })
    if (!ok) {
      return
    }
    setActionError(null)
    setSaveOk(null)
    const res = await postData('/api/ai/prompt-templates/restore', {
      name: selectedName,
      locale: editLocale,
    })
    if (!res) {
      return
    }
    setSaveOk(t('prompt_templates.restored'))
    await load()
  }

  const handleReseed = async () => {
    const ok = await confirm({
      title: t('prompt_templates.confirm_reseed'),
      variant: 'danger',
    })
    if (!ok) {
      return
    }
    setActionError(null)
    setSaveOk(null)
    const res = await postData('/api/ai/prompt-templates/reseed', {})
    if (!res) {
      return
    }
    setSaveOk(t('prompt_templates.reseeded'))
    await load()
  }

  const updatedLabel =
    currentRow?.updated_at != null ? formatDateTime(currentRow.updated_at, languageTag) : '—'

  if (initLoading && rows.length === 0) {
    return <LoadingScreen minHeight="300px" />
  }

  return (
    <div className={legacyLayoutClass('page-stack')}>
      {error && <ErrorAlert error={error} />}
      {actionError && <ErrorAlert error={actionError} />}
      {saveOk && <p className={cardSubtitleClass}>{saveOk}</p>}

      <div className={cardClass()}>
        <div className={cardIntroClass}>
          <div className={cardHeaderRowClass}>
            <h2>{t('prompt_templates.title')}</h2>
          </div>
          <p
            className={cn(cardLeadClass, cardLeadSingleLineClass)}
            title={t('prompt_templates.manage_hint')}
          >
            {t('prompt_templates.manage_hint')}
          </p>
        </div>

        <div className={formRowClass}>
          <label className={cn(legacyFormClass('form-field'), 'min-w-40')}>
            <span className={legacyFormClass('form-label')}>
              {t('prompt_templates.label_locale')}
            </span>
            <Select
              value={editLocale}
              options={localeOptions}
              onChange={(v) => setEditLocale(v)}
              size="sm"
            />
          </label>
          <label className={cn(legacyFormClass('form-field'), 'min-w-56 flex-1')}>
            <span className={legacyFormClass('form-label')}>
              {t('prompt_templates.label_section')}
            </span>
            <Select
              value={selectedName}
              options={templateOptions}
              onChange={(v) => setSelectedName(v)}
              size="sm"
            />
          </label>
        </div>

        <div className="mt-3 mb-4 flex flex-wrap items-center gap-2">
          <span className="text-caption border-border text-foreground-muted rounded border bg-white/2 px-2 py-0.5 font-medium">
            {t('prompt_templates.meta_updated', { date: updatedLabel })}
          </span>
          <span className="text-caption border-border text-foreground-muted rounded border bg-white/2 px-2 py-0.5 font-medium">
            {t('prompt_templates.meta_chars', { count: draft.length })}
          </span>
          <span className="text-caption bg-accent/8 border-accent/15 text-accent rounded border px-2 py-0.5 font-medium">
            {t('prompt_templates.meta_version', { version: currentRow?.version ?? '-' })}
          </span>
        </div>

        {TEMPLATE_PARAMS[selectedName].length > 0 && (
          <ParameterPalette
            params={TEMPLATE_PARAMS[selectedName]}
            onInsert={insertParameter}
            onDragStateChange={setDraggingParam}
            t={t}
          />
        )}

        <div className={cn(promptEditorContainerClass, 'rounded-lg transition-shadow')}>
          <pre className={promptEditorUnderlayClass}>{highlightContent(draft)}</pre>
          <textarea
            ref={textareaRef}
            className={promptEditorTextareaClass}
            rows={22}
            value={draft}
            onChange={handleTextareaChange}
            onSelect={handleTextareaSelect}
            onKeyDown={handleKeyDown}
            onScroll={handleTextareaScroll}
            onDragOver={(e) => handleEditorDragOver(e, draggingParam)}
            onDrop={(e) => handleEditorDrop(e, insertParameter, setDraggingParam)}
            spellCheck={false}
          />
          <EditorDropOverlay active={draggingParam} label={t('prompt_templates.param_drop_hint')} />
          {showSuggestions && suggestions.length > 0 && (
            <div className="autocomplete-dropdown bg-card-raised border-border-strong absolute bottom-5 left-5 z-10 max-h-37.5 w-65 overflow-y-auto rounded-lg border p-1 shadow-lg backdrop-blur-md">
              <div className="text-2xs text-foreground-muted border-border mb-1 border-b px-1.5 py-1">
                {t('prompt_templates.intellisense_hint')}
              </div>
              {suggestions.map((s, idx) => {
                const isSelected = idx === suggestionIndex
                return (
                  <div
                    key={s}
                    className={cn(
                      'text-caption cursor-pointer rounded px-2 py-1.5 font-mono',
                      isSelected
                        ? 'bg-accent text-white'
                        : 'text-foreground hover:bg-canvas-subtle',
                    )}
                    onClick={() => insertParameter(s, queryStartIdx)}
                    onMouseEnter={() => setSuggestionIndex(idx)}
                  >
                    {s}
                  </div>
                )
              })}
            </div>
          )}
        </div>

        <div className={cn(formRowClass, 'mt-4 gap-2')}>
          <button
            type="button"
            className={buttonClass('primary', { size: 'sm' })}
            disabled={loading || !dirty}
            onClick={() => void handleSave()}
          >
            {t('prompt_templates.save')}
          </button>
          <button
            type="button"
            className={buttonClass('secondary', { size: 'sm' })}
            disabled={loading}
            onClick={() => void handleRestore()}
          >
            {t('prompt_templates.restore_default')}
          </button>
          <button
            type="button"
            className={buttonClass('danger-outline', { size: 'sm' })}
            disabled={loading}
            onClick={() => void handleReseed()}
          >
            {t('prompt_templates.reseed_all')}
          </button>
        </div>

        {versionHistory.length > 0 && (
          <VersionHistoryTable versionHistory={versionHistory} languageTag={languageTag} t={t} />
        )}
      </div>
    </div>
  )
}
