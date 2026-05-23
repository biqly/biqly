import { Fragment, useEffect, useMemo, useRef, useState } from 'react'
import { useApi } from '../hooks/useApi'
import { fetchDescribeBatchConflict } from '../api/describeBatchConflict'
import { jobIsActive, useAIJobs } from '../hooks/useAIJobs'
import { runMetadataDescribeDirect, type DescribeResult } from '../api/metadataDescribe'
import { useQueryParam } from '../hooks/useQueryParam'
import { FALLBACK_LOCALE, LOCALE_OPTIONS, SUPPORTED_LOCALES, useLocale, useT } from '../i18n'
import type { Locale, TranslationKey } from '../i18n'
import type { Datasource } from '../types/metadata'
import type { ColumnRow, TableRow } from '../types/semantic'
import { ErrorAlert } from './ui/ErrorAlert'
import { InlineEdit } from './ui/InlineEdit'
import { Select } from './ui/Select'
import { ModelBadgeRow } from './ui/ModelBadgeRow'
import type { AIRuntimeSettings } from '../types/ai'
import {
  BulkProgressHeader,
  BulkQueuePreview,
  BulkStatusBadge,
  objectTypeLabel,
  sortBulkEntriesForDisplay,
} from './metadata/bulkProgress'

type TFunction = (key: TranslationKey, params?: Record<string, string | number>) => string

/** PK / FK etiketleri — ayrı kolon yerine kolon adı satırında gösterilir. */
function columnKeySuffix(c: ColumnRow, t: TFunction): string | null {
  const parts: string[] = []
  if (c.is_primary_key) parts.push(t('metadata.col_pk'))
  if (c.is_foreign_key) {
    if (c.referenced_table && c.referenced_column) {
      const refSchema = c.referenced_schema?.trim()
      const crossSchema = refSchema && refSchema !== c.schema_name
      const target = crossSchema
        ? `${refSchema}.${c.referenced_table}.${c.referenced_column}`
        : `${c.referenced_table}.${c.referenced_column}`
      parts.push(t('metadata.col_fk_target', { target }))
    } else {
      parts.push(t('metadata.col_fk'))
    }
  }
  if (parts.length === 0) return null
  return parts.join(', ')
}

const DESC_TEXTAREA_MAX_ROWS = 24
/** Yaklaşık sütun genişliği (karakter); tek satırda yumuşak satır kırılımı tahmini için. */
const DESC_SOFT_WRAP_CHARS = 72

/** Düzenleme açılırken textarea satır sayısı: gerçek \n satırları + uzun satırlar için tahmini kırılım. */
function textareaRowsForDescription(text: string | null | undefined): number {
  const raw = text ?? ''
  if (!raw.trim()) return 1
  const parts = raw.split('\n')
  let rows = 0
  for (const line of parts) {
    rows += line.length === 0 ? 1 : Math.max(1, Math.ceil(line.length / DESC_SOFT_WRAP_CHARS))
  }
  return Math.min(DESC_TEXTAREA_MAX_ROWS, Math.max(1, rows))
}

export default function Metadata() {
  const { get, patchData, putData, loading, error } = useApi()
  const { runJob, bulkDescribe, jobs } = useAIJobs()
  const t = useT()
  const [locale] = useLocale()
  const [editLocale, setEditLocale] = useState<Locale>(locale)
  const descriptionLocaleOpts = useMemo(
    () => ({ headers: { 'X-Locale': editLocale } }),
    [editLocale],
  )
  const [datasources, setDatasources] = useState<Datasource[]>([])
  const [dsParam, setDsParam] = useQueryParam('ds')
  const [schemaParam, setSchemaParam] = useQueryParam('schema')
  const [typeParam, setTypeParam] = useQueryParam('type')
  const [datasourceId, setDatasourceId] = useState(dsParam)
  const [tables, setTables] = useState<TableRow[]>([])
  const [openTableId, setOpenTableId] = useState<string | null>(null)
  const [columns, setColumns] = useState<ColumnRow[]>([])
  const [editing, setEditing] = useState<{ kind: 'table' | 'column'; id: string; value: string } | null>(null)
  const [describeOpen, setDescribeOpen] = useState<TableRow | null>(null)
  const [describeForm, setDescribeForm] = useState({ sample_size: 10, auto_apply: false })
  const [describeResult, setDescribeResult] = useState<DescribeResult | null>(null)
  const [describeRunning, setDescribeRunning] = useState(false)
  const [describeError, setDescribeError] = useState<string | null>(null)
  const [bulkOpen, setBulkOpen] = useState(false)
  const [bulkConfig, setBulkConfig] = useState({ sample_size: 10, skip_existing: true })
  const { running: bulkRunning, entries: bulkEntries, summary: bulkSummary, start: startBulkDescribe, cancel: cancelBulkDescribe } =
    bulkDescribe
  const skipBlurSaveRef = useRef(false)
  const [tableFilterSchema, setTableFilterSchema] = useState(schemaParam)
  const [tableFilterType, setTableFilterType] = useState(typeParam)
  const [aiRuntime, setAiRuntime] = useState<AIRuntimeSettings | null>(null)
  /** Batch modal: which table_type values to include (all keys set true in openBulk). */
  const [bulkTypeEnabled, setBulkTypeEnabled] = useState<Record<string, boolean>>({})
  const [bulkSchemaRestrict, setBulkSchemaRestrict] = useState(false)
  const [bulkSchemasSelected, setBulkSchemasSelected] = useState<string[]>([])
  const [bulkScopeConflict, setBulkScopeConflict] = useState<{
    message: string
    schemas?: string
  } | null>(null)

  useEffect(() => {
    get<Datasource[]>('/api/datasources').then((data) => {
      if (!data) return
      setDatasources(data)
      setDatasourceId((prev) => {
        if (prev && data.some((d) => d.id === prev)) return prev
        return data[0]?.id ?? ''
      })
    })
    // Server AI runtime (BI_AI_MODEL etc.) — fed into the Describe / Bulk
    // Describe modals so users can see which LLM will write the descriptions.
    get<AIRuntimeSettings>('/api/ai/settings').then((data) => {
      if (data) setAiRuntime(data)
    })
  }, [])

  useEffect(() => {
    setDsParam(datasourceId)
  }, [datasourceId, setDsParam])
  useEffect(() => {
    setSchemaParam(tableFilterSchema)
  }, [tableFilterSchema, setSchemaParam])
  useEffect(() => {
    setTypeParam(tableFilterType)
  }, [tableFilterType, setTypeParam])

  /** Reset filters only when DS actually changes (preserves URL filters on first load,
   *  and is safe under StrictMode double-invoke in dev). */
  const prevDsRef = useRef(datasourceId)

  useEffect(() => {
    if (!datasourceId) return
    get<TableRow[]>(`/api/datasources/${datasourceId}/tables`, descriptionLocaleOpts).then((data) =>
      setTables(data || []),
    )
    setOpenTableId(null)
    setColumns([])
    if (prevDsRef.current && prevDsRef.current !== datasourceId) {
      setTableFilterSchema('')
      setTableFilterType('')
    }
    prevDsRef.current = datasourceId
  }, [datasourceId, editLocale, descriptionLocaleOpts, get])

  const schemaOptions = useMemo(
    () => [...new Set(tables.map((t) => t.schema_name))].sort((a, b) => a.localeCompare(b)),
    [tables]
  )
  const typeOptions = useMemo(
    () => [...new Set(tables.map((t) => t.table_type))].sort((a, b) => a.localeCompare(b)),
    [tables]
  )
  const filteredTables = useMemo(
    () =>
      tables.filter((t) => {
        if (tableFilterSchema && t.schema_name !== tableFilterSchema) return false
        if (tableFilterType && t.table_type !== tableFilterType) return false
        return true
      }),
    [tables, tableFilterSchema, tableFilterType]
  )

  useEffect(() => {
    if (!openTableId) return
    if (!filteredTables.some((t) => t.id === openTableId)) {
      setOpenTableId(null)
      setColumns([])
    }
  }, [filteredTables, openTableId])

  useEffect(() => {
    if (!datasourceId || !openTableId) return
    const tab = tables.find((t) => t.id === openTableId)
    if (!tab) return
    get<ColumnRow[]>(
      `/api/datasources/${datasourceId}/columns?schema=${encodeURIComponent(tab.schema_name)}&table=${encodeURIComponent(tab.table_name)}`,
      descriptionLocaleOpts,
    ).then((data) => setColumns(data || []))
  }, [datasourceId, openTableId, editLocale, descriptionLocaleOpts, get, tables])

  const bulkTargetTables = useMemo(() => {
    const restrictTypes = Object.keys(bulkTypeEnabled).length > 0
    return tables.filter((t) => {
      if (restrictTypes && !bulkTypeEnabled[t.table_type]) return false
      if (bulkSchemaRestrict) {
        if (bulkSchemasSelected.length === 0) return false
        if (!bulkSchemasSelected.includes(t.schema_name)) return false
      }
      return true
    })
  }, [tables, bulkTypeEnabled, bulkSchemaRestrict, bulkSchemasSelected])

  const bulkHasObjectType = typeOptions.length === 0 || typeOptions.some((ty) => bulkTypeEnabled[ty])
  const bulkScopeSchemas = useMemo(() => {
    if (bulkSchemaRestrict) {
      return [...bulkSchemasSelected].sort((a, b) => a.localeCompare(b))
    }
    return [...new Set(bulkTargetTables.map((t) => t.schema_name))].sort((a, b) => a.localeCompare(b))
  }, [bulkSchemaRestrict, bulkSchemasSelected, bulkTargetTables])

  const activeDescribeBatchJob = useMemo(
    () => jobs.find((j) => j.kind === 'describe_batch' && jobIsActive(j)),
    [jobs],
  )

  useEffect(() => {
    if (!datasourceId || bulkScopeSchemas.length === 0) {
      setBulkScopeConflict(null)
      return
    }
    let cancelled = false
    void fetchDescribeBatchConflict(datasourceId, bulkScopeSchemas).then((res) => {
      if (cancelled) return
      if (res?.conflict) {
        setBulkScopeConflict({
          message: t('metadata.already_running'),
          schemas: res.scope_schemas?.join(', ') ?? bulkScopeSchemas.join(', '),
        })
      } else {
        setBulkScopeConflict(null)
      }
    })
    return () => {
      cancelled = true
    }
  }, [datasourceId, bulkScopeSchemas, t])

  const bulkCanStart =
    bulkTargetTables.length > 0 &&
    bulkHasObjectType &&
    (!bulkSchemaRestrict || bulkSchemasSelected.length > 0) &&
    !bulkScopeConflict &&
    !bulkRunning

  const bulkEntriesDisplay = useMemo(
    () => (bulkEntries.length > 0 ? sortBulkEntriesForDisplay(bulkEntries) : []),
    [bulkEntries]
  )

  useEffect(() => {
    if (!describeOpen) return
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setDescribeOpen(null)
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [describeOpen])

  const toggleTable = async (t: TableRow) => {
    if (openTableId === t.id) {
      setOpenTableId(null)
      setColumns([])
      return
    }
    setOpenTableId(t.id)
    const data = await get<ColumnRow[]>(
      `/api/datasources/${datasourceId}/columns?schema=${encodeURIComponent(t.schema_name)}&table=${encodeURIComponent(t.table_name)}`,
      descriptionLocaleOpts,
    )
    setColumns(data || [])
  }

  const saveDescription = async () => {
    if (skipBlurSaveRef.current) {
      skipBlurSaveRef.current = false
      return
    }
    if (!editing) return
    const entityPath = editing.kind === 'table' ? 'tables' : 'columns'
    const value = editing.value.trim() === '' ? null : editing.value
    let ok = false
    if (editLocale === FALLBACK_LOCALE) {
      // The fallback locale corresponds to the legacy raw `description` column.
      const res = await patchData(`/api/metadata/${entityPath}/${editing.id}`, { description: value })
      ok = !!res
    } else {
      // Non-English: route through the translations endpoint so the raw
      // column stays as fallback and per-locale overlays are stored cleanly.
      const body: Record<string, Record<string, string>> = {
        [editLocale]: { description: value ?? '' },
      }
      const res = await putData(`/api/metadata/${entityPath}/${editing.id}/translations`, body)
      ok = !!res
    }
    if (ok) {
      if (editing.kind === 'table') {
        setTables(tables.map((t) => (t.id === editing.id ? { ...t, description: value } : t)))
      } else {
        setColumns(columns.map((c) => (c.id === editing.id ? { ...c, description: value } : c)))
      }
    }
    setEditing(null)
  }

  const openDescribe = (t: TableRow) => {
    setDescribeOpen(t)
    setDescribeResult(null)
    setDescribeError(null)
    setDescribeForm({ sample_size: 10, auto_apply: false })
  }

  const refreshDescribeTarget = (row: TableRow, applied: boolean) => {
    if (!applied) return
    get<TableRow[]>(`/api/datasources/${datasourceId}/tables`, descriptionLocaleOpts).then((d) => setTables(d || []))
    if (openTableId === row.id) {
      get<ColumnRow[]>(
        `/api/datasources/${datasourceId}/columns?schema=${encodeURIComponent(row.schema_name)}&table=${encodeURIComponent(row.table_name)}`,
        descriptionLocaleOpts,
      ).then((d) => setColumns(d || []))
    }
  }

  const runDescribe = async () => {
    if (!describeOpen) return
    setDescribeRunning(true)
    setDescribeError(null)
    const row = describeOpen
    const request = {
      datasource_id: datasourceId,
      schema: row.schema_name,
      table: row.table_name,
      sample_size: describeForm.sample_size,
      auto_apply: describeForm.auto_apply,
    }
    try {
      let res = await runJob<typeof request, DescribeResult>('describe', request, {
        onError: (message) => setDescribeError(message),
      })
      if (res === 'fallback') {
        res = await runMetadataDescribeDirect(request)
      }
      if (res) {
        setDescribeResult(res)
        refreshDescribeTarget(row, res.applied)
      }
    } catch (err) {
      setDescribeError(err instanceof Error ? err.message : t('metadata.bulk_network_error'))
    } finally {
      setDescribeRunning(false)
    }
  }

  const openBulk = () => {
    const types = [...new Set(tables.map((t) => t.table_type))].sort((a, b) => a.localeCompare(b))
    setBulkTypeEnabled(Object.fromEntries(types.map((ty) => [ty, true])))
    setBulkSchemaRestrict(false)
    setBulkSchemasSelected([])
    setBulkOpen(true)
  }

  const closeBulk = () => {
    setBulkOpen(false)
  }

  const runBulkDescribe = () => {
    const targets = bulkTargetTables
    if (!datasourceId || targets.length === 0 || bulkScopeConflict) return
    setBulkScopeConflict(null)
    startBulkDescribe({
      datasourceId,
      targets,
      sampleSize: bulkConfig.sample_size,
      skipExisting: bulkConfig.skip_existing,
      skipExistingMessage: t('metadata.bulk_skip_has_desc'),
      networkErrorMessage: t('metadata.bulk_network_error'),
      okColumnsMessage: (cols) => t('metadata.bulk_ok_columns', { cols }),
      onConflict: (message) => {
        setBulkScopeConflict({ message })
      },
      onFinished: () => {
        void get<TableRow[]>(`/api/datasources/${datasourceId}/tables`, descriptionLocaleOpts).then((fresh) => {
          if (fresh) setTables(fresh)
        })
      },
    })
  }

  const applySuggestion = async (kind: 'table' | 'column', name: string, description: string) => {
    if (!describeOpen) return
    if (kind === 'table') {
      await patchData(`/api/metadata/tables/${describeOpen.id}`, { description })
      setTables(tables.map((t) => (t.id === describeOpen.id ? { ...t, description } : t)))
    } else {
      const col = columns.find((c) => c.column_name === name)
      if (!col) return
      await patchData(`/api/metadata/columns/${col.id}`, { description })
      setColumns(columns.map((c) => (c.id === col.id ? { ...c, description } : c)))
    }
  }

  return (
    <div className="page-stack">
      <div className="card">
        <h2>{t('metadata.page_title')}</h2>
        <div className="form-group">
          <label>{t('metadata.datasource_label')}</label>
          <Select
            value={datasourceId}
            onChange={setDatasourceId}
            placeholder={t('query_builder.placeholder_pick_datasource')}
            header={t('query_builder.header_datasources')}
            options={datasources.map((d) => ({ value: d.id, label: d.name, hint: d.type }))}
          />
        </div>
        <ErrorAlert error={error} />
      </div>

      {datasourceId && (
        <div className="card">
          <div className="metadata-toolbar">
            <h2 className="metadata-toolbar__title">
              {t('metadata.tables')} (
              {filteredTables.length}
              {filteredTables.length !== tables.length ? ` / ${tables.length}` : ''})
            </h2>
            {tables.length > 0 && (
              <div className="metadata-table-filters metadata-table-filters--toolbar">
                <div className="form-group metadata-filter-field">
                  <Select
                    id="metadata-filter-schema"
                    ariaLabel={t('metadata.filter_schema_aria')}
                    value={tableFilterSchema}
                    onChange={setTableFilterSchema}
                    options={[
                      { value: '', label: t('metadata.filter_all_schemas') },
                      ...schemaOptions.map((s) => ({ value: s, label: s })),
                    ]}
                  />
                </div>
                <div className="form-group metadata-filter-field">
                  <Select
                    id="metadata-filter-type"
                    ariaLabel={t('metadata.filter_type_aria')}
                    value={tableFilterType}
                    onChange={setTableFilterType}
                    options={[
                      { value: '', label: t('metadata.filter_all_types') },
                      ...typeOptions.map((ty) => ({ value: ty, label: ty })),
                    ]}
                  />
                </div>
              </div>
            )}
            <div className="metadata-toolbar__actions">
              <div
                className="metadata-lang-tabs"
                role="tablist"
                aria-label={t('metadata.lang_tabs_aria')}
              >
                {SUPPORTED_LOCALES.map((loc) => (
                  <button
                    key={loc}
                    type="button"
                    role="tab"
                    aria-selected={editLocale === loc}
                    className={`metadata-lang-tab${editLocale === loc ? ' metadata-lang-tab--active' : ''}`}
                    onClick={() => setEditLocale(loc)}
                  >
                    {LOCALE_OPTIONS[loc].short}
                  </button>
                ))}
              </div>
              {editLocale !== FALLBACK_LOCALE && (
                <button
                  type="button"
                  className="metadata-hint-btn"
                  aria-label={t('metadata.desc_lang_hint_aria')}
                  title={t('metadata.desc_lang_tr_hint')}
                >
                  i
                </button>
              )}
              {tables.length > 0 && (
                <button
                  type="button"
                  className="btn btn-sm"
                  onClick={openBulk}
                  disabled={bulkRunning || (!!bulkScopeConflict && !bulkOpen)}
                  title={
                    bulkScopeConflict
                      ? `${bulkScopeConflict.message} ${t('metadata.already_running_schemas', { schemas: bulkScopeConflict.schemas ?? '' })}`
                      : undefined
                  }
                >
                  {t('metadata.bulk_ai_btn')}
                </button>
              )}
            </div>
          </div>
          {tables.length === 0 && !loading && (
            <p className="metadata-empty-hint">
              {t('metadata.no_tables_before')}
              <strong>{t('datasources.sync')}</strong>
              {t('metadata.no_tables_after')}
            </p>
          )}
          <table className="results-table results-table--metadata-list" lang={locale}>
            <colgroup>
              <col className="metadata-cw-name" />
              <col className="metadata-cw-type" />
              <col className="metadata-cw-desc" />
              <col className="metadata-cw-actions" />
            </colgroup>
            <thead>
              <tr>
                <th>{t('metadata.col_table_name')}</th>
                <th className="metadata-col-type">{t('metadata.col_object_type')}</th>
                <th>{t('metadata.col_table_desc')}</th>
                <th className="actions">{t('metadata.col_actions')}</th>
              </tr>
            </thead>
            <tbody>
              {filteredTables.length === 0 && tables.length > 0 && (
                <tr>
                  <td colSpan={4} style={{ color: 'var(--text-secondary)', fontSize: '0.85rem', padding: '0.75rem' }}>
                    {t('metadata.filter_no_match')}
                  </td>
                </tr>
              )}
              {filteredTables.map((tab) => (
                <Fragment key={tab.id}>
                  <tr className={openTableId === tab.id ? 'metadata-table-row metadata-table-row--expanded' : 'metadata-table-row'}>
                    <td>
                      <button
                        type="button"
                        className="icon-btn"
                        aria-expanded={openTableId === tab.id}
                        aria-label={
                          openTableId === tab.id
                            ? t('metadata.aria_table_collapse', { name: `${tab.schema_name}.${tab.table_name}` })
                            : t('metadata.aria_table_expand', { name: `${tab.schema_name}.${tab.table_name}` })
                        }
                        onClick={() => toggleTable(tab)}
                      >
                        <span className="chevron">{openTableId === tab.id ? '▼' : '▶'}</span>
                        {tab.schema_name}.{tab.table_name}
                      </button>
                    </td>
                    <td className="metadata-col-type">{tab.table_type}</td>
                    <InlineEdit
                      editing={editing?.kind === 'table' && editing.id === tab.id}
                      value={editing?.kind === 'table' && editing.id === tab.id ? editing.value : tab.description ?? ''}
                      placeholder={t('metadata.placeholder_double_click')}
                      rows={textareaRowsForDescription(editing?.kind === 'table' && editing.id === tab.id ? editing.value : tab.description)}
                      onStart={() => {
                        skipBlurSaveRef.current = false
                        setEditing({ kind: 'table', id: tab.id, value: tab.description ?? '' })
                      }}
                      onChange={(value) => setEditing({ kind: 'table', id: tab.id, value })}
                      onSave={() => void saveDescription()}
                      onCancel={() => {
                        skipBlurSaveRef.current = true
                        setEditing(null)
                      }}
                    />
                    <td className="actions">
                      <button type="button" className="btn btn-sm" onClick={() => openDescribe(tab)}>
                        {t('metadata.btn_ai_describe')}
                      </button>
                    </td>
                  </tr>
                  {openTableId === tab.id && columns.length > 0 && (
                    <tr className="metadata-nested-row">
                      <td colSpan={4} className="metadata-nested-cell">
                        <div className="metadata-nested-panel">
                          <table className="results-table results-table--metadata-list results-table--nested" lang={locale}>
                          <caption className="metadata-nested-caption">
                            {t('metadata.nested_columns_caption', { fqn: `${tab.schema_name}.${tab.table_name}` })}
                          </caption>
                          <colgroup>
                            <col className="metadata-ncol-name" />
                            <col className="metadata-ncol-type" />
                            <col className="metadata-ncol-desc" />
                          </colgroup>
                          <thead>
                            <tr>
                              <th scope="col">{t('metadata.col_column_name')}</th>
                              <th scope="col" className="metadata-col-type">{t('metadata.col_data_type')}</th>
                              <th scope="col">{t('metadata.col_column_desc')}</th>
                            </tr>
                          </thead>
                          <tbody>
                            {columns.map((c) => {
                              const keySuffix = columnKeySuffix(c, t)
                              const fkMultiline = !!(c.is_foreign_key && c.referenced_table && c.referenced_column)
                              return (
                              <tr key={c.id}>
                                <td className="metadata-col-name-cell">
                                  <span className="metadata-col-name-base">{c.column_name}</span>
                                  {keySuffix && (
                                    <span
                                      className={
                                        fkMultiline
                                          ? 'metadata-col-name-suffix metadata-col-name-suffix--multiline'
                                          : 'metadata-col-name-suffix'
                                      }
                                    >
                                      {fkMultiline ? `(${keySuffix})` : ` (${keySuffix})`}
                                    </span>
                                  )}
                                </td>
                                <td className="metadata-col-type">{c.data_type}{c.nullable ? '' : t('metadata.not_null_suffix')}</td>
                                <InlineEdit
                                  editing={editing?.kind === 'column' && editing.id === c.id}
                                  value={editing?.kind === 'column' && editing.id === c.id ? editing.value : c.description ?? ''}
                                  placeholder={t('metadata.placeholder_double_click')}
                                  rows={textareaRowsForDescription(editing?.kind === 'column' && editing.id === c.id ? editing.value : c.description)}
                                  onStart={() => {
                                    skipBlurSaveRef.current = false
                                    setEditing({ kind: 'column', id: c.id, value: c.description ?? '' })
                                  }}
                                  onChange={(value) => setEditing({ kind: 'column', id: c.id, value })}
                                  onSave={() => void saveDescription()}
                                  onCancel={() => {
                                    skipBlurSaveRef.current = true
                                    setEditing(null)
                                  }}
                                />
                              </tr>
                              )
                            })}
                          </tbody>
                        </table>
                        </div>
                      </td>
                    </tr>
                  )}
                </Fragment>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {bulkOpen && (
        <div
          className="modal-backdrop"
          role="presentation"
          onClick={(e) => { if (e.target === e.currentTarget) closeBulk() }}
        >
          <section
            className="modal-card modal-card--bulk-describe"
            role="dialog"
            aria-modal="true"
            aria-labelledby="bulk-metadata-title"
          >
            <header className="modal-header modal-header--compact">
              <div>
                <h2 id="bulk-metadata-title" className="bulk-modal-title">{t('metadata.bulk_modal_title')}</h2>
                <p className="bulk-modal-subtitle">{t('metadata.bulk_modal_subtitle')}</p>
                <ModelBadgeRow
                  primaryLabel={t('metadata.describe_badge_label')}
                  primaryModel={aiRuntime?.llm_model}
                  translationModel={aiRuntime?.translation_enabled ? aiRuntime?.translation_model : undefined}
                />
              </div>
              <button
                type="button"
                className="modal-close"
                aria-label={t('metadata.bulk_close_aria')}
                onClick={closeBulk}
              >
                ×
              </button>
            </header>
            <div className={`modal-body${bulkEntries.length > 0 ? ' modal-body--scroll' : ''}`}>
              {bulkEntries.length === 0 && !bulkRunning && (
                <>
                  <p className="bulk-lede">
                    {t('metadata.bulk_lede')}
                  </p>
                  <div className="bulk-panel-grid">
                    <fieldset className="bulk-fieldset">
                      <legend className="bulk-legend">{t('metadata.bulk_legend_types')}</legend>
                      <div className="bulk-pill-row" role="group" aria-label={t('metadata.bulk_aria_types')}>
                        {typeOptions.map((ty) => (
                          <button
                            key={ty}
                            type="button"
                            className={`bulk-pill${bulkTypeEnabled[ty] === true ? ' bulk-pill--on' : ''}`}
                            aria-pressed={bulkTypeEnabled[ty] === true}
                            onClick={() =>
                              setBulkTypeEnabled((prev) => ({ ...prev, [ty]: !prev[ty] }))
                            }
                          >
                            <span className="bulk-pill-label">{objectTypeLabel(ty, t)}</span>
                            <span className="bulk-pill-code">{ty}</span>
                          </button>
                        ))}
                      </div>
                      {!bulkHasObjectType && (
                        <p className="bulk-modal-warn">{t('metadata.bulk_warn_pick_type')}</p>
                      )}
                    </fieldset>
                    <fieldset className="bulk-fieldset">
                      <legend className="bulk-legend">{t('metadata.bulk_legend_schemas')}</legend>
                      <div
                        className="bulk-segmented"
                        role="group"
                        aria-label={t('metadata.bulk_aria_schema_scope')}
                      >
                        <button
                          type="button"
                          className={!bulkSchemaRestrict ? 'bulk-segmented__btn bulk-segmented__btn--active' : 'bulk-segmented__btn'}
                          onClick={() => {
                            setBulkSchemaRestrict(false)
                            setBulkSchemasSelected([])
                          }}
                        >
                          {t('metadata.bulk_all_schemas')}
                        </button>
                        <button
                          type="button"
                          className={bulkSchemaRestrict ? 'bulk-segmented__btn bulk-segmented__btn--active' : 'bulk-segmented__btn'}
                          onClick={() => {
                            setBulkSchemaRestrict(true)
                            setBulkSchemasSelected((prev) => (prev.length > 0 ? prev : [...schemaOptions]))
                          }}
                        >
                          {t('metadata.bulk_pick_schemas')}
                        </button>
                      </div>
                      <div
                        className={`bulk-schema-box${bulkSchemaRestrict ? ' bulk-schema-box--active' : ''}`}
                      >
                        {!bulkSchemaRestrict ? (
                          <p className="bulk-schema-placeholder">{t('metadata.bulk_schema_all_hint')}</p>
                        ) : (
                          <>
                            <select
                              id="bulk-schema-multiselect"
                              className="bulk-schema-multiselect"
                              multiple
                              size={Math.min(8, Math.max(4, schemaOptions.length))}
                              value={bulkSchemasSelected}
                              onChange={(e) =>
                                setBulkSchemasSelected([...e.target.selectedOptions].map((o) => o.value))
                              }
                              aria-label={t('metadata.bulk_aria_schemas_pick')}
                            >
                              {schemaOptions.map((s) => (
                                <option key={s} value={s}>{s}</option>
                              ))}
                            </select>
                            <div className="bulk-schema-multiselect-tools">
                              <button
                                type="button"
                                className="btn btn-ghost btn-sm"
                                onClick={() => setBulkSchemasSelected([...schemaOptions])}
                              >
                                {t('metadata.bulk_select_all')}
                              </button>
                              <button
                                type="button"
                                className="btn btn-ghost btn-sm"
                                onClick={() => setBulkSchemasSelected([])}
                              >
                                {t('metadata.bulk_select_none')}
                              </button>
                              <span className="bulk-schema-hint">{t('metadata.bulk_multiselect_hint')}</span>
                            </div>
                          </>
                        )}
                      </div>
                    </fieldset>
                  </div>
                  <div className="bulk-options-row">
                    <div className="form-group bulk-opt-field">
                      <label className="bulk-opt-label" htmlFor="bulk-sample-size">{t('metadata.bulk_sample_rows')}</label>
                      <input
                        id="bulk-sample-size"
                        type="number"
                        min={1}
                        max={100}
                        className="bulk-opt-input"
                        value={bulkConfig.sample_size}
                        onChange={(e) => setBulkConfig({ ...bulkConfig, sample_size: Number(e.target.value) })}
                      />
                    </div>
                    <label className="bulk-skip-label" htmlFor="bulk-skip-existing">
                      <input
                        id="bulk-skip-existing"
                        type="checkbox"
                        checked={bulkConfig.skip_existing}
                        onChange={(e) => setBulkConfig({ ...bulkConfig, skip_existing: e.target.checked })}
                      />
                      <span>{t('metadata.bulk_skip_existing')}</span>
                    </label>
                  </div>
                  <div className="bulk-scope-footer">
                    <span className="bulk-scope-stat">
                      {t('metadata.bulk_scope_objects')} <strong>{bulkTargetTables.length}</strong> {t('metadata.bulk_scope_suffix')}
                      {bulkTargetTables.length !== tables.length && (
                        <span className="bulk-scope-of">{t('metadata.bulk_scope_total', { total: tables.length })}</span>
                      )}
                    </span>
                  </div>
                  {bulkScopeConflict && (
                    <p className="bulk-modal-warn" role="status">
                      {bulkScopeConflict.message}{' '}
                      {bulkScopeConflict.schemas
                        ? t('metadata.already_running_schemas', { schemas: bulkScopeConflict.schemas })
                        : null}
                    </p>
                  )}
                  <div className="modal-actions">
                    <button type="button" className="btn btn-ghost btn-sm" onClick={closeBulk}>{t('metadata.bulk_cancel')}</button>
                    <button
                      type="button"
                      className="btn btn-sm"
                      onClick={runBulkDescribe}
                      disabled={!bulkCanStart}
                    >
                      {t('metadata.bulk_start', { count: bulkTargetTables.length })}
                    </button>
                  </div>
                </>
              )}

              {bulkEntries.length > 0 && (
                <>
                  <BulkProgressHeader entries={bulkEntries} running={bulkRunning} summary={bulkSummary} />
                  {bulkRunning && (
                    <BulkQueuePreview
                      entries={bulkEntries}
                      progress={activeDescribeBatchJob?.progress_json ?? null}
                    />
                  )}
                  <div className="bulk-describe-scroll">
                    <table className="results-table results-table--dense" style={{ margin: 0 }}>
                      <thead>
                        <tr>
                          <th className="bulk-col-idx">{t('metadata.bulk_table_idx')}</th>
                          <th>{t('metadata.bulk_table_name')}</th>
                          <th className="bulk-col-status">{t('metadata.bulk_table_status')}</th>
                          <th>{t('metadata.bulk_table_detail')}</th>
                        </tr>
                      </thead>
                      <tbody>
                        {bulkEntriesDisplay.map((e, idx) => (
                          <tr key={`${e.schema}.${e.table}`}>
                            <td className="bulk-col-idx">{idx + 1}</td>
                            <td className="bulk-col-name">
                              <code>{e.schema}.{e.table}</code>
                            </td>
                            <td className="bulk-col-status">
                              <BulkStatusBadge status={e.status} />
                            </td>
                            <td className="bulk-col-detail" style={{ color: 'var(--text-secondary)' }}>
                              <span className="bulk-col-detail-inner" title={e.message}>
                                {e.message || (e.status === 'pending' ? t('common.em_dash') : '')}
                              </span>
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                  <div className="modal-actions">
                    {bulkRunning ? (
                      <>
                        <button type="button" className="btn btn-ghost btn-sm" onClick={closeBulk}>
                          {t('metadata.bulk_run_background')}
                        </button>
                        <button type="button" className="btn btn-ghost btn-sm" onClick={cancelBulkDescribe}>
                          {t('metadata.bulk_stop_after')}
                        </button>
                      </>
                    ) : (
                      <button type="button" className="btn btn-sm" onClick={closeBulk}>{t('metadata.bulk_close_btn')}</button>
                    )}
                  </div>
                </>
              )}
            </div>
          </section>
        </div>
      )}

      {describeOpen && (
        <div
          className="modal-backdrop"
          role="presentation"
          onClick={(e) => { if (e.target === e.currentTarget) setDescribeOpen(null) }}
        >
          <section
            className="modal-card"
            role="dialog"
            aria-modal="true"
            aria-labelledby="describe-title"
          >
            <header className="modal-header">
              <div>
                <h2 id="describe-title">
                  {t('metadata.describe_modal_title', { fqn: `${describeOpen.schema_name}.${describeOpen.table_name}` })}
                </h2>
                <ModelBadgeRow
                  primaryLabel={t('metadata.describe_badge_label')}
                  primaryModel={describeResult?.model ?? aiRuntime?.llm_model}
                  translationModel={
                    describeResult?.translation_applied
                      ? describeResult?.translation_model
                      : aiRuntime?.translation_enabled
                        ? aiRuntime?.translation_model
                        : undefined
                  }
                />
              </div>
              <button
                type="button"
                className="modal-close"
                aria-label={t('metadata.describe_close_aria')}
                onClick={() => setDescribeOpen(null)}
              >
                ×
              </button>
            </header>

            <div className="modal-body">
              <p style={{ color: 'var(--text-secondary)', margin: 0 }}>
                {t('metadata.describe_intro')}
              </p>

              {!describeResult && (
                <>
                  <div className="modal-form-row">
                    <div className="form-group">
                      <label htmlFor="describe-sample-size">{t('metadata.describe_sample_size')}</label>
                      <input
                        id="describe-sample-size"
                        name="sample_size"
                        type="number"
                        min={1}
                        max={100}
                        value={describeForm.sample_size}
                        onChange={(e) => setDescribeForm({ ...describeForm, sample_size: Number(e.target.value) })}
                      />
                    </div>
                    <div className="form-group">
                      <label>{t('metadata.describe_options')}</label>
                      <div className="checkbox-row">
                        <input
                          id="auto-apply"
                          name="auto_apply"
                          type="checkbox"
                          checked={describeForm.auto_apply}
                          onChange={(e) => setDescribeForm({ ...describeForm, auto_apply: e.target.checked })}
                        />
                        <label htmlFor="auto-apply">{t('metadata.describe_auto_apply')}</label>
                      </div>
                    </div>
                  </div>
                  <ErrorAlert error={describeError || error} />
                  <div className="modal-actions">
                    <button
                      type="button"
                      className="btn btn-ghost btn-sm"
                      onClick={() => setDescribeOpen(null)}
                    >
                      {t('metadata.bulk_cancel')}
                    </button>
                    <button
                      type="button"
                      className="btn btn-sm"
                      onClick={runDescribe}
                      disabled={describeRunning}
                    >
                      {describeRunning ? t('metadata.describe_analyzing') : t('metadata.describe_generate')}
                    </button>
                  </div>
                </>
              )}

              {describeResult && (
                <>
                  {describeResult.model && (
                    <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>
                      {t('metadata.describe_model_line')} <code translate="no">{describeResult.model}</code>
                      {describeResult.translation_applied && describeResult.translation_model ? (
                        <>{t('metadata.describe_translation_sep')} <code translate="no">{describeResult.translation_model}</code></>
                      ) : null}
                    </div>
                  )}
                  <p style={{ color: 'var(--text-secondary)', margin: 0 }}>
                    {t('metadata.describe_rows_sampled', { n: describeResult.sample_rows })}{' '}
                    {describeResult.applied
                      ? <span className="success">{t('metadata.describe_all_applied')}</span>
                      : t('metadata.describe_review_apply')}
                  </p>
                  {describeResult.translation_error && (
                    <p style={{ margin: 0, color: 'var(--error)' }}>
                      {t('metadata.describe_translation_failed')} {describeResult.translation_error}
                    </p>
                  )}

                  <div>
                    <h3 style={{ marginBottom: '0.4rem' }}>{t('metadata.describe_section_table')}</h3>
                    <div className="suggestion-block">
                      {describeResult.description || <em style={{ color: 'var(--text-secondary)' }}>{t('metadata.describe_empty_paren')}</em>}
                    </div>
                    {!describeResult.applied && describeResult.description && (
                      <div className="modal-actions">
                        <button
                          type="button"
                          className="btn btn-sm"
                          onClick={() => applySuggestion('table', '', describeResult.description)}
                        >
                          {t('metadata.describe_apply_table')}
                        </button>
                      </div>
                    )}
                  </div>

                  <div>
                    <h3 style={{ marginBottom: '0.4rem' }}>{t('metadata.describe_section_columns')}</h3>
                    <table className="results-table">
                      <thead>
                        <tr>
                          <th>{t('metadata.describe_col_column')}</th>
                          <th>{t('metadata.describe_col_suggestion')}</th>
                          {!describeResult.applied && <th style={{ textAlign: 'right' }}>{t('metadata.describe_col_action')}</th>}
                        </tr>
                      </thead>
                      <tbody>
                        {describeResult.columns.map((c) => (
                          <tr key={c.name}>
                            <td><code>{c.name}</code></td>
                            <td>{c.description || <em style={{ color: 'var(--text-secondary)' }}>{t('metadata.describe_empty_paren')}</em>}</td>
                            {!describeResult.applied && (
                              <td className="actions">
                                {c.description && (
                                  <button
                                    type="button"
                                    className="btn btn-sm"
                                    onClick={() => applySuggestion('column', c.name, c.description)}
                                  >
                                    {t('metadata.describe_apply')}
                                  </button>
                                )}
                              </td>
                            )}
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>

                  <div className="modal-actions">
                    <button
                      type="button"
                      className="btn btn-ghost btn-sm"
                      onClick={() => { setDescribeResult(null); setDescribeOpen(null) }}
                    >
                      {t('metadata.describe_close_footer')}
                    </button>
                  </div>
                </>
              )}
            </div>
          </section>
        </div>
      )}
    </div>
  )
}
