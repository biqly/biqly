import { useEffect, useMemo, useRef, useState } from 'react'

import { fetchUserAIModels } from '../api/aiUserModels'
import { type DescribeResult } from '../api/metadataDescribe'
import { jobIsActive, useAIJobs } from '../hooks/useAIJobs'
import type { DescribeJobRequest } from '../hooks/useAIJobsUtils'
import { useApi } from '../hooks/useApi'
import { useQueryParam } from '../hooks/useQueryParam'
import type { Locale } from '../i18n'
import { FALLBACK_LOCALE, useLocale, useT } from '../i18n'
import { cardClass } from '../lib/cardClasses'
import { legacyFormClass } from '../lib/formClasses'
import { legacyLayoutClass } from '../lib/layoutClasses'
import type { AIRuntimeSettings } from '../types/ai'
import type { Datasource, RelationDetail } from '../types/metadata'
import type { ColumnRow, TableRow } from '../types/semantic'
import { MetadataAIJobsStrip } from './metadata/MetadataAIJobsStrip'
import { MetadataBulkDescribeModal } from './metadata/MetadataBulkDescribeModal'
import { MetadataDescribeModal } from './metadata/MetadataDescribeModal'
import { MetadataRelationsPanel } from './metadata/MetadataRelationsPanel'
import { filterMetadataTables } from './metadata/metadataTableFilters'
import { MetadataTablesPanel } from './metadata/MetadataTablesPanel'
import type { MetadataEditingState } from './metadata/utils'
import { ErrorAlert } from './ui/ErrorAlert'
import { LoadingScreen } from './ui/LoadingScreen'
import { Select } from './ui/Select'

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
  const [relations, setRelations] = useState<RelationDetail[]>([])
  const [relationsLoading, setRelationsLoading] = useState(false)
  const [openTableId, setOpenTableId] = useState<string | null>(null)
  const [columns, setColumns] = useState<ColumnRow[]>([])
  const [editing, setEditing] = useState<MetadataEditingState | null>(null)
  const [describeOpen, setDescribeOpen] = useState<TableRow | null>(null)
  const [bulkOpen, setBulkOpen] = useState(false)
  const {
    running: bulkRunning,
    entries: bulkEntries,
    summary: bulkSummary,
    start: startBulkDescribe,
    cancel: cancelBulkDescribe,
  } = bulkDescribe
  const skipBlurSaveRef = useRef(false)
  // Guards the async columns fetch in toggleTable against out-of-order
  // responses when the user switches tables quickly.
  const openTableReqRef = useRef(0)
  const [tableFilterSchema, setTableFilterSchema] = useState(schemaParam)
  const [tableFilterType, setTableFilterType] = useState(typeParam)
  const [aiRuntime, setAiRuntime] = useState<AIRuntimeSettings | null>(null)
  // The describe model actually used is resolved per-user (the user's preference
  // when set, otherwise the global default). Resolve it so the modal badge
  // reflects what will run, not just the global default.
  const [effectiveDescribeModel, setEffectiveDescribeModel] = useState<string | null>(null)

  const [initLoading, setInitLoading] = useState(true)
  const [tablesLoading, setTablesLoading] = useState(false)

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setInitLoading(true)
    void Promise.all([
      get<Datasource[]>('/api/datasources').then((data) => {
        if (!data) {
          return
        }
        setDatasources(data)
        setDatasourceId((prev) => {
          if (prev && data.some((d) => d.id === prev)) {
            return prev
          }
          return data[0]?.id ?? ''
        })
      }),
      get<AIRuntimeSettings>('/api/ai/settings').then((data) => {
        if (data) {
          setAiRuntime(data)
        }
      }),
    ]).finally(() => {
      setInitLoading(false)
    })
  }, [get])

  useEffect(() => {
    const controller = new AbortController()
    void fetchUserAIModels()
      .then((res) => {
        if (controller.signal.aborted) {
          return
        }
        const prefId = res.preferences.describe
        if (!prefId) {
          setEffectiveDescribeModel(null)
          return
        }
        const m = res.models.find((mm) => mm.id === prefId)
        setEffectiveDescribeModel(m ? m.display_name || m.model_id : null)
      })
      .catch(() => {
        if (!controller.signal.aborted) {
          setEffectiveDescribeModel(null)
        }
      })
    return () => {
      controller.abort()
    }
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
    if (!datasourceId) {
      return
    }
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setTablesLoading(true)
    void get<TableRow[]>(`/api/datasources/${datasourceId}/tables`, descriptionLocaleOpts)
      .then((data) => setTables(data ?? []))
      .finally(() => setTablesLoading(false))
    setRelationsLoading(true)
    void get<RelationDetail[]>(`/api/datasources/${datasourceId}/relations`)
      .then((data) => setRelations(data ?? []))
      .finally(() => setRelationsLoading(false))
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
  const commonSchemas = useMemo(
    () =>
      new Set([
        'public',
        'dbo',
        'pg_catalog',
        'information_schema',
        'guest',
        'sys',
        'mysql',
        'performance_schema',
      ]),
    [],
  )
  const sortedSchemas = useMemo(
    () => [
      ...schemaOptions.filter((s) => commonSchemas.has(s)).sort((a, b) => a.localeCompare(b)),
      ...schemaOptions.filter((s) => !commonSchemas.has(s)).sort((a, b) => a.localeCompare(b)),
    ],
    [schemaOptions, commonSchemas],
  )

  // Default to first common schema if none selected
  useEffect(() => {
    if (!tableFilterSchema && sortedSchemas.length > 0) {
      const first = sortedSchemas[0]
      if (first) {
        // eslint-disable-next-line react-hooks/set-state-in-effect
        setTableFilterSchema(first)
      }
    }
  }, [sortedSchemas, tableFilterSchema])
  const typeOptions = useMemo(
    () => [...new Set(tables.map((tab) => tab.table_type))].sort((a, b) => a.localeCompare(b)),
    [tables],
  )
  const filteredTables = useMemo(
    () => filterMetadataTables(tables, tableFilterSchema, tableFilterType),
    [tables, tableFilterSchema, tableFilterType],
  )
  const filteredRelations = useMemo(
    () =>
      tableFilterSchema
        ? relations.filter(
            (rel) => rel.from_schema === tableFilterSchema || rel.to_schema === tableFilterSchema,
          )
        : relations,
    [relations, tableFilterSchema],
  )

  useEffect(() => {
    if (!openTableId) {
      return
    }
    if (!filteredTables.some((tab) => tab.id === openTableId)) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setOpenTableId(null)
      setColumns([])
    }
  }, [filteredTables, openTableId])

  useEffect(() => {
    if (!datasourceId || !openTableId) {
      return
    }
    const tab = tables.find((row) => row.id === openTableId)
    if (!tab) {
      return
    }
    void get<ColumnRow[]>(
      `/api/datasources/${datasourceId}/columns?schema=${encodeURIComponent(tab.schema_name)}&table=${encodeURIComponent(tab.table_name)}`,
      descriptionLocaleOpts,
    ).then((data) => setColumns(data ?? []))
  }, [datasourceId, openTableId, editLocale, descriptionLocaleOpts, get, tables])

  const activeDescribeBatchJob = useMemo(
    () => jobs.find((j) => j.kind === 'describe_batch' && jobIsActive(j)),
    [jobs],
  )

  // Refresh descriptions when a bulk run finishes. Needed for runs resumed
  // after a page refresh, where the modal's onFinished callback no longer
  // exists; for normal runs the extra fetch is harmless.
  const prevBulkRunningRef = useRef(bulkRunning)
  useEffect(() => {
    const wasRunning = prevBulkRunningRef.current
    prevBulkRunningRef.current = bulkRunning
    if (!wasRunning || bulkRunning || !bulkSummary || !datasourceId) {
      return
    }
    void get<TableRow[]>(`/api/datasources/${datasourceId}/tables`, descriptionLocaleOpts).then(
      (fresh) => {
        if (fresh) {
          setTables(fresh)
        }
      },
    )
  }, [bulkRunning, bulkSummary, datasourceId, descriptionLocaleOpts, get])

  const refreshOpenColumns = () => {
    if (!openTableId) {
      return
    }
    const tab = tables.find((row) => row.id === openTableId)
    if (!tab) {
      return
    }
    void get<ColumnRow[]>(
      `/api/datasources/${datasourceId}/columns?schema=${encodeURIComponent(tab.schema_name)}&table=${encodeURIComponent(tab.table_name)}`,
      descriptionLocaleOpts,
    ).then((fresh) => setColumns(fresh ?? []))
  }

  const refreshTables = () => {
    void get<TableRow[]>(`/api/datasources/${datasourceId}/tables`, descriptionLocaleOpts).then(
      (fresh) => {
        if (fresh) {
          setTables(fresh)
        }
      },
    )
    refreshOpenColumns()
  }

  const toggleTable = async (tab: TableRow) => {
    if (openTableId === tab.id) {
      openTableReqRef.current++
      setOpenTableId(null)
      setColumns([])
      return
    }
    const reqId = ++openTableReqRef.current
    setOpenTableId(tab.id)
    const data = await get<ColumnRow[]>(
      `/api/datasources/${datasourceId}/columns?schema=${encodeURIComponent(tab.schema_name)}&table=${encodeURIComponent(tab.table_name)}`,
      descriptionLocaleOpts,
    )
    // Ignore a stale response if the user has since opened/closed another table.
    if (reqId === openTableReqRef.current) {
      setColumns(data ?? [])
    }
  }

  const saveDescription = async () => {
    if (skipBlurSaveRef.current) {
      skipBlurSaveRef.current = false
      return
    }
    if (!editing) {
      return
    }
    const entityPath = editing.kind === 'table' ? 'tables' : 'columns'
    const value = editing.value.trim() === '' ? null : editing.value
    let ok = false
    if (editLocale === FALLBACK_LOCALE) {
      const res = await patchData(`/api/metadata/${entityPath}/${editing.id}`, {
        description: value,
      })
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
        setTables(
          tables.map((row) => (row.id === editing.id ? { ...row, description: value } : row)),
        )
      } else {
        setColumns(columns.map((c) => (c.id === editing.id ? { ...c, description: value } : c)))
      }
    }
    setEditing(null)
  }

  const refreshDescribeTarget = (row: TableRow) => {
    refreshTables()
    if (openTableId === row.id) {
      void get<ColumnRow[]>(
        `/api/datasources/${datasourceId}/columns?schema=${encodeURIComponent(row.schema_name)}&table=${encodeURIComponent(row.table_name)}`,
        descriptionLocaleOpts,
      ).then((d) => setColumns(d ?? []))
    }
  }

  const patchDescribeDescription = async (
    kind: 'table' | 'column',
    id: string,
    description: string,
  ) => {
    const entityPath = kind === 'table' ? 'tables' : 'columns'
    if (editLocale === FALLBACK_LOCALE) {
      await patchData(`/api/metadata/${entityPath}/${id}`, { description })
    } else {
      await putData(`/api/metadata/${entityPath}/${id}/translations`, {
        [editLocale]: { description },
      })
    }
    if (kind === 'table') {
      setTables(tables.map((row) => (row.id === id ? { ...row, description } : row)))
    } else {
      setColumns(columns.map((c) => (c.id === id ? { ...c, description } : c)))
    }
  }

  const runDescribeJob = (request: DescribeJobRequest, onError: (message: string) => void) =>
    runJob<typeof request, DescribeResult>('describe', request, { onError })

  if (initLoading && datasources.length === 0) {
    return <LoadingScreen minHeight="300px" />
  }

  return (
    <div className={legacyLayoutClass('page-stack')}>
      <div className={cardClass()}>
        <div className="grid grid-cols-1 items-end gap-4 sm:grid-cols-3">
          <div className={legacyFormClass('form-group mb-0')}>
            <label>{t('metadata.datasource_label')}</label>
            <Select
              value={datasourceId}
              onChange={setDatasourceId}
              placeholder={t('query_builder.placeholder_pick_datasource')}
              header={t('query_builder.header_datasources')}
              options={datasources.map((d) => ({ value: d.id, label: d.name, hint: d.type }))}
            />
          </div>
          <div className={legacyFormClass('form-group mb-0')}>
            <label>{t('metadata.filter_schema_label')}</label>
            <Select
              id="metadata-filter-schema"
              size="sm"
              ariaLabel={t('metadata.filter_schema_aria')}
              value={tableFilterSchema}
              onChange={setTableFilterSchema}
              options={[
                { value: '', label: t('metadata.filter_all_schemas') },
                ...sortedSchemas.map((s) => ({ value: s, label: s })),
              ]}
            />
          </div>
          <div className={legacyFormClass('form-group mb-0')}>
            <label>{t('metadata.filter_type_label')}</label>
            <Select
              id="metadata-filter-type"
              size="sm"
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
        <ErrorAlert error={error} />
      </div>

      <MetadataAIJobsStrip />

      {datasourceId && (
        <MetadataTablesPanel
          t={t}
          locale={locale}
          editLocale={editLocale}
          onEditLocaleChange={setEditLocale}
          tables={tables}
          filteredTables={filteredTables}
          tablesLoading={tablesLoading}
          loading={loading}
          openTableId={openTableId}
          columns={columns}
          editing={editing}
          onBulkOpen={() => setBulkOpen(true)}
          bulkRunning={bulkRunning}
          activeDescribeBatchJob={activeDescribeBatchJob}
          onToggleTable={(tab) => void toggleTable(tab)}
          onStartEditTable={(tab) => {
            skipBlurSaveRef.current = false
            setEditing({ kind: 'table', id: tab.id, value: tab.description ?? '' })
          }}
          onEditTableChange={(id, value) => setEditing({ kind: 'table', id, value })}
          onSaveDescription={() => void saveDescription()}
          onCancelEdit={() => {
            skipBlurSaveRef.current = true
            setEditing(null)
          }}
          onDescribeOpen={setDescribeOpen}
          onStartEditColumn={(c) => {
            skipBlurSaveRef.current = false
            setEditing({ kind: 'column', id: c.id, value: c.description ?? '' })
          }}
          onEditColumnChange={(columnId, value) =>
            setEditing({ kind: 'column', id: columnId, value })
          }
          onSaveDisplayExpression={async (tab, expr) => {
            const res = await patchData<TableRow>(`/api/metadata/tables/${tab.id}`, {
              display_expression: expr,
            })
            if (!res) {
              return false
            }
            setTables((prev) =>
              prev.map((row) =>
                row.id === tab.id
                  ? { ...row, display_expression: res.display_expression ?? null }
                  : row,
              ),
            )
            return true
          }}
        />
      )}

      {datasourceId && (
        <MetadataRelationsPanel t={t} relations={filteredRelations} loading={relationsLoading} />
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
          describeModel={effectiveDescribeModel ?? undefined}
          bulkRunning={bulkRunning}
          bulkEntries={bulkEntries}
          bulkSummary={bulkSummary}
          activeDescribeBatchJob={activeDescribeBatchJob}
          onStartBulk={({ targets, sampleSize, skipExisting, onConflict, onFinished }) => {
            startBulkDescribe({
              datasourceId,
              targets,
              sampleSize,
              locale: editLocale,
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
          open={!!describeOpen}
          table={describeOpen}
          datasourceId={datasourceId}
          columns={columns}
          aiRuntime={aiRuntime}
          apiError={error}
          runDescribeJob={runDescribeJob}
          locale={editLocale}
          patchDescription={patchDescribeDescription}
          onClose={() => setDescribeOpen(null)}
          onApplied={refreshDescribeTarget}
        />
      )}
    </div>
  )
}
