import { useCallback, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import { useApi } from '../../hooks/useApi'
import { useDatasources } from '../../hooks/useDatasources'
import { useFetch } from '../../hooks/useFetch'
import { useModelDetail } from '../../hooks/useModelDetail'
import { useSemanticModels } from '../../hooks/useSemanticModels'
import { useLocale, useT } from '../../i18n'
import type { ColumnRow, SemanticModelDetail, TableRow } from '../../types/semantic'
import { pickPublishedModelId, pickValidIdOrFirst } from '../../utils/effectiveSelection'
import { localeNumberTag } from '../../utils/formatters'
import { splitTableKey, tableKey } from '../modeling/utils'
import { useTableBrowserFilterState } from './useTableBrowserFilterState'
import { useTableBrowserQueryState } from './useTableBrowserQueryState'

const PAGE_SIZE_OPTIONS = [25, 50, 100] as const

export interface DetailRowState {
  displayIndex: number
  row: unknown[]
}

export interface BrowserField {
  name: string
  label?: string | null
}

function collectModelTables(model: SemanticModelDetail): { value: string; label: string }[] {
  const seen = new Set<string>()
  const out: { value: string; label: string }[] = []
  const add = (schema: string, table: string) => {
    const key = tableKey(schema, table)
    if (seen.has(key)) {
      return
    }
    seen.add(key)
    out.push({ value: key, label: key })
  }
  add(model.base_schema, model.base_table)
  for (const j of model.joins ?? []) {
    if (j.is_active === false) {
      continue
    }
    add(j.from_schema ?? model.base_schema, j.from_table)
    add(j.to_schema ?? model.base_schema, j.to_table)
  }
  return out.sort((a, b) => a.label.localeCompare(b.label))
}

export function resolveSelectedTableKey(
  defaultTableKey: string,
  selectedTableKeyInput: string,
  tableOptions: readonly { value: string }[],
): string {
  if (!defaultTableKey) {
    return ''
  }
  if (
    selectedTableKeyInput &&
    tableOptions.some((option) => option.value === selectedTableKeyInput)
  ) {
    return selectedTableKeyInput
  }
  return defaultTableKey
}

export function useTableBrowserPage() {
  const navigate = useNavigate()
  const t = useT()
  const [locale] = useLocale()
  const localeTag = localeNumberTag(locale)
  const formatInt = useCallback((n: number) => n.toLocaleString(localeTag), [localeTag])
  const { get, postData } = useApi()

  const { datasources, loading: dsLoading } = useDatasources()
  const [selectedDatasourceId, setSelectedDatasourceId] = useState('')
  const datasourceId = useMemo(
    () => pickValidIdOrFirst(selectedDatasourceId, datasources),
    [selectedDatasourceId, datasources],
  )
  const { models, loading: modelsLoading } = useSemanticModels(datasourceId)
  const [selectedModelId, setSelectedModelId] = useState('')
  const modelId = useMemo(
    () => pickPublishedModelId(selectedModelId, models),
    [selectedModelId, models],
  )
  const { model: modelDetail, loading: modelLoading } = useModelDetail(modelId)
  const tableOptions = useMemo(() => {
    if (!modelDetail) {
      return []
    }
    return collectModelTables(modelDetail)
  }, [modelDetail])
  const defaultTableKey = modelDetail
    ? tableKey(modelDetail.base_schema, modelDetail.base_table)
    : ''
  const [selectedTableKeyInput, setSelectedTableKeyInput] = useState('')
  const selectedTableKey = useMemo(
    () => resolveSelectedTableKey(defaultTableKey, selectedTableKeyInput, tableOptions),
    [defaultTableKey, selectedTableKeyInput, tableOptions],
  )
  const { schema: selectedSchema, table: selectedTable } = useMemo(
    () => splitTableKey(selectedTableKey),
    [selectedTableKey],
  )
  const [detailRow, setDetailRow] = useState<DetailRowState | null>(null)

  // Physical columns of the selected table: the browser shows the table's own
  // data, independent of the semantic model's base table.
  const columnsScopeKey = `${datasourceId}:${selectedTableKey}`
  const { data: fetchedColumns } = useFetch(
    async () => {
      const data = await get<ColumnRow[]>(
        `/api/datasources/${datasourceId}/columns?schema=${encodeURIComponent(selectedSchema)}&table=${encodeURIComponent(selectedTable)}`,
      )
      return { key: columnsScopeKey, cols: data ?? [] }
    },
    [get, datasourceId, selectedSchema, selectedTable],
    { enabled: !!datasourceId && !!selectedSchema && !!selectedTable },
  )
  const tableColumnsState = useMemo(() => fetchedColumns ?? { key: '', cols: [] }, [fetchedColumns])
  const tableColumns = useMemo(
    () => (tableColumnsState.key === columnsScopeKey ? tableColumnsState.cols : []),
    [tableColumnsState, columnsScopeKey],
  )

  // Table metadata (description/label/display_expression) for modal titles.
  const { data: fetchedTablesMeta } = useFetch(
    async () => {
      const data = await get<TableRow[]>(`/api/datasources/${datasourceId}/tables`)
      return { key: datasourceId, tables: data ?? [] }
    },
    [get, datasourceId],
    { enabled: !!datasourceId },
  )
  const tablesMetaState = useMemo(
    () => fetchedTablesMeta ?? { key: '', tables: [] },
    [fetchedTablesMeta],
  )
  const tablesMeta = useMemo(
    () => (tablesMetaState.key === datasourceId ? tablesMetaState.tables : []),
    [tablesMetaState, datasourceId],
  )

  const displayExpressionByTable = useMemo(() => {
    const m = new Map<string, string>()
    for (const tab of tablesMeta) {
      if (tab.display_expression) {
        m.set(tableKey(tab.schema_name, tab.table_name), tab.display_expression)
      }
    }
    return m
  }, [tablesMeta])

  const setDatasourceId = useCallback((id: string) => {
    setSelectedDatasourceId(id)
    setSelectedModelId('')
    setSelectedTableKeyInput('')
    setDetailRow(null)
  }, [])

  const setModelId = useCallback((id: string) => {
    setSelectedModelId(id)
    setSelectedTableKeyInput('')
    setDetailRow(null)
  }, [])

  const setSelectedTableKey = useCallback((key: string) => {
    setSelectedTableKeyInput(key)
    setDetailRow(null)
  }, [])

  const onFiltersChange = useCallback(() => {
    setDetailRow(null)
  }, [])

  const browserFields: BrowserField[] = useMemo(
    () => tableColumns.map((c) => ({ name: c.column_name, label: c.column_name })),
    [tableColumns],
  )

  const fieldNamesKey = useMemo(
    () =>
      browserFields
        .map((d) => d.name)
        .sort()
        .join('\0'),
    [browserFields],
  )

  const filterState = useTableBrowserFilterState({
    activeDimensions: browserFields,
    t,
    onFiltersChange,
    dimensionNamesKey: fieldNamesKey,
    modelId,
    selectedTableKey,
  })

  const queryState = useTableBrowserQueryState({
    datasourceId,
    schema: selectedSchema,
    table: selectedTable,
    filterPayload: filterState.filterPayload,
    columnOrder: filterState.columnOrder,
    onPageReset: () => setDetailRow(null),
    filtersKey: filterState.filtersKey,
  })

  const pageSizeOptions = useMemo(
    () =>
      PAGE_SIZE_OPTIONS.map((n) => ({
        value: String(n),
        label: String(n),
      })),
    [],
  )

  const openModeling = useCallback(() => {
    const params = new URLSearchParams()
    if (datasourceId) {
      params.set('ds', datasourceId)
    }
    if (modelId) {
      params.set('model', modelId)
    }
    const qs = params.toString()
    void navigate(qs ? `/modeling?${qs}` : '/modeling')
  }, [datasourceId, modelId, navigate])

  const loading = dsLoading || modelsLoading || (modelId ? modelLoading : false)

  return {
    t,
    loading,
    datasources,
    datasourceId,
    setDatasourceId,
    models,
    modelId,
    setModelId,
    modelDetail,
    selectedTableKey,
    selectedSchema,
    selectedTable,
    setSelectedTableKey,
    tableOptions,
    browserFields,
    displayExpressionByTable,
    // error comes from queryState (...queryState below): the rows request's
    // selection-scoped error, so a stale/superseded "table not found" never
    // shows over freshly-loaded rows.
    detailRow,
    setDetailRow,
    openModeling,
    pageSizeOptions,
    formatInt,
    postData,
    ...filterState,
    ...queryState,
    displayColumnNames: queryState.displayColumnNames,
    getDimensionLabel: filterState.getDimensionLabel,
  }
}
