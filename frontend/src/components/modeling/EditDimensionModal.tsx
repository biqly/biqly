import { useState, useMemo, useEffect } from 'react'

import type { useT } from '../../i18n'
import type {
  ColumnRow,
  SemanticDimension,
  SemanticModelDetail,
  TableRow,
  SemanticExprNode,
} from '../../types/semantic'
import { Modal } from '../ui/Modal'
import { Select } from '../ui/Select'
import { ExpressionBuilder } from './ExpressionBuilder'

export interface EditDimensionModalProps {
  model: SemanticModelDetail
  includedTables: TableRow[]
  columns: ColumnRow[]
  dimension: SemanticDimension
  onClose: () => void
  onSaved: () => void | Promise<void>
  putData: (url: string, body: unknown) => Promise<unknown>
  t: ReturnType<typeof useT>
}

export function EditDimensionModal({
  model,
  includedTables,
  columns,
  dimension,
  onClose,
  onSaved,
  putData,
  t,
}: EditDimensionModalProps) {
  const [label, setLabel] = useState(dimension.label || '')
  const [type, setType] = useState(dimension.type)
  const [sourceMode, setSourceMode] = useState<'column' | 'calculated'>(
    dimension.calculated_expression ? 'calculated' : 'column'
  )
  const [saving, setSaving] = useState(false)

  // Column ref fields
  const parts = dimension.column_ref.split('.')
  let initialSchema: string = model.base_schema
  let initialTable: string = model.base_table
  let initialColumn = ''
  if (parts.length === 3) {
    initialSchema = parts[0] || model.base_schema
    initialTable = parts[1] || model.base_table
    initialColumn = parts[2] || ''
  } else if (parts.length === 2) {
    initialTable = parts[0] || model.base_table
    initialColumn = parts[1] || ''
  } else if (parts.length === 1) {
    initialColumn = parts[0] || ''
  }

  const [selectedSchema, setSelectedSchema] = useState(initialSchema)
  const [selectedTable, setSelectedTable] = useState(initialTable)
  const [selectedColumn, setSelectedColumn] = useState(initialColumn)

  // Calculated expression fields
  const [calculatedExpression, setCalculatedExpression] = useState(
    dimension.calculated_expression || ''
  )
  const [calculatedExpr, setCalculatedExpr] = useState<SemanticExprNode | undefined>(
    dimension.calculated_expr
  )

  const modelTableKeys = useMemo(() => {
    const keys = new Set<string>()
    if (model) {
      keys.add(`${model.base_schema}.${model.base_table}`)
      ;(model.joins ?? []).forEach((j) => {
        if (j.is_active !== false) {
          keys.add(`${j.from_schema || model.base_schema}.${j.from_table}`)
          keys.add(`${j.to_schema || model.base_schema}.${j.to_table}`)
        }
      })
    }
    return keys
  }, [model])

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

  const submit = async () => {
    if (!label.trim()) {
      return
    }

    let finalColumnRef = dimension.column_ref
    let finalExprStr = ''
    let finalExprAst: SemanticExprNode | undefined = undefined

    if (sourceMode === 'column') {
      if (!selectedColumn) {
        return
      }
      finalColumnRef =
        selectedSchema === model.base_schema
          ? `${selectedTable}.${selectedColumn}`
          : `${selectedSchema}.${selectedTable}.${selectedColumn}`
    } else {
      if (!calculatedExpression.trim()) {
        return
      }
      finalExprStr = calculatedExpression.trim()
      finalExprAst = calculatedExpr
    }

    setSaving(true)
    try {
      await putData(`/api/semantic/models/${model.id}/dimensions/${dimension.id}`, {
        name: dimension.name,
        label: label.trim(),
        column_ref: finalColumnRef,
        type: type,
        synonyms: dimension.synonyms ?? [],
        description: dimension.description ?? '',
        is_active: dimension.is_active,
        calculated_expression: finalExprStr,
        calculated_expr: finalExprAst,
      })
      await onSaved()
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal
      open
      onClose={onClose}
      closeOnBackdrop={!saving}
      className={sourceMode === 'calculated' ? 'modal-card--metric' : 'modal-card--modeling'}
      labelledBy="modeling-edit-dimension-title"
      title={t('modeling.edit_dimension_title')}
    >
      <form
        onSubmit={(event) => {
          event.preventDefault()
          void submit()
        }}
      >
        <div className="modal-form-row">
          <div className="form-group">
            <label htmlFor="dim-name">{t('modeling.metric_name_label')}</label>
            <input id="dim-name" value={dimension.name} disabled readOnly />
          </div>
          <div className="form-group">
            <label htmlFor="dim-label">{t('modeling.metric_label_label')}</label>
            <input
              id="dim-label"
              autoFocus
              value={label}
              onChange={(e) => setLabel(e.target.value)}
              disabled={saving}
              autoComplete="off"
            />
          </div>
        </div>

        <div className="modal-form-row">
          <div className="form-group">
            <label htmlFor="dim-type">{t('modeling.display_name_label')}</label>
            <Select
              id="dim-type"
              name="type"
              value={type}
              onChange={(val) => setType(val)}
              disabled={saving}
              options={[
                { value: 'string', label: 'string' },
                { value: 'number', label: 'number' },
                { value: 'boolean', label: 'boolean' },
                { value: 'date', label: 'date' },
                { value: 'timestamp', label: 'timestamp' },
              ]}
            />
          </div>
          <div className="form-group">
            <label htmlFor="dim-source-mode">{t('modeling.join_type_label')}</label>
            <Select
              id="dim-source-mode"
              name="sourceMode"
              value={sourceMode}
              onChange={(val) => setSourceMode(val)}
              disabled={saving}
              options={[
                { value: 'column', label: t('modeling.simple_metric') },
                { value: 'calculated', label: t('modeling.custom_expression') },
              ]}
            />
          </div>
        </div>

        {sourceMode === 'column' ? (
          <>
            <div className="modal-form-row">
              <div className="form-group">
                <label htmlFor="dim-schema">{t('modeling.pick_schema')}</label>
                <Select
                  id="dim-schema"
                  name="schema"
                  value={selectedSchema}
                  onChange={(val) => setSelectedSchema(val)}
                  disabled={saving}
                  options={availableSchemas.map((s) => ({ value: s, label: s }))}
                />
              </div>
              <div className="form-group">
                <label htmlFor="dim-table">{t('modeling.pick_table')}</label>
                <Select
                  id="dim-table"
                  name="table"
                  value={selectedTable}
                  onChange={(val) => setSelectedTable(val)}
                  disabled={saving}
                  options={availableTables.map((tbl) => ({
                    value: tbl.table_name,
                    label: tbl.label || tbl.table_name,
                  }))}
                />
              </div>
            </div>
            <div className="modal-form-row">
              <div className="form-group">
                <label htmlFor="dim-column">{t('modeling.pick_column')}</label>
                <Select
                  id="dim-column"
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
            </div>
          </>
        ) : (
          <div className="form-group" style={{ display: 'block', width: '100%' }}>
            <label htmlFor="dim-expression">
              {t('modeling.metric_expression_label')}
            </label>
            <ExpressionBuilder
              model={model}
              columns={columns}
              initialNode={calculatedExpr}
              initialText={calculatedExpression}
              onChange={(node, textExpr) => {
                setCalculatedExpression(textExpr)
                setCalculatedExpr(node)
              }}
              t={(key, vars) => t(key as any, vars)}
            />
          </div>
        )}

        <div className="modal-actions">
          <button className="btn btn-secondary" type="button" onClick={onClose} disabled={saving}>
            {t('common.cancel')}
          </button>
          <button
            className="btn btn-primary"
            type="submit"
            disabled={
              saving || !label.trim() || (sourceMode === 'column' ? !selectedColumn : !calculatedExpression.trim())
            }
          >
            {saving ? t('common.saving') : t('common.save')}
          </button>
        </div>
      </form>
    </Modal>
  )
}

EditDimensionModal.displayName = 'EditDimensionModal'

