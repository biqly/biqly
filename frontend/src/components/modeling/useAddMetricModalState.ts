import { useEffect, useMemo, useState } from 'react'

import type {
  ColumnRow,
  SemanticExprNode,
  SemanticMetric,
  SemanticModelDetail,
  TableRow,
} from '../../types/semantic'
import {
  buildMetricColumnRef,
  buildMetricSubmitPayload,
  parseMetricExpressionParts,
} from './addMetricUtils'
import { buildModelTableKeys } from './modelingTableCards'

const METRIC_AGGREGATION_OPTIONS = [
  { value: 'count', label: 'count' },
  { value: 'count_distinct', label: 'count_distinct' },
  { value: 'sum', label: 'sum' },
  { value: 'avg', label: 'avg' },
  { value: 'min', label: 'min' },
  { value: 'max', label: 'max' },
] as const

type StandardAggregation = (typeof METRIC_AGGREGATION_OPTIONS)[number]['value']

function isStandardAggregation(value: string): value is StandardAggregation {
  return METRIC_AGGREGATION_OPTIONS.some((opt) => opt.value === value)
}

export function useAddMetricModalState(
  model: SemanticModelDetail,
  includedTables: TableRow[],
  columns: ColumnRow[],
  metric: SemanticMetric | undefined,
) {
  const [name, setName] = useState(metric ? metric.name : '')
  const [label, setLabel] = useState(metric ? (metric.label ?? '') : '')
  const [mode, setMode] = useState<'simple' | 'custom'>(
    metric ? (metric.aggregation === 'custom' ? 'custom' : 'simple') : 'simple',
  )
  const [saving, setSaving] = useState(false)

  const initialParts = parseMetricExpressionParts(metric, model)

  const [selectedSchema, setSelectedSchema] = useState(initialParts.schema)
  const [selectedTable, setSelectedTable] = useState(initialParts.table)
  const [selectedColumn, setSelectedColumn] = useState(initialParts.column)
  const [selectedAggregation, setSelectedAggregation] = useState<StandardAggregation>(
    metric && metric.aggregation !== 'custom' && isStandardAggregation(metric.aggregation)
      ? metric.aggregation
      : 'sum',
  )
  const [format, setFormat] = useState(metric ? (metric.format ?? '') : '')
  const [rateBehavior, setRateBehavior] = useState(metric ? (metric.rate_behavior ?? '') : '')
  const [expression, setExpression] = useState(metric ? metric.expression : '')
  const [astNode, setAstNode] = useState<SemanticExprNode | undefined>(
    metric ? metric.expr : undefined,
  )

  const modelTableKeys = useMemo(() => {
    return buildModelTableKeys(model, includedTables)
  }, [model, includedTables])

  const availableSchemas = useMemo(() => {
    const schemas = new Set<string>()
    modelTableKeys.forEach((key) => {
      const parts = key.split('.')
      if (parts.length >= 2 && parts[0]) {
        schemas.add(parts[0])
      }
    })
    return Array.from(schemas).sort()
  }, [modelTableKeys])

  const availableTables = useMemo(
    () =>
      includedTables.filter(
        (tbl) =>
          tbl.schema_name === selectedSchema &&
          modelTableKeys.has(`${tbl.schema_name}.${tbl.table_name}`),
      ),
    [includedTables, selectedSchema, modelTableKeys],
  )

  const availableColumns = useMemo(
    () => columns.filter((c) => c.schema_name === selectedSchema && c.table_name === selectedTable),
    [columns, selectedSchema, selectedTable],
  )

  const [isFirstRender, setIsFirstRender] = useState(true)
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setIsFirstRender(false)
  }, [])

  useEffect(() => {
    if (isFirstRender) {
      return
    }
    if (availableTables.length > 0) {
      const found = availableTables.find((tbl) => tbl.table_name === selectedTable)
      if (!found && availableTables[0]) {
        // eslint-disable-next-line react-hooks/set-state-in-effect
        setSelectedTable(availableTables[0].table_name)
      }
    } else {
      setSelectedTable('')
    }
  }, [selectedSchema, availableTables, selectedTable, isFirstRender])

  useEffect(() => {
    if (isFirstRender) {
      return
    }
    if (availableColumns.length > 0) {
      const found = availableColumns.find((c) => c.column_name === selectedColumn)
      if (!found && availableColumns[0]) {
        // eslint-disable-next-line react-hooks/set-state-in-effect
        setSelectedColumn(availableColumns[0].column_name)
      }
    } else {
      setSelectedColumn('')
    }
  }, [selectedTable, availableColumns, selectedColumn, isFirstRender])

  const handleModeChange = (newMode: 'simple' | 'custom') => {
    if (newMode === 'custom' && mode === 'simple' && selectedColumn) {
      const ref = buildMetricColumnRef(model, selectedSchema, selectedTable, selectedColumn)
      setExpression(`${selectedAggregation}([${ref}])`)
    }
    setMode(newMode)
  }

  const buildSubmitBody = () => {
    if (!name.trim()) {
      return null
    }
    const payload = buildMetricSubmitPayload(mode, model, {
      name: name.trim(),
      label: label.trim(),
      format: format.trim(),
      selectedSchema,
      selectedTable,
      selectedColumn,
      selectedAggregation,
      expression,
      astNode,
    })
    if (!payload) {
      return null
    }
    return {
      name: name.trim(),
      label: label.trim() || undefined,
      expression: payload.expression,
      aggregation: payload.aggregation,
      format: format.trim() || undefined,
      expr: payload.expr,
      rate_behavior: rateBehavior || undefined,
    }
  }

  const canSubmit =
    !saving && !!name.trim() && (mode === 'simple' ? !!selectedColumn : !!expression.trim())

  return {
    name,
    setName,
    label,
    setLabel,
    mode,
    saving,
    setSaving,
    format,
    setFormat,
    rateBehavior,
    setRateBehavior,
    expression,
    setExpression,
    astNode,
    setAstNode,
    availableSchemas,
    availableTables,
    availableColumns,
    selectedSchema,
    setSelectedSchema,
    selectedTable,
    setSelectedTable,
    selectedColumn,
    setSelectedColumn,
    selectedAggregation,
    setSelectedAggregation,
    handleModeChange,
    buildSubmitBody,
    canSubmit,
  }
}
