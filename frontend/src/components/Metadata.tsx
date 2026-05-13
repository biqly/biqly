import { Fragment, useEffect, useMemo, useRef, useState } from 'react'
import { useApi } from '../hooks/useApi'
import { useQueryParam } from '../hooks/useQueryParam'
import type { Datasource } from '../types/metadata'
import { InlineEdit } from './ui/InlineEdit'
import { Select } from './ui/Select'
import { ModelBadgeRow } from './ui/ModelBadgeRow'
import type { AIRuntimeSettings } from '../types/ai'
import { BulkProgressHeader, BulkStatusBadge, objectTypeLabel, sortBulkEntriesForDisplay, type BulkEntry } from './metadata/bulkProgress'

/**
 * AI metadata/describe can run primary LLM + optional translation in one request.
 * Default useApi timeout (30s) aborts too early; align with server AIRequestTimeout / nginx.
 */
const AI_METADATA_DESCRIBE_TIMEOUT_MS = 600_000


interface TableRow {
  id: string
  schema_name: string
  table_name: string
  table_type: string
  description: string | null
}

interface ColumnRow {
  id: string
  schema_name: string
  table_name: string
  column_name: string
  data_type: string
  nullable: boolean
  description: string | null
  is_primary_key: boolean
  is_foreign_key: boolean
  referenced_table: string | null
  referenced_column: string | null
}

/** PK / FK etiketleri — ayrı kolon yerine kolon adı satırında gösterilir. */
function columnKeySuffix(c: ColumnRow): string | null {
  const parts: string[] = []
  if (c.is_primary_key) parts.push('PK')
  if (c.is_foreign_key) {
    if (c.referenced_table && c.referenced_column) {
      parts.push(`FK→${c.referenced_table}.${c.referenced_column}`)
    } else {
      parts.push('FK')
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

interface DescribeResult {
  schema: string
  table: string
  description: string
  columns: { name: string; description: string }[]
  applied: boolean
  sample_rows: number
  /** LLM that produced the suggestions (Backend BI_AI_MODEL). */
  model?: string
  translation_applied?: boolean
  translation_model?: string
  translation_error?: string
}

export default function Metadata() {
  const { get, postData, patchData, loading, error } = useApi()
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
  const [bulkOpen, setBulkOpen] = useState(false)
  const [bulkConfig, setBulkConfig] = useState({ sample_size: 10, skip_existing: true })
  const [bulkRunning, setBulkRunning] = useState(false)
  const [bulkEntries, setBulkEntries] = useState<BulkEntry[]>([])
  const [bulkSummary, setBulkSummary] = useState<{ ok: number; error: number; skipped: number } | null>(null)
  const bulkCancelRef = useRef(false)
  const skipBlurSaveRef = useRef(false)
  const [tableFilterSchema, setTableFilterSchema] = useState(schemaParam)
  const [tableFilterType, setTableFilterType] = useState(typeParam)
  const [aiRuntime, setAiRuntime] = useState<AIRuntimeSettings | null>(null)
  /** Batch modal: which table_type values to include (all keys set true in openBulk). */
  const [bulkTypeEnabled, setBulkTypeEnabled] = useState<Record<string, boolean>>({})
  const [bulkSchemaRestrict, setBulkSchemaRestrict] = useState(false)
  const [bulkSchemasSelected, setBulkSchemasSelected] = useState<string[]>([])

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
    get<TableRow[]>(`/api/datasources/${datasourceId}/tables`).then((data) => setTables(data || []))
    setOpenTableId(null)
    setColumns([])
    if (prevDsRef.current && prevDsRef.current !== datasourceId) {
      setTableFilterSchema('')
      setTableFilterType('')
    }
    prevDsRef.current = datasourceId
  }, [datasourceId])

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
  const bulkCanStart =
    bulkTargetTables.length > 0 && bulkHasObjectType && (!bulkSchemaRestrict || bulkSchemasSelected.length > 0)

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
      `/api/datasources/${datasourceId}/columns?schema=${encodeURIComponent(t.schema_name)}&table=${encodeURIComponent(t.table_name)}`
    )
    setColumns(data || [])
  }

  const saveDescription = async () => {
    if (skipBlurSaveRef.current) {
      skipBlurSaveRef.current = false
      return
    }
    if (!editing) return
    const url = editing.kind === 'table' ? `/api/metadata/tables/${editing.id}` : `/api/metadata/columns/${editing.id}`
    const value = editing.value.trim() === '' ? null : editing.value
    const res = await patchData(url, { description: value })
    if (res) {
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
    setDescribeForm({ sample_size: 10, auto_apply: false })
  }

  const runDescribe = async () => {
    if (!describeOpen) return
    const res = await postData<DescribeResult>(
      '/api/ai/metadata/describe',
      {
        datasource_id: datasourceId,
        schema: describeOpen.schema_name,
        table: describeOpen.table_name,
        sample_size: describeForm.sample_size,
        auto_apply: describeForm.auto_apply,
      },
      { timeout: AI_METADATA_DESCRIBE_TIMEOUT_MS },
    )
    if (res) {
      setDescribeResult(res)
      if (res.applied) {
        // refresh table + columns
        get<TableRow[]>(`/api/datasources/${datasourceId}/tables`).then((d) => setTables(d || []))
        if (openTableId === describeOpen.id) {
          get<ColumnRow[]>(
            `/api/datasources/${datasourceId}/columns?schema=${encodeURIComponent(describeOpen.schema_name)}&table=${encodeURIComponent(describeOpen.table_name)}`
          ).then((d) => setColumns(d || []))
        }
      }
    }
  }

  const openBulk = () => {
    const types = [...new Set(tables.map((t) => t.table_type))].sort((a, b) => a.localeCompare(b))
    setBulkTypeEnabled(Object.fromEntries(types.map((ty) => [ty, true])))
    setBulkSchemaRestrict(false)
    setBulkSchemasSelected([])
    setBulkOpen(true)
    setBulkEntries([])
    setBulkSummary(null)
    setBulkRunning(false)
    bulkCancelRef.current = false
  }

  const closeBulk = () => {
    if (bulkRunning) bulkCancelRef.current = true
    setBulkOpen(false)
  }

  const runBulkDescribe = async () => {
    const targets = bulkTargetTables
    if (!datasourceId || targets.length === 0) return
    bulkCancelRef.current = false
    setBulkRunning(true)
    setBulkSummary(null)

    const queue: BulkEntry[] = targets.map((t) => {
      if (bulkConfig.skip_existing && t.description) {
        return { schema: t.schema_name, table: t.table_name, status: 'skipped', message: 'zaten açıklamalı' }
      }
      return { schema: t.schema_name, table: t.table_name, status: 'pending' }
    })
    setBulkEntries(queue)

    let ok = 0
    let errCount = 0
    let skipped = queue.filter((q) => q.status === 'skipped').length

    for (let i = 0; i < targets.length; i++) {
      if (bulkCancelRef.current) break
      const t = targets[i]
      const entry = queue[i]
      if (!t || !entry || entry.status === 'skipped') continue

      const schema = t.schema_name
      const table = t.table_name
      queue[i] = { schema, table, status: 'running' }
      setBulkEntries([...queue])

      try {
        const res = await fetch('/api/ai/metadata/describe', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            datasource_id: datasourceId,
            schema,
            table,
            sample_size: bulkConfig.sample_size,
            auto_apply: true,
          }),
        })
        const text = await res.text()
        const data = text ? JSON.parse(text) : null
        if (!res.ok) {
          queue[i] = { schema, table, status: 'error', message: data?.error || `HTTP ${res.status}` }
          errCount++
        } else {
          const cols = data?.columns?.length ?? 0
          queue[i] = { schema, table, status: 'ok', message: `${cols} kolon açıklandı` }
          ok++
        }
      } catch (err) {
        queue[i] = { schema, table, status: 'error', message: err instanceof Error ? err.message : 'ağ hatası' }
        errCount++
      }
      setBulkEntries([...queue])
    }

    setBulkRunning(false)
    setBulkSummary({ ok, error: errCount, skipped })
    // refresh table list to pick up new descriptions
    const fresh = await get<TableRow[]>(`/api/datasources/${datasourceId}/tables`)
    if (fresh) setTables(fresh)
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
        <h2>Metadata Tarayıcı</h2>
        <div className="form-group">
          <label>Veri Kaynağı</label>
          <Select
            value={datasourceId}
            onChange={setDatasourceId}
            placeholder="— seçin —"
            header="Veri Kaynakları"
            options={datasources.map((d) => ({ value: d.id, label: d.name, hint: d.type }))}
          />
        </div>
        {error && <div className="error">{error}</div>}
      </div>

      {datasourceId && (
        <div className="card">
          <div className="card-header-row card-header-row--spaced">
            <h2>
              Tablolar (
              {filteredTables.length}
              {filteredTables.length !== tables.length ? ` / ${tables.length}` : ''})
            </h2>
            {tables.length > 0 && (
              <button type="button" className="btn btn-sm" onClick={openBulk} disabled={bulkRunning}>
                AI Metadata Üretici
              </button>
            )}
          </div>
          {tables.length === 0 && !loading && (
            <p style={{ color: 'var(--text-secondary)' }}>
              Tablo yok. Veri Kaynakları sekmesinden <strong>Eşitle</strong> çalıştırın.
            </p>
          )}
          {tables.length > 0 && (
            <div className="metadata-table-filters">
              <div className="form-group metadata-filter-field">
                <Select
                  id="metadata-filter-schema"
                  ariaLabel="Şema filtresi"
                  value={tableFilterSchema}
                  onChange={setTableFilterSchema}
                  options={[
                    { value: '', label: 'Tüm şemalar' },
                    ...schemaOptions.map((s) => ({ value: s, label: s })),
                  ]}
                />
              </div>
              <div className="form-group metadata-filter-field">
                <Select
                  id="metadata-filter-type"
                  ariaLabel="Tablo türü filtresi"
                  value={tableFilterType}
                  onChange={setTableFilterType}
                  options={[
                    { value: '', label: 'Tüm türler' },
                    ...typeOptions.map((ty) => ({ value: ty, label: ty })),
                  ]}
                />
              </div>
            </div>
          )}
          <table className="results-table results-table--metadata-list" lang="tr">
            <colgroup>
              <col className="metadata-cw-name" />
              <col className="metadata-cw-type" />
              <col className="metadata-cw-desc" />
              <col className="metadata-cw-actions" />
            </colgroup>
            <thead>
              <tr>
                <th>Tablo adı</th>
                <th className="metadata-col-type">Nesne türü</th>
                <th>Tablo açıklaması</th>
                <th className="actions">İşlemler</th>
              </tr>
            </thead>
            <tbody>
              {filteredTables.length === 0 && tables.length > 0 && (
                <tr>
                  <td colSpan={4} style={{ color: 'var(--text-secondary)', fontSize: '0.85rem', padding: '0.75rem' }}>
                    Mevcut filtrelerle eşleşen tablo yok.
                  </td>
                </tr>
              )}
              {filteredTables.map((t) => (
                <Fragment key={t.id}>
                  <tr className={openTableId === t.id ? 'metadata-table-row metadata-table-row--expanded' : 'metadata-table-row'}>
                    <td>
                      <button
                        type="button"
                        className="icon-btn"
                        aria-expanded={openTableId === t.id}
                        aria-label={`${t.schema_name}.${t.table_name} ${openTableId === t.id ? 'kapat' : 'genişlet'}`}
                        onClick={() => toggleTable(t)}
                      >
                        <span className="chevron">{openTableId === t.id ? '▼' : '▶'}</span>
                        {t.schema_name}.{t.table_name}
                      </button>
                    </td>
                    <td className="metadata-col-type">{t.table_type}</td>
                    <InlineEdit
                      editing={editing?.kind === 'table' && editing.id === t.id}
                      value={editing?.kind === 'table' && editing.id === t.id ? editing.value : t.description ?? ''}
                      placeholder="(düzenlemek için çift tıklayın)"
                      rows={textareaRowsForDescription(editing?.kind === 'table' && editing.id === t.id ? editing.value : t.description)}
                      onStart={() => {
                        skipBlurSaveRef.current = false
                        setEditing({ kind: 'table', id: t.id, value: t.description ?? '' })
                      }}
                      onChange={(value) => setEditing({ kind: 'table', id: t.id, value })}
                      onSave={() => void saveDescription()}
                      onCancel={() => {
                        skipBlurSaveRef.current = true
                        setEditing(null)
                      }}
                    />
                    <td className="actions">
                      <button type="button" className="btn btn-sm" onClick={() => openDescribe(t)}>
                        🤖 AI Açıkla
                      </button>
                    </td>
                  </tr>
                  {openTableId === t.id && columns.length > 0 && (
                    <tr className="metadata-nested-row">
                      <td colSpan={4} className="metadata-nested-cell">
                        <div className="metadata-nested-panel">
                          <table className="results-table results-table--metadata-list results-table--nested" lang="tr">
                          <caption className="metadata-nested-caption">
                            Kolonlar — {t.schema_name}.{t.table_name}
                          </caption>
                          <colgroup>
                            <col className="metadata-ncol-name" />
                            <col className="metadata-ncol-type" />
                            <col className="metadata-ncol-desc" />
                          </colgroup>
                          <thead>
                            <tr>
                              <th scope="col">Kolon adı</th>
                              <th scope="col" className="metadata-col-type">Veri türü</th>
                              <th scope="col">Kolon tanımı</th>
                            </tr>
                          </thead>
                          <tbody>
                            {columns.map((c) => {
                              const keySuffix = columnKeySuffix(c)
                              const suffixMultiline = Boolean(keySuffix?.includes('FK'))
                              return (
                              <tr key={c.id}>
                                <td className="metadata-col-name-cell">
                                  <span className="metadata-col-name-base">{c.column_name}</span>
                                  {keySuffix && (
                                    <span
                                      className={
                                        suffixMultiline
                                          ? 'metadata-col-name-suffix metadata-col-name-suffix--multiline'
                                          : 'metadata-col-name-suffix'
                                      }
                                    >
                                      {suffixMultiline ? `(${keySuffix})` : ` (${keySuffix})`}
                                    </span>
                                  )}
                                </td>
                                <td className="metadata-col-type">{c.data_type}{c.nullable ? '' : ' NOT NULL'}</td>
                                <InlineEdit
                                  editing={editing?.kind === 'column' && editing.id === c.id}
                                  value={editing?.kind === 'column' && editing.id === c.id ? editing.value : c.description ?? ''}
                                  placeholder="(düzenlemek için çift tıklayın)"
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
          onClick={(e) => { if (e.target === e.currentTarget && !bulkRunning) closeBulk() }}
        >
          <section
            className="modal-card modal-card--bulk-describe"
            role="dialog"
            aria-modal="true"
            aria-labelledby="bulk-metadata-title"
          >
            <header className="modal-header modal-header--compact">
              <div>
                <h2 id="bulk-metadata-title" className="bulk-modal-title">AI Metadata Üretici</h2>
                <p className="bulk-modal-subtitle">Seçili tablo ve kolonlar için Türkçe öncelikli LLM açıklamaları</p>
                <ModelBadgeRow
                  primaryLabel="Açıklama"
                  primaryModel={aiRuntime?.llm_model}
                  translationModel={aiRuntime?.translation_enabled ? aiRuntime?.translation_model : undefined}
                />
              </div>
              <button
                type="button"
                className="modal-close"
                aria-label="Kapat"
                onClick={closeBulk}
              >
                ×
              </button>
            </header>
            <div className={`modal-body${bulkEntries.length > 0 ? ' modal-body--scroll' : ''}`}>
              {bulkEntries.length === 0 && !bulkRunning && (
                <>
                  <p className="bulk-lede">
                    Açıklamalar örneklenen satırlardan çıkarılır ve metadata'ya kaydedilir. Varsayılan olarak Türkçe üretilirken yararlı teknik tablo/kolon adları korunur. Büyük seçimler daha fazla token ve zaman kullanır.
                  </p>
                  <div className="bulk-panel-grid">
                    <fieldset className="bulk-fieldset">
                      <legend className="bulk-legend">Nesne türleri</legend>
                      <div className="bulk-pill-row" role="group" aria-label="Dahil edilecek nesne türleri">
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
                            <span className="bulk-pill-label">{objectTypeLabel(ty)}</span>
                            <span className="bulk-pill-code">{ty}</span>
                          </button>
                        ))}
                      </div>
                      {!bulkHasObjectType && (
                        <p className="bulk-modal-warn">En az bir tür seçin.</p>
                      )}
                    </fieldset>
                    <fieldset className="bulk-fieldset">
                      <legend className="bulk-legend">Şemalar</legend>
                      <div
                        className="bulk-segmented"
                        role="group"
                        aria-label="Şema kapsamı"
                      >
                        <button
                          type="button"
                          className={!bulkSchemaRestrict ? 'bulk-segmented__btn bulk-segmented__btn--active' : 'bulk-segmented__btn'}
                          onClick={() => {
                            setBulkSchemaRestrict(false)
                            setBulkSchemasSelected([])
                          }}
                        >
                          Tüm şemalar
                        </button>
                        <button
                          type="button"
                          className={bulkSchemaRestrict ? 'bulk-segmented__btn bulk-segmented__btn--active' : 'bulk-segmented__btn'}
                          onClick={() => {
                            setBulkSchemaRestrict(true)
                            setBulkSchemasSelected((prev) => (prev.length > 0 ? prev : [...schemaOptions]))
                          }}
                        >
                          Seç…
                        </button>
                      </div>
                      <div
                        className={`bulk-schema-box${bulkSchemaRestrict ? ' bulk-schema-box--active' : ''}`}
                      >
                        {!bulkSchemaRestrict ? (
                          <p className="bulk-schema-placeholder">Bu veri kaynağındaki tüm şemalar dahil.</p>
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
                              aria-label="Dahil edilecek şemalar"
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
                                Tümü
                              </button>
                              <button
                                type="button"
                                className="btn btn-ghost btn-sm"
                                onClick={() => setBulkSchemasSelected([])}
                              >
                                Hiçbiri
                              </button>
                              <span className="bulk-schema-hint">Çoklu seçim için Ctrl/Cmd+tıklayın</span>
                            </div>
                          </>
                        )}
                      </div>
                    </fieldset>
                  </div>
                  <div className="bulk-options-row">
                    <div className="form-group bulk-opt-field">
                      <label className="bulk-opt-label" htmlFor="bulk-sample-size">Örnek satır</label>
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
                      <span>Açıklaması olan tabloları atla</span>
                    </label>
                  </div>
                  <div className="bulk-scope-footer">
                    <span className="bulk-scope-stat">
                      Kapsamda <strong>{bulkTargetTables.length}</strong> nesne
                      {bulkTargetTables.length !== tables.length && (
                        <span className="bulk-scope-of"> · toplam {tables.length}</span>
                      )}
                    </span>
                  </div>
                  <div className="modal-actions">
                    <button type="button" className="btn btn-ghost btn-sm" onClick={closeBulk}>İptal</button>
                    <button
                      type="button"
                      className="btn btn-sm"
                      onClick={runBulkDescribe}
                      disabled={!bulkCanStart}
                    >
                      Başlat ({bulkTargetTables.length} kapsamda)
                    </button>
                  </div>
                </>
              )}

              {bulkEntries.length > 0 && (
                <>
                  <BulkProgressHeader entries={bulkEntries} running={bulkRunning} summary={bulkSummary} />
                  <div className="bulk-describe-scroll">
                    <table className="results-table results-table--dense" style={{ margin: 0 }}>
                      <thead>
                        <tr>
                          <th className="bulk-col-idx">#</th>
                          <th>Şema.Tablo</th>
                          <th className="bulk-col-status">Durum</th>
                          <th>Detay</th>
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
                                {e.message || (e.status === 'pending' ? '—' : '')}
                              </span>
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                  <div className="modal-actions">
                    {bulkRunning ? (
                      <button
                        type="button"
                        className="btn btn-ghost btn-sm"
                        onClick={() => { bulkCancelRef.current = true }}
                      >
                        Bu işten sonra durdur
                      </button>
                    ) : (
                      <button type="button" className="btn btn-sm" onClick={closeBulk}>Kapat</button>
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
                  🤖 AI Açıkla — {describeOpen.schema_name}.{describeOpen.table_name}
                </h2>
                <ModelBadgeRow
                  primaryLabel="Açıklama"
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
                aria-label="AI Açıkla'yı kapat"
                onClick={() => setDescribeOpen(null)}
              >
                ×
              </button>
            </header>

            <div className="modal-body">
              <p style={{ color: 'var(--text-secondary)', margin: 0 }}>
                Kaynak veritabanından N satır örnekler ve LLM'den tablo ile her kolonu Türkçe açıklamasını ister; şema eşleştirmesi için yararlı teknik adlar korunur.
              </p>

              {!describeResult && (
                <>
                  <div className="modal-form-row">
                    <div className="form-group">
                      <label htmlFor="describe-sample-size">Örnek boyutu</label>
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
                      <label>Seçenekler</label>
                      <div className="checkbox-row">
                        <input
                          id="auto-apply"
                          name="auto_apply"
                          type="checkbox"
                          checked={describeForm.auto_apply}
                          onChange={(e) => setDescribeForm({ ...describeForm, auto_apply: e.target.checked })}
                        />
                        <label htmlFor="auto-apply">Önerileri otomatik uygula</label>
                      </div>
                    </div>
                  </div>
                  {error && <div className="error">{error}</div>}
                  <div className="modal-actions">
                    <button
                      type="button"
                      className="btn btn-ghost btn-sm"
                      onClick={() => setDescribeOpen(null)}
                    >
                      İptal
                    </button>
                    <button
                      type="button"
                      className="btn btn-sm"
                      onClick={runDescribe}
                      disabled={loading}
                    >
                      {loading ? 'Analiz ediliyor…' : 'Türkçe Açıklama Üret'}
                    </button>
                  </div>
                </>
              )}

              {describeResult && (
                <>
                  {describeResult.model && (
                    <div style={{ fontSize: '0.8rem', color: 'var(--text-secondary)' }}>
                      Açıklama modeli: <code translate="no">{describeResult.model}</code>
                      {describeResult.translation_applied && describeResult.translation_model ? (
                        <> · Çeviri: <code translate="no">{describeResult.translation_model}</code></>
                      ) : null}
                    </div>
                  )}
                  <p style={{ color: 'var(--text-secondary)', margin: 0 }}>
                    {describeResult.sample_rows} satır örneklendi.{' '}
                    {describeResult.applied
                      ? <span className="success">Tüm öneriler uygulandı.</span>
                      : 'İnceleyip seçerek uygulayın.'}
                  </p>
                  {describeResult.translation_error && (
                    <p style={{ margin: 0, color: 'var(--error)' }}>
                      Çeviri katmanı başarısız oldu; özgün Türkçe öncelikli AI açıklamaları gösteriliyor. {describeResult.translation_error}
                    </p>
                  )}

                  <div>
                    <h3 style={{ marginBottom: '0.4rem' }}>Tablo açıklaması</h3>
                    <div className="suggestion-block">
                      {describeResult.description || <em style={{ color: 'var(--text-secondary)' }}>(yok)</em>}
                    </div>
                    {!describeResult.applied && describeResult.description && (
                      <div className="modal-actions">
                        <button
                          type="button"
                          className="btn btn-sm"
                          onClick={() => applySuggestion('table', '', describeResult.description)}
                        >
                          Tabloya uygula
                        </button>
                      </div>
                    )}
                  </div>

                  <div>
                    <h3 style={{ marginBottom: '0.4rem' }}>Kolonlar</h3>
                    <table className="results-table">
                      <thead>
                        <tr>
                          <th>Kolon</th>
                          <th>Öneri</th>
                          {!describeResult.applied && <th style={{ textAlign: 'right' }}>İşlem</th>}
                        </tr>
                      </thead>
                      <tbody>
                        {describeResult.columns.map((c) => (
                          <tr key={c.name}>
                            <td><code>{c.name}</code></td>
                            <td>{c.description || <em style={{ color: 'var(--text-secondary)' }}>(yok)</em>}</td>
                            {!describeResult.applied && (
                              <td className="actions">
                                {c.description && (
                                  <button
                                    type="button"
                                    className="btn btn-sm"
                                    onClick={() => applySuggestion('column', c.name, c.description)}
                                  >
                                    Uygula
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
                      Kapat
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
