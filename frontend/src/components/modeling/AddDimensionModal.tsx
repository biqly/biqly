import { useEffect, useMemo, useState } from 'react'

import { useAutofocus } from '../../hooks/useAutofocus'
import type { TranslationKey, useT } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
import { modelingFormGroupClass } from '../../lib/formClasses'
import {
  modalActionsBorderedClass,
  modalFormRowClass,
  modalModelingCardClass,
} from '../../lib/modalClasses'
import { metricModeToggleClass, toggleBtnClass, toggleGroupClass } from '../../lib/toggleClasses'
import type {
  ColumnRow,
  SemanticExprNode,
  SemanticModelDetail,
  TableRow,
} from '../../types/semantic'
import { Modal } from '../ui/Modal'
import { Select } from '../ui/Select'
import { ExpressionBuilder } from './ExpressionBuilder'
import { buildModelTableKeys } from './modelingTableCards'

type DimensionSourceMode = 'column' | 'derived'

const TYPE_OPTIONS = [
  { value: 'string', label: 'string' },
  { value: 'number', label: 'number' },
  { value: 'boolean', label: 'boolean' },
  { value: 'date', label: 'date' },
  { value: 'timestamp', label: 'timestamp' },
]

export interface AddDimensionModalProps {
  model: SemanticModelDetail
  includedTables: TableRow[]
  columns: ColumnRow[]
  onClose: () => void
  onCreated: () => void | Promise<void>
  postData: (url: string, body: unknown) => Promise<unknown>
  t: ReturnType<typeof useT>
}

/**
 * AddDimensionModal creates a new dimension from scratch — either backed by a
 * single column or by a derived expression (concat/case/coalesce/…). It mirrors
 * AddMetricModal's layout and reuses ExpressionBuilder + the CreateDimension
 * endpoint (which already accepts calculated_expression / calculated_expr).
 */
export function AddDimensionModal({
  model,
  includedTables,
  columns,
  onClose,
  onCreated,
  postData,
  t,
}: AddDimensionModalProps) {
  const nameInputRef = useAutofocus<HTMLInputElement>(true)

  const [name, setName] = useState('')
  const [label, setLabel] = useState('')
  const [type, setType] = useState('string')
  const [mode, setMode] = useState<DimensionSourceMode>('column')
  const [saving, setSaving] = useState(false)

  const [selectedSchema, setSelectedSchema] = useState(model.base_schema)
  const [selectedTable, setSelectedTable] = useState(model.base_table)
  const [selectedColumn, setSelectedColumn] = useState('')

  const [derivedExpression, setDerivedExpression] = useState('')
  const [derivedExpr, setDerivedExpr] = useState<SemanticExprNode | undefined>(undefined)

  const modelTableKeys = useMemo(
    () => buildModelTableKeys(model, includedTables),
    [model, includedTables],
  )

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

  // Keep the table/column selections valid as the parent select changes. Skip
  // the first render so the initial base_schema/base_table stay intact.
  const [isFirstRender, setIsFirstRender] = useState(true)
  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setIsFirstRender(false)
  }, [])

  useEffect(() => {
    if (isFirstRender) {
      return
    }
    const found = availableTables.find((tbl) => tbl.table_name === selectedTable)
    if (!found) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setSelectedTable(availableTables[0]?.table_name ?? '')
    }
  }, [availableTables, selectedTable, isFirstRender])

  useEffect(() => {
    if (isFirstRender) {
      return
    }
    const found = availableColumns.find((c) => c.column_name === selectedColumn)
    if (!found) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setSelectedColumn(availableColumns[0]?.column_name ?? '')
    }
  }, [availableColumns, selectedColumn, isFirstRender])

  const canSubmit =
    !saving && !!name.trim() && (mode === 'column' ? !!selectedColumn : !!derivedExpression.trim())

  const submit = async () => {
    if (!canSubmit) {
      return
    }

    let columnRef = ''
    let calcExpression = ''
    let calcExpr: SemanticExprNode | undefined

    if (mode === 'column') {
      columnRef =
        selectedSchema === model.base_schema
          ? `${selectedTable}.${selectedColumn}`
          : `${selectedSchema}.${selectedTable}.${selectedColumn}`
    } else {
      calcExpression = derivedExpression.trim()
      calcExpr = derivedExpr
    }

    setSaving(true)
    try {
      await postData(`/api/semantic/models/${model.id}/dimensions`, {
        name: name.trim(),
        label: label.trim(),
        column_ref: columnRef,
        type,
        calculated_expression: calcExpression,
        calculated_expr: calcExpr,
      })
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
      className={
        mode === 'derived'
          ? 'w-full max-w-184 transition-[width] duration-200 ease-in-out'
          : modalModelingCardClass()
      }
      labelledBy="modeling-add-dimension-title"
      title={t('modeling.add_dimension_title')}
    >
      <form
        onSubmit={(event) => {
          event.preventDefault()
          void submit()
        }}
      >
        <div className={modalFormRowClass()}>
          <div className={modelingFormGroupClass}>
            <label htmlFor="add-dim-name">{t('modeling.metric_name_label')}</label>
            <input
              id="add-dim-name"
              ref={nameInputRef}
              value={name}
              onChange={(e) => setName(e.target.value)}
              disabled={saving}
              autoComplete="off"
            />
          </div>
          <div className={modelingFormGroupClass}>
            <label htmlFor="add-dim-label">{t('modeling.metric_label_label')}</label>
            <input
              id="add-dim-label"
              value={label}
              onChange={(e) => setLabel(e.target.value)}
              disabled={saving}
              autoComplete="off"
            />
          </div>
        </div>

        <div
          className={toggleGroupClass(metricModeToggleClass)}
          role="tablist"
          aria-label={t('modeling.add_dimension_title')}
        >
          <button
            type="button"
            className={toggleBtnClass(mode === 'column')}
            onClick={() => setMode('column')}
            disabled={saving}
            role="tab"
            aria-selected={mode === 'column'}
          >
            {t('modeling.dimension_source_column')}
          </button>
          <button
            type="button"
            className={toggleBtnClass(mode === 'derived')}
            onClick={() => setMode('derived')}
            disabled={saving}
            role="tab"
            aria-selected={mode === 'derived'}
          >
            {t('modeling.dimension_source_derived')}
          </button>
        </div>

        {mode === 'column' ? (
          <>
            <div className={modalFormRowClass()}>
              <div className={modelingFormGroupClass}>
                <label htmlFor="add-dim-schema">{t('modeling.pick_schema')}</label>
                <Select
                  id="add-dim-schema"
                  name="schema"
                  value={selectedSchema}
                  onChange={setSelectedSchema}
                  disabled={saving}
                  options={availableSchemas.map((s) => ({ value: s, label: s }))}
                />
              </div>
              <div className={modelingFormGroupClass}>
                <label htmlFor="add-dim-table">{t('modeling.pick_table')}</label>
                <Select
                  id="add-dim-table"
                  name="table"
                  value={selectedTable}
                  onChange={setSelectedTable}
                  disabled={saving}
                  options={availableTables.map((tbl) => ({
                    value: tbl.table_name,
                    label: tbl.label ?? tbl.table_name,
                  }))}
                />
              </div>
            </div>
            <div className={modalFormRowClass()}>
              <div className={modelingFormGroupClass}>
                <label htmlFor="add-dim-column">{t('modeling.pick_column')}</label>
                <Select
                  id="add-dim-column"
                  name="column"
                  value={selectedColumn}
                  onChange={setSelectedColumn}
                  disabled={saving}
                  options={availableColumns.map((col) => ({
                    value: col.column_name,
                    label: `${col.column_name} (${col.data_type})`,
                  }))}
                />
              </div>
              <div className={modelingFormGroupClass}>
                <label htmlFor="add-dim-type">{t('modeling.dimension_type_label')}</label>
                <Select
                  id="add-dim-type"
                  name="type"
                  value={type}
                  onChange={setType}
                  disabled={saving}
                  options={TYPE_OPTIONS}
                />
              </div>
            </div>
          </>
        ) : (
          <>
            <div className={modelingFormGroupClass} style={{ display: 'block', width: '100%' }}>
              <label htmlFor="add-dim-expression">{t('modeling.metric_expression_label')}</label>
              <ExpressionBuilder
                model={model}
                columns={columns}
                initialNode={derivedExpr}
                initialText={derivedExpression}
                onChange={(node, textExpr) => {
                  setDerivedExpression(textExpr)
                  setDerivedExpr(node)
                }}
                t={(key, vars) => t(key as TranslationKey, vars)}
              />
            </div>
            <div className={modalFormRowClass()}>
              <div className={modelingFormGroupClass}>
                <label htmlFor="add-dim-type-derived">{t('modeling.dimension_type_label')}</label>
                <Select
                  id="add-dim-type-derived"
                  name="type"
                  value={type}
                  onChange={setType}
                  disabled={saving}
                  options={TYPE_OPTIONS}
                />
              </div>
            </div>
          </>
        )}

        <div className={modalActionsBorderedClass()}>
          <button
            className={buttonClass('secondary')}
            type="button"
            onClick={onClose}
            disabled={saving}
          >
            {t('common.cancel')}
          </button>
          <button className={buttonClass('primary')} type="submit" disabled={!canSubmit}>
            {saving ? t('common.saving') : t('common.create')}
          </button>
        </div>
      </form>
    </Modal>
  )
}

AddDimensionModal.displayName = 'AddDimensionModal'
