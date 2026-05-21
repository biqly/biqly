import { useCallback, useEffect, useMemo, useState } from 'react'
import { useApi } from '../hooks/useApi'
import { useT } from '../i18n'
import type { TranslationKey } from '../i18n'
import { ErrorAlert } from './ui/ErrorAlert'
import { Select } from './ui/Select'

const TEMPLATE_NAMES = ['system_rules', 'output_format'] as const
type TemplateName = (typeof TEMPLATE_NAMES)[number]
type EditLocale = 'en' | 'tr'

interface PromptTemplateRow {
  name: string
  locale: string
  content: string
  updated_at: string
}

const nameLabelKeys: Record<TemplateName, TranslationKey> = {
  system_rules: 'prompt_templates.name_system_rules',
  output_format: 'prompt_templates.name_output_format',
}

export default function PromptTemplates() {
  const t = useT()
  const { get, putData, postData, loading, error } = useApi()
  const [rows, setRows] = useState<PromptTemplateRow[]>([])
  const [editLocale, setEditLocale] = useState<EditLocale>('tr')
  const [selectedName, setSelectedName] = useState<TemplateName>('system_rules')
  const [draft, setDraft] = useState('')
  const [dirty, setDirty] = useState(false)
  const [saveOk, setSaveOk] = useState<string | null>(null)
  const [actionError, setActionError] = useState<string | null>(null)

  const load = useCallback(async () => {
    const data = await get<PromptTemplateRow[]>('/api/ai/prompt-templates')
    if (data) setRows(data)
  }, [get])

  useEffect(() => {
    void load()
  }, [load])

  const currentRow = useMemo(
    () => rows.find((r) => r.name === selectedName && r.locale === editLocale),
    [rows, selectedName, editLocale],
  )

  useEffect(() => {
    setDraft(currentRow?.content ?? '')
    setDirty(false)
    setSaveOk(null)
  }, [currentRow?.name, currentRow?.locale, currentRow?.content, currentRow?.updated_at])

  const localeOptions = useMemo(
    () => [
      { value: 'tr', label: t('prompt_templates.locale_tr') },
      { value: 'en', label: t('prompt_templates.locale_en') },
    ],
    [t],
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
    const ok = await putData(`/api/ai/prompt-templates/${selectedName}/${editLocale}`, { content: trimmed })
    if (!ok) return
    setDirty(false)
    setSaveOk(t('prompt_templates.saved'))
    await load()
  }

  const handleRestore = async () => {
    if (!window.confirm(t('prompt_templates.confirm_restore'))) return
    setActionError(null)
    setSaveOk(null)
    const ok = await postData('/api/ai/prompt-templates/restore', {
      name: selectedName,
      locale: editLocale,
    })
    if (!ok) return
    setSaveOk(t('prompt_templates.restored'))
    await load()
  }

  const handleReseed = async () => {
    if (!window.confirm(t('prompt_templates.confirm_reseed'))) return
    setActionError(null)
    setSaveOk(null)
    const ok = await postData('/api/ai/prompt-templates/reseed', {})
    if (!ok) return
    setSaveOk(t('prompt_templates.reseeded'))
    await load()
  }

  const updatedLabel =
    currentRow?.updated_at != null
      ? new Date(currentRow.updated_at).toLocaleString()
      : '—'

  return (
    <div className="page-stack">
      {error && <ErrorAlert error={error} />}
      {actionError && <ErrorAlert error={actionError} />}
      {saveOk && <p className="card-subtitle">{saveOk}</p>}

      <div className="card">
        <div className="card-header-row">
          <h2>{t('prompt_templates.title')}</h2>
        </div>
        <p className="card-subtitle">{t('prompt_templates.manage_hint')}</p>

        <div className="form-row" style={{ gap: '1rem', flexWrap: 'wrap', alignItems: 'flex-end' }}>
          <label className="form-field" style={{ minWidth: '10rem' }}>
            <span className="form-label">{t('prompt_templates.label_locale')}</span>
            <Select
              value={editLocale}
              options={localeOptions}
              onChange={(v) => setEditLocale(v as EditLocale)}
            />
          </label>
          <label className="form-field" style={{ minWidth: '14rem', flex: 1 }}>
            <span className="form-label">{t('prompt_templates.label_section')}</span>
            <Select
              value={selectedName}
              options={templateOptions}
              onChange={(v) => setSelectedName(v as TemplateName)}
            />
          </label>
        </div>

        <p className="card-subtitle" style={{ marginTop: '0.75rem' }}>
          {t('prompt_templates.meta_updated', { date: updatedLabel })}
          {' · '}
          {t('prompt_templates.meta_chars', { count: draft.length })}
        </p>

        <textarea
          className="input"
          rows={22}
          style={{ width: '100%', fontFamily: 'var(--font-mono, monospace)', fontSize: '0.85rem' }}
          value={draft}
          onChange={(e) => {
            setDraft(e.target.value)
            setDirty(true)
            setSaveOk(null)
          }}
          spellCheck={false}
        />

        <div className="form-row" style={{ marginTop: '1rem', gap: '0.5rem', flexWrap: 'wrap' }}>
          <button
            type="button"
            className="btn btn-primary"
            disabled={loading || !dirty}
            onClick={() => void handleSave()}
          >
            {t('prompt_templates.save')}
          </button>
          <button type="button" className="btn btn-sm" disabled={loading} onClick={() => void handleRestore()}>
            {t('prompt_templates.restore_default')}
          </button>
          <button type="button" className="btn btn-sm btn-danger-outline" disabled={loading} onClick={() => void handleReseed()}>
            {t('prompt_templates.reseed_all')}
          </button>
        </div>
      </div>
    </div>
  )
}
