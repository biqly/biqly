import { useCallback, useMemo, useState } from 'react'
import { useNavigate } from 'react-router-dom'

import { useApi } from '../../hooks/useApi'
import { useDatasources } from '../../hooks/useDatasources'
import { useModelDetail } from '../../hooks/useModelDetail'
import { useSemanticModels } from '../../hooks/useSemanticModels'
import { useLocale, useT } from '../../i18n'
import type { SemanticDimension, SemanticModelDetail } from '../../types/semantic'
import { pickPublishedModelId, pickValidIdOrFirst } from '../../utils/effectiveSelection'
import { localeNumberTag } from '../../utils/formatters'
import { columnRefMatchesTable, splitTableKey, tableKey } from '../modeling/utils'
import { useTableBrowserFilterState } from './useTableBrowserFilterState'
import { useTableBrowserQueryState } from './useTableBrowserQueryState'

const PAGE_SIZE_OPTIONS = [25, 50, 100] as const

export interface DetailRowState {
  displayIndex: number
  row: unknown[]
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
  const { postData, error } = useApi()

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
  const [detailRow, setDetailRow] = useState<DetailRowState | null>(null)

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

  const activeDimensions = useMemo(() => {
    if (!modelDetail || !selectedTableKey) {
      return []
    }
    const { schema, table } = splitTableKey(selectedTableKey)
    return (modelDetail.dimensions ?? []).filter(
      (d) =>
        d.is_active !== false &&
        columnRefMatchesTable(d.column_ref, schema, table, modelDetail.base_schema),
    )
  }, [modelDetail, selectedTableKey])

  const dimensionNamesKey = useMemo(
    () =>
      activeDimensions
        .map((d) => d.name)
        .sort()
        .join('\0'),
    [activeDimensions],
  )

  const filterState = useTableBrowserFilterState({
    activeDimensions,
    t,
    onFiltersChange,
    dimensionNamesKey,
    modelId,
    selectedTableKey,
  })

  const orderedDimensions = useMemo(() => {
    const byName = new Map(activeDimensions.map((d) => [d.name, d]))
    const ordered: SemanticDimension[] = []
    for (const name of filterState.columnOrder) {
      const d = byName.get(name)
      if (d) {
        ordered.push(d)
      }
    }
    for (const d of activeDimensions) {
      if (!filterState.columnOrder.includes(d.name)) {
        ordered.push(d)
      }
    }
    return ordered
  }, [activeDimensions, filterState.columnOrder])

  const queryState = useTableBrowserQueryState({
    datasourceId,
    modelId,
    modelMetrics: modelDetail?.metrics,
    orderedDimensions,
    filterPayload: filterState.filterPayload,
    columnOrder: filterState.columnOrder,
    postData,
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
    setSelectedTableKey,
    tableOptions,
    activeDimensions,
    error,
    detailRow,
    setDetailRow,
    openModeling,
    pageSizeOptions,
    formatInt,
    ...filterState,
    ...queryState,
    displayColumnNames: queryState.displayColumnNames,
    getDimensionLabel: filterState.getDimensionLabel,
  }
}
