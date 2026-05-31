import { Fragment, useEffect, useMemo, useRef, useState } from 'react'
import { useApi } from '../hooks/useApi'
import { jobIsActive, useAIJobs } from '../hooks/useAIJobs'
import { type DescribeResult } from '../api/metadataDescribe'
import { useQueryParam } from '../hooks/useQueryParam'
import { FALLBACK_LOCALE, LOCALE_OPTIONS, SUPPORTED_LOCALES, useLocale, useT } from '../i18n'
import type { Locale } from '../i18n'
import type { Datasource } from '../types/metadata'
import type { ColumnRow, TableRow } from '../types/semantic'
import { ErrorAlert } from './ui/ErrorAlert'
import { Select } from './ui/Select'
import { LoadingScreen } from './ui/LoadingScreen'
import type { AIRuntimeSettings } from '../types/ai'
import { MetadataBulkDescribeModal } from './metadata/MetadataBulkDescribeModal'
import { MetadataColumnPanel } from './metadata/MetadataColumnPanel'
import { MetadataDescribeModal } from './metadata/MetadataDescribeModal'
import { MetadataDescriptionCell } from './metadata/MetadataDescriptionCell'
import type { MetadataEditingState } from './metadata/utils'

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
  const [editing, setEditing] = useState<MetadataEditingState | null>(null)
  const [describeOpen, setDescribeOpen] = useState<TableRow | null>(null)
  const [bulkOpen, setBulkOpen] = useState(false)
  const { running: bulkRunning, entries: bulkEntries, summary: bulkSummary, start: startBulkDescribe, cancel: cancelBulkDescribe } =
    bulkDescribe
  const skipBlurSaveRef = useRef(false)
  const [tableFilterSchema, setTableFilterSchema] = useState(schemaParam)
  const [tableFilterType, setTableFilterType] = useState(typeParam)
  const [aiRuntime, setAiRuntime] = useState<AIRuntimeSettings | null>(null)

  const [initLoading, setInitLoading] = useState(true)
  const [tablesLoading, setTablesLoading] = useState(false)

  useEffect(() => {
    setInitLoading(true)
    Promise.all([
      get<Datasource[]>('/api/datasources').then((data) => {
        if (!data) return
        setDatasources(data)
        setDatasourceId((prev) => {
          if (prev && data.some((d) => d.id === prev)) return prev
          return data[0]?.id ?? ''
        })
      }),
      get<AIRuntimeSettings>('/api/ai/settings').then((data) => {
        if (data) setAiRuntime(data)
      })
    ]).finally(() => {
      setInitLoading(false)
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

  const prevDsRef = useRef(datasourceId)

  useEffect(() => {
    if (!datasourceId) return
    setTablesLoading(true)
    get<TableRow[]>(`/api/datasources/${datasourceId}/tables`, descriptionLocaleOpts)
      .then((data) => setTables(data || []))
      .finally(() => setTablesLoading(false))
    setOpenTableId(null)
    setColumns([])
    if (prevDsRef.current && prevDsRef.current !== datasourceId) {
      setTableFilterSchema('')
      setTableFilterType('')
    }
    prevDsRef.current = datasourceId
  }, [datasourceId, editLocale, descriptionLocaleOpts, get])

  const schemaOptions = useMemo(
    () => [...new Set(tables.map((tab) => tab.schema_name))].sort((a, b) => a.localeCompare(b)),
    [tables],
  )
  const typeOptions = useMemo(
    () => [...new Set(tables.map((tab) => tab.table_type))].sort((a, b) => a.localeCompare(b)),
    [tables],
  )
  const filteredTables = useMemo(
    () =>
      tables.filter((tab) => {
        if (tableFilterSchema && tab.schema_name !== tableFilterSchema) return false
        if (tableFilterType && tab.table_type !== tableFilterType) return false
        return true
      }),
    [tables, tableFilterSchema, tableFilterType],
  )

  useEffect(() => {
    if (!openTableId) return
    if (!filteredTables.some((tab) => tab.id === openTableId)) {
      setOpenTableId(null)
      setColumns([])
    }
  }, [filteredTables, openTableId])

  useEffect(() => {
    if (!datasourceId || !openTableId) return
    const tab = tables.find((row) => row.id === openTableId)
    if (!tab) return
    get<ColumnRow[]>(
      `/api/datasources/${datasourceId}/columns?schema=${encodeURIComponent(tab.schema_name)}&table=${encodeURIComponent(tab.table_name)}`,
      descriptionLocaleOpts,
    ).then((data) => setColumns(data || []))
  }, [datasourceId, openTableId, editLocale, descriptionLocaleOpts, get, tables])

  const activeDescribeBatchJob = useMemo(
    () => jobs.find((j) => j.kind === 'describe_batch' && jobIsActive(j)),
    [jobs],
  )

  const refreshTables = () => {
    void get<TableRow[]>(`/api/datasources/${datasourceId}/tables`, descriptionLocaleOpts).then((fresh) => {
      if (fresh) setTables(fresh)
    })
  }

  const toggleTable = async (tab: TableRow) => {
    if (openTableId === tab.id) {
      setOpenTableId(null)
      setColumns([])
      return
    }
    setOpenTableId(tab.id)
    const data = await get<ColumnRow[]>(
      `/api/datasources/${datasourceId}/columns?schema=${encodeURIComponent(tab.schema_name)}&table=${encodeURIComponent(tab.table_name)}`,
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
      const res = await patchData(`/api/metadata/${entityPath}/${editing.id}`, { description: value })
      ok = !!res
    } else {
      const body: Record<string, Record<string, string>> = {
        [editLocale]: { description: value ?? '' },
      }
      const res = await putData(`/api/metadata/${entityPath}/${editing.id}/translations`, body)
      ok = !!res
    }
    if (ok) {
      if (editing.kind === 'table') {
        setTables(tables.map((row) => (row.id === editing.id ? { ...row, description: value } : row)))
      } else {
        setColumns(columns.map((c) => (c.id === editing.id ? { ...c, description: value } : c)))
      }
    }
    setEditing(null)
  }

  const refreshDescribeTarget = (row: TableRow) => {
    refreshTables()
    if (openTableId === row.id) {
      get<ColumnRow[]>(
        `/api/datasources/${datasourceId}/columns?schema=${encodeURIComponent(row.schema_name)}&table=${encodeURIComponent(row.table_name)}`,
        descriptionLocaleOpts,
      ).then((d) => setColumns(d || []))
    }
  }

  const patchDescribeDescription = async (kind: 'table' | 'column', id: string, description: string) => {
    const entityPath = kind === 'table' ? 'tables' : 'columns'
    await patchData(`/api/metadata/${entityPath}/${id}`, { description })
    if (kind === 'table') {
      setTables(tables.map((row) => (row.id === id ? { ...row, description } : row)))
    } else {
      setColumns(columns.map((c) => (c.id === id ? { ...c, description } : c)))
    }
  }

  const runDescribeJob = (
    request: {
      datasource_id: string
      schema: string
      table: string
      sample_size: number
      auto_apply: boolean
    },
    onError: (message: string) => void,
  ) =>
    runJob<typeof request, DescribeResult>('describe', request, { onError })

  if (initLoading && datasources.length === 0) {
    return <LoadingScreen minHeight="300px" />
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
                  onClick={() => setBulkOpen(true)}
                  disabled={bulkRunning || !!activeDescribeBatchJob}
                >
                  {t('metadata.bulk_ai_btn')}
                </button>
              )}
            </div>
          </div>
          {tablesLoading && tables.length === 0 ? (
            <LoadingScreen minHeight="150px" />
          ) : tables.length === 0 && !loading ? (
            <p className="metadata-empty-hint">
              {t('metadata.no_tables_before')}
              <strong>{t('datasources.sync')}</strong>
              {t('metadata.no_tables_after')}
            </p>
          ) : null}

          {(!tablesLoading || tables.length > 0) && tables.length > 0 && (
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
                          onClick={() => void toggleTable(tab)}
                        >
                          <span className="chevron">{openTableId === tab.id ? '▼' : '▶'}</span>
                          {tab.schema_name}.{tab.table_name}
                        </button>
                      </td>
                      <td className="metadata-col-type">{tab.table_type}</td>
                      <MetadataDescriptionCell
                        kind="table"
                        entityId={tab.id}
                        description={tab.description}
                        editing={editing}
                        placeholder={t('metadata.placeholder_double_click')}
                        onStartEdit={() => {
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
                        <button type="button" className="btn btn-sm" onClick={() => setDescribeOpen(tab)}>
                          {t('metadata.btn_ai_describe')}
                        </button>
                      </td>
                    </tr>
                    {openTableId === tab.id && columns.length > 0 && (
                      <MetadataColumnPanel
                        table={tab}
                        columns={columns}
                        locale={locale}
                        editing={editing}
                        onStartEdit={(c) => {
                          skipBlurSaveRef.current = false
                          setEditing({ kind: 'column', id: c.id, value: c.description ?? '' })
                        }}
                        onEditChange={(columnId, value) => setEditing({ kind: 'column', id: columnId, value })}
                        onSave={() => void saveDescription()}
                        onCancelEdit={() => {
                          skipBlurSaveRef.current = true
                          setEditing(null)
                        }}
                      />
                    )}
                  </Fragment>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}

      {bulkOpen && (
        <MetadataBulkDescribeModal
          open={bulkOpen}
          onClose={() => setBulkOpen(false)}
          datasourceId={datasourceId}
          tables={tables}
          schemaOptions={schemaOptions}
          typeOptions={typeOptions}
          aiRuntime={aiRuntime}
          bulkRunning={bulkRunning}
          bulkEntries={bulkEntries}
          bulkSummary={bulkSummary}
          activeDescribeBatchJob={activeDescribeBatchJob}
          onStartBulk={({ targets, sampleSize, skipExisting, onConflict, onFinished }) => {
            startBulkDescribe({
              datasourceId,
              targets,
              sampleSize,
              skipExisting,
              skipExistingMessage: t('metadata.bulk_skip_has_desc'),
              networkErrorMessage: t('metadata.bulk_network_error'),
              okColumnsMessage: (cols) => t('metadata.bulk_ok_columns', { cols }),
              onConflict,
              onFinished,
            })
          }}
          onCancelBulk={cancelBulkDescribe}
          onRefreshTables={refreshTables}
        />
      )}

      {describeOpen && (
        <MetadataDescribeModal
          table={describeOpen}
          datasourceId={datasourceId}
          columns={columns}
          aiRuntime={aiRuntime}
          apiError={error}
          runDescribeJob={runDescribeJob}
          patchDescription={patchDescribeDescription}
          onClose={() => setDescribeOpen(null)}
          onApplied={refreshDescribeTarget}
        />
      )}
    </div>
  )
}
