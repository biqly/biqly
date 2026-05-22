import { useCallback, useEffect, useMemo, useState } from 'react'
import { useApi } from '../hooks/useApi'
import { DEFAULT_LOCALE, LOCALE_OPTIONS, SUPPORTED_LOCALES, useT } from '../i18n'
import type { Locale, TranslationKey } from '../i18n'
import { ErrorAlert } from './ui/ErrorAlert'
import { Select } from './ui/Select'

const TEMPLATE_NAMES = ['system_rules', 'output_format', 'retry', 'clarification', 'prompt_layout'] as const
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

export default function PromptTemplates() {
  const t = useT()
  const { get, putData, postData, loading, error } = useApi()
  const [rows, setRows] = useState<PromptTemplateRow[]>([])
  const [editLocale, setEditLocale] = useState<EditLocale>(DEFAULT_LOCALE)
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
    () => rows.find((r) => r.name === selectedName && r.locale === editLocale && r.is_active),
    [rows, selectedName, editLocale],
  )

  const versionHistory = useMemo(
    () => rows.filter((r) => r.name === selectedName && r.locale === editLocale),
    [rows, selectedName, editLocale],
  )

  useEffect(() => {
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

        <div style={{
          display: 'flex',
          flexWrap: 'wrap',
          alignItems: 'center',
          gap: '0.5rem',
          marginTop: '0.75rem',
          marginBottom: '1rem',
        }}>
          <span style={{
            fontSize: '0.72rem',
            fontWeight: 500,
            background: 'rgba(255, 255, 255, 0.03)',
            border: '1px solid var(--border)',
            borderRadius: '4px',
            padding: '0.15rem 0.5rem',
            color: 'var(--text-secondary)'
          }}>
            {t('prompt_templates.meta_updated', { date: updatedLabel })}
          </span>
          <span style={{
            fontSize: '0.72rem',
            fontWeight: 500,
            background: 'rgba(255, 255, 255, 0.03)',
            border: '1px solid var(--border)',
            borderRadius: '4px',
            padding: '0.15rem 0.5rem',
            color: 'var(--text-secondary)'
          }}>
            {t('prompt_templates.meta_chars', { count: draft.length })}
          </span>
          <span style={{
            fontSize: '0.72rem',
            fontWeight: 500,
            background: 'rgba(99, 102, 241, 0.08)',
            border: '1px solid rgba(99, 102, 241, 0.15)',
            borderRadius: '4px',
            padding: '0.15rem 0.5rem',
            color: 'var(--accent)'
          }}>
            {t('prompt_templates.meta_version', { version: currentRow?.version ?? '-' })}
          </span>
        </div>

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

        {versionHistory.length > 0 && (
          <div style={{ marginTop: '1.5rem' }}>
            <h3 style={{ fontSize: '0.95rem', marginBottom: '0.5rem' }}>{t('prompt_templates.version_history')}</h3>
            <div className="table-wrap">
              <table className="results-table">
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
                      <td>{row.is_active ? t('prompt_templates.status_active') : t('prompt_templates.status_inactive')}</td>
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
