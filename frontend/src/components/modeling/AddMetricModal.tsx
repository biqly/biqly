import { useEffect, useMemo, useState } from 'react'

import type { TranslationKey, useT } from '../../i18n'
import type {
  ColumnRow,
  SemanticExprNode,
  SemanticMetric,
  SemanticModelDetail,
  TableRow,
} from '../../types/semantic'
import { Modal } from '../ui/Modal'
import { Select } from '../ui/Select'
import { ExpressionBuilder } from './ExpressionBuilder'

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

export interface AddMetricModalProps {
  model: SemanticModelDetail
  includedTables: TableRow[]
  columns: ColumnRow[]
  metric?: SemanticMetric
  onClose: () => void
  onCreated: () => void | Promise<void>
  postData: (url: string, body: unknown) => Promise<unknown>
  putData?: (url: string, body: unknown) => Promise<unknown>
  t: ReturnType<typeof useT>
}

export function AddMetricModal({
  model,
  includedTables,
  columns,
  metric,
  onClose,
  onCreated,
  postData,
  putData,
  t,
}: AddMetricModalProps) {
  const [name, setName] = useState(metric ? metric.name : '')
  const [label, setLabel] = useState(metric ? (metric.label ?? '') : '')
  const [mode, setMode] = useState<'simple' | 'custom'>(
    metric ? (metric.aggregation === 'custom' ? 'custom' : 'simple') : 'simple',
  )
  const [saving, setSaving] = useState(false)

  // Simple Mode state
  const parts = metric && metric.aggregation !== 'custom' ? metric.expression.split('.') : []
  let initialSchema: string = model.base_schema
  let initialTable: string = model.base_table
  let initialColumn = ''
  if (parts.length === 3) {
    initialSchema = parts[0] ?? model.base_schema
    initialTable = parts[1] ?? model.base_table
    initialColumn = parts[2] ?? ''
  } else if (parts.length === 2) {
    initialTable = parts[0] ?? model.base_table
    initialColumn = parts[1] ?? ''
  } else if (parts.length === 1) {
    initialColumn = parts[0] ?? ''
  }

  const [selectedSchema, setSelectedSchema] = useState(initialSchema)
  const [selectedTable, setSelectedTable] = useState(initialTable)
  const [selectedColumn, setSelectedColumn] = useState(initialColumn)
  const [selectedAggregation, setSelectedAggregation] = useState<StandardAggregation>(
    metric && metric.aggregation !== 'custom' && isStandardAggregation(metric.aggregation)
      ? metric.aggregation
      : 'sum',
  )
  const [format, setFormat] = useState(metric ? (metric.format ?? '') : '')

  // Custom Mode state
  const [expression, setExpression] = useState(metric ? metric.expression : '')
  const [astNode, setAstNode] = useState<SemanticExprNode | undefined>(
    metric ? metric.expr : undefined,
  )

  // Get active tables in model
  const modelTableKeys = useMemo(() => {
    const keys = new Set<string>()
    keys.add(`${model.base_schema}.${model.base_table}`)
    ;(model.joins ?? []).forEach((j) => {
      if (j.is_active !== false) {
        keys.add(`${j.from_schema ?? model.base_schema}.${j.from_table}`)
        keys.add(`${j.to_schema ?? model.base_schema}.${j.to_table}`)
      }
    })
    return keys
  }, [model])

  // Simple Mode lists
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

  const availableTables = useMemo(() => {
    return includedTables.filter((t) => {
      return (
        t.schema_name === selectedSchema && modelTableKeys.has(`${t.schema_name}.${t.table_name}`)
      )
    })
  }, [includedTables, selectedSchema, modelTableKeys])

  const availableColumns = useMemo(() => {
    return columns.filter((c) => {
      return c.schema_name === selectedSchema && c.table_name === selectedTable
    })
  }, [columns, selectedSchema, selectedTable])

  const [isFirstRender, setIsFirstRender] = useState(true)
  useEffect(() => {
    setIsFirstRender(false)
  }, [])

  // Select first table/column when schema/table changes
  useEffect(() => {
    if (isFirstRender) {
      return
    }
    if (availableTables.length > 0) {
      const found = availableTables.find((t) => t.table_name === selectedTable)
      if (!found && availableTables[0]) {
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
        setSelectedColumn(availableColumns[0].column_name)
      }
    } else {
      setSelectedColumn('')
    }
  }, [selectedTable, availableColumns, selectedColumn, isFirstRender])

  // Sync Simple selection to Custom mode if they toggle tabs
  const handleModeChange = (newMode: 'simple' | 'custom') => {
    if (newMode === 'custom' && mode === 'simple' && selectedColumn) {
      const ref =
        selectedSchema === model.base_schema
          ? `${selectedTable}.${selectedColumn}`
          : `${selectedSchema}.${selectedTable}.${selectedColumn}`
      setExpression(`${selectedAggregation}([${ref}])`)
    }
    setMode(newMode)
  }

  const submit = async () => {
    if (!name.trim()) {
      return
    }

    let finalExpr = ''
    let finalAgg = ''
    let finalAst: SemanticExprNode | undefined = undefined

    if (mode === 'simple') {
      if (!selectedColumn) {
        return
      }
      const ref =
        selectedSchema === model.base_schema
          ? `${selectedTable}.${selectedColumn}`
          : `${selectedSchema}.${selectedTable}.${selectedColumn}`
      finalExpr = ref
      finalAgg = selectedAggregation
    } else {
      if (!expression.trim()) {
        return
      }
      finalExpr = expression.trim()
      finalAgg = 'custom'
      finalAst = astNode
    }

    setSaving(true)
    try {
      if (metric && putData) {
        await putData(`/api/semantic/models/${model.id}/metrics/${metric.id}`, {
          name: name.trim(),
          label: label.trim() || undefined,
          expression: finalExpr,
          aggregation: finalAgg,
          format: format.trim() || undefined,
          expr: finalAst,
        })
      } else {
        await postData(`/api/semantic/models/${model.id}/metrics`, {
          name: name.trim(),
          label: label.trim() || undefined,
          expression: finalExpr,
          aggregation: finalAgg,
          format: format.trim() || undefined,
          expr: finalAst,
        })
      }
      await onCreated()
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal
      open
      onClose={onClose}
      closeOnBackdrop={!saving}
      className={mode === 'custom' ? 'modal-card--metric' : 'modal-card--modeling'}
      labelledBy="modeling-add-metric-title"
      title={metric ? t('modeling.edit_metric_title') : t('modeling.add_metric_title')}
    >
      <form
        onSubmit={(event) => {
          event.preventDefault()
          void submit()
        }}
      >
        <div className="modal-form-row">
          <div className="form-group">
            <label htmlFor="metric-name">{t('modeling.metric_name_label')}</label>
            <input
              id="metric-name"
              autoFocus={!metric}
              value={name}
              onChange={(e) => setName(e.target.value)}
              disabled={saving || !!metric}
              autoComplete="off"
            />
          </div>
          <div className="form-group">
            <label htmlFor="metric-label">{t('modeling.metric_label_label')}</label>
            <input
              id="metric-label"
              autoFocus={!!metric}
              value={label}
              onChange={(e) => setLabel(e.target.value)}
              disabled={saving}
              autoComplete="off"
            />
          </div>
        </div>

        <div
          className="toggle-group metric-mode-toggle"
          role="tablist"
          aria-label={metric ? t('modeling.edit_metric_title') : t('modeling.add_metric_title')}
        >
          <button
            type="button"
            className={`toggle-btn ${mode === 'simple' ? 'active' : ''}`}
            onClick={() => handleModeChange('simple')}
            disabled={saving}
            role="tab"
            aria-selected={mode === 'simple'}
          >
            {t('modeling.simple_metric')}
          </button>
          <button
            type="button"
            className={`toggle-btn ${mode === 'custom' ? 'active' : ''}`}
            onClick={() => handleModeChange('custom')}
            disabled={saving}
            role="tab"
            aria-selected={mode === 'custom'}
          >
            {t('modeling.custom_expression')}
          </button>
        </div>

        {mode === 'simple' ? (
          <>
            <div className="modal-form-row">
              <div className="form-group">
                <label htmlFor="metric-schema">{t('modeling.pick_schema')}</label>
                <Select
                  id="metric-schema"
                  name="schema"
                  value={selectedSchema}
                  onChange={(val) => setSelectedSchema(val)}
                  disabled={saving}
                  options={availableSchemas.map((s) => ({ value: s, label: s }))}
                />
              </div>
              <div className="form-group">
                <label htmlFor="metric-table">{t('modeling.pick_table')}</label>
                <Select
                  id="metric-table"
                  name="table"
                  value={selectedTable}
                  onChange={(val) => setSelectedTable(val)}
                  disabled={saving}
                  options={availableTables.map((tbl) => ({
                    value: tbl.table_name,
                    label: tbl.label ?? tbl.table_name,
                  }))}
                />
              </div>
            </div>
            <div className="modal-form-row">
              <div className="form-group">
                <label htmlFor="metric-column">{t('modeling.pick_column')}</label>
                <Select
                  id="metric-column"
                  name="column"
                  value={selectedColumn}
                  onChange={(val) => setSelectedColumn(val)}
                  disabled={saving}
                  options={availableColumns.map((col) => ({
                    value: col.column_name,
                    label: `${col.column_name} (${col.data_type})`,
                  }))}
                />
              </div>
              <div className="form-group">
                <label htmlFor="metric-aggregation">{t('modeling.metric_aggregation_label')}</label>
                <Select
                  id="metric-aggregation"
                  name="aggregation"
                  value={selectedAggregation}
                  onChange={(value) => setSelectedAggregation(value)}
                  disabled={saving}
                  options={[...METRIC_AGGREGATION_OPTIONS]}
                />
              </div>
            </div>
          </>
        ) : (
          <div className="form-group" style={{ display: 'block', width: '100%' }}>
            <label htmlFor="metric-expression">{t('modeling.metric_expression_label')}</label>
            <ExpressionBuilder
              model={model}
              columns={columns}
              initialNode={astNode}
              initialText={expression}
              onChange={(node, textExpr) => {
                setExpression(textExpr)
                setAstNode(node)
              }}
              t={(key, vars) => t(key as TranslationKey, vars)}
            />
          </div>
        )}

        <div className="form-group">
          <label htmlFor="metric-format">{t('modeling.metric_format_label')}</label>
          <input
            id="metric-format"
            value={format}
            onChange={(e) => setFormat(e.target.value)}
            disabled={saving}
            placeholder="$#,##0.00"
            autoComplete="off"
          />
        </div>
        <div className="modal-actions">
          <button className="btn btn-secondary" type="button" onClick={onClose} disabled={saving}>
            {t('common.cancel')}
          </button>
          <button
            className="btn btn-primary"
            type="submit"
            disabled={
              saving || !name.trim() || (mode === 'simple' ? !selectedColumn : !expression.trim())
            }
          >
            {saving ? t('common.saving') : metric ? t('common.save') : t('common.create')}
          </button>
        </div>
      </form>
    </Modal>
  )
}
