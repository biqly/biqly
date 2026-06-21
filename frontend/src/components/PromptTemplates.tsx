import { useCallback, useEffect, useMemo, useRef, useState } from 'react'

import { useApi } from '../hooks/useApi'
import { useConfirm } from '../hooks/useConfirm'
import type { Locale, TranslationKey } from '../i18n'
import { DEFAULT_LOCALE, LOCALE_OPTIONS, SUPPORTED_LOCALES, useT } from '../i18n'
import { legacyButtonClass } from '../lib/buttonClasses'
import { legacyCardClass } from '../lib/cardClasses'
import { cn } from '../lib/cn'
import { formRowClass, legacyFormClass } from '../lib/formClasses'
import { legacyLayoutClass } from '../lib/layoutClasses'
import {
  promptEditorContainerClass,
  promptEditorTextareaClass,
  promptEditorUnderlayClass,
} from '../lib/promptEditorClasses'
import { legacyTableClass } from '../lib/tableClasses'
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
        <span key={idx} style={{ fontWeight: 'bold' }}>
          <span style={{ color: 'var(--text-muted)' }}>{'{{'}</span>
          {keyword && <span style={{ color: 'var(--error)', fontWeight: 'bold' }}>{keyword}</span>}
          <span style={{ color: 'var(--accent)', fontWeight: 'bold' }}>{rest}</span>
          <span style={{ color: 'var(--text-muted)' }}>{'}}'}</span>
        </span>
      )
    }
    return <span key={idx}>{part}</span>
  })
}

export default function PromptTemplates() {
  const t = useT()
  const { get, putData, postData, loading, error } = useApi()
  const confirm = useConfirm()
  const [initLoading, setInitLoading] = useState(true)
  const [rows, setRows] = useState<PromptTemplateRow[]>([])
  const [editLocale, setEditLocale] = useState<EditLocale>(DEFAULT_LOCALE)
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

  const checkSuggestions = useCallback(
    (val: string, cursorIndex: number) => {
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
    },
    [selectedName],
  )

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
    currentRow?.updated_at != null ? new Date(currentRow.updated_at).toLocaleString() : '—'

  if (initLoading && rows.length === 0) {
    return <LoadingScreen minHeight="300px" />
  }

  return (
    <div className={legacyLayoutClass('page-stack')}>
      {error && <ErrorAlert error={error} />}
      {actionError && <ErrorAlert error={actionError} />}
      {saveOk && <p className={legacyCardClass('card-subtitle')}>{saveOk}</p>}

      <div className={legacyCardClass('card')}>
        <div className={legacyCardClass('card-intro')}>
          <div className={legacyCardClass('card-header-row')}>
            <h2>{t('prompt_templates.title')}</h2>
          </div>
          <p
            className={legacyCardClass('card-lead card-lead--single-line')}
            title={t('prompt_templates.manage_hint')}
          >
            {t('prompt_templates.manage_hint')}
          </p>
        </div>

        <div className={formRowClass}>
          <label className={legacyFormClass('form-field')} style={{ minWidth: '10rem' }}>
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
          <label className={legacyFormClass('form-field')} style={{ minWidth: '14rem', flex: 1 }}>
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

        <div
          style={{
            display: 'flex',
            flexWrap: 'wrap',
            alignItems: 'center',
            gap: '0.5rem',
            marginTop: '0.75rem',
            marginBottom: '1rem',
          }}
        >
          <span
            style={{
              fontSize: '0.72rem',
              fontWeight: 500,
              background: 'rgba(255, 255, 255, 0.03)',
              border: '1px solid var(--border)',
              borderRadius: '4px',
              padding: '0.15rem 0.5rem',
              color: 'var(--text-secondary)',
            }}
          >
            {t('prompt_templates.meta_updated', { date: updatedLabel })}
          </span>
          <span
            style={{
              fontSize: '0.72rem',
              fontWeight: 500,
              background: 'rgba(255, 255, 255, 0.03)',
              border: '1px solid var(--border)',
              borderRadius: '4px',
              padding: '0.15rem 0.5rem',
              color: 'var(--text-secondary)',
            }}
          >
            {t('prompt_templates.meta_chars', { count: draft.length })}
          </span>
          <span
            style={{
              fontSize: '0.72rem',
              fontWeight: 500,
              background: 'rgba(99, 102, 241, 0.08)',
              border: '1px solid rgba(99, 102, 241, 0.15)',
              borderRadius: '4px',
              padding: '0.15rem 0.5rem',
              color: 'var(--accent)',
            }}
          >
            {t('prompt_templates.meta_version', { version: currentRow?.version ?? '-' })}
          </span>
        </div>

        {/* Clickable parameter pills */}
        {TEMPLATE_PARAMS[selectedName].length > 0 && (
          <div
            style={{
              marginBottom: '1rem',
              padding: '0.75rem',
              background: 'rgba(255, 255, 255, 0.015)',
              border: '1px solid var(--border)',
              borderRadius: '0.5rem',
            }}
          >
            <div
              style={{
                fontSize: '0.75rem',
                fontWeight: 600,
                color: 'var(--text-secondary)',
                marginBottom: '0.5rem',
              }}
            >
              {t('prompt_templates.available_params')}
            </div>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.35rem' }}>
              {TEMPLATE_PARAMS[selectedName].map((param) => (
                <button
                  key={param}
                  type="button"
                  className={legacyButtonClass('btn btn-sm')}
                  style={{
                    width: 'auto',
                    marginTop: 0,
                    padding: '0.15rem 0.5rem',
                    minHeight: '1.5rem',
                    fontSize: '0.72rem',
                    fontFamily: 'var(--font-mono, monospace)',
                    borderRadius: '4px',
                    borderColor: 'rgba(99, 102, 241, 0.2)',
                    background: 'rgba(99, 102, 241, 0.03)',
                    color: 'var(--accent)',
                    display: 'inline-flex',
                    alignItems: 'center',
                    justifyContent: 'center',
                  }}
                  onClick={() => insertParameter(param)}
                >
                  {param}
                </button>
              ))}
            </div>
          </div>
        )}

        <div className={promptEditorContainerClass}>
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
            spellCheck={false}
          />
          {showSuggestions && suggestions.length > 0 && (
            <div
              className="autocomplete-dropdown"
              style={{
                position: 'absolute',
                bottom: '1.25rem',
                left: '1.25rem',
                background: 'var(--bg-card-raised)',
                border: '1px solid var(--border-strong)',
                borderRadius: '0.5rem',
                boxShadow:
                  'var(--shadow-lg, 0 10px 25px -5px rgba(0, 0, 0, 0.3), 0 8px 10px -6px rgba(0, 0, 0, 0.3))',
                zIndex: 10,
                maxHeight: '150px',
                overflowY: 'auto',
                width: '260px',
                padding: '0.25rem',
                backdropFilter: 'blur(12px)',
                WebkitBackdropFilter: 'blur(12px)',
              }}
            >
              <div
                style={{
                  fontSize: '0.68rem',
                  color: 'var(--text-secondary)',
                  padding: '0.25rem 0.4rem',
                  borderBottom: '1px solid var(--border)',
                  marginBottom: '0.25rem',
                }}
              >
                {t('prompt_templates.intellisense_hint')}
              </div>
              {suggestions.map((s, idx) => {
                const isSelected = idx === suggestionIndex
                return (
                  <div
                    key={s}
                    style={{
                      padding: '0.35rem 0.5rem',
                      borderRadius: '0.3rem',
                      cursor: 'pointer',
                      fontSize: '0.78rem',
                      fontFamily: 'var(--font-mono, monospace)',
                      color: isSelected ? '#ffffff' : 'var(--text-primary)',
                      background: isSelected ? 'var(--accent)' : 'transparent',
                    }}
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
            className={legacyButtonClass('btn btn-primary btn-sm')}
            disabled={loading || !dirty}
            onClick={() => void handleSave()}
          >
            {t('prompt_templates.save')}
          </button>
          <button
            type="button"
            className={legacyButtonClass('btn btn-sm')}
            disabled={loading}
            onClick={() => void handleRestore()}
          >
            {t('prompt_templates.restore_default')}
          </button>
          <button
            type="button"
            className={legacyButtonClass('btn btn-sm btn-danger-outline')}
            disabled={loading}
            onClick={() => void handleReseed()}
          >
            {t('prompt_templates.reseed_all')}
          </button>
        </div>

        {versionHistory.length > 0 && (
          <div style={{ marginTop: '1.5rem' }}>
            <h3 style={{ fontSize: '0.95rem', marginBottom: '0.5rem' }}>
              {t('prompt_templates.version_history')}
            </h3>
            <div className="table-wrap">
              <table className={legacyTableClass('results-table')}>
                <thead>
                  <tr>
                    <th>{t('prompt_templates.col_version')}</th>
                    <th>{t('prompt_templates.col_status')}</th>
                    <th>{t('prompt_templates.col_updated')}</th>
                    <th>{t('prompt_templates.col_chars')}</th>
                  </tr>
                </thead>
                <tbody>
                  {versionHistory.map((row) => (
                    <tr key={`${row.name}:${row.locale}:${row.version}`}>
                      <td>v{row.version}</td>
                      <td>
                        {row.is_active
                          ? t('prompt_templates.status_active')
                          : t('prompt_templates.status_inactive')}
                      </td>
                      <td>{new Date(row.updated_at).toLocaleString()}</td>
                      <td>{row.content.length}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
