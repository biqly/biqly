import { useAutofocus } from '../../hooks/useAutofocus'
import type { TranslationKey, useT } from '../../i18n'
import type { ColumnRow, SemanticMetric, SemanticModelDetail, TableRow } from '../../types/semantic'
import { Modal } from '../ui/Modal'
import { AddMetricSimpleFields } from './AddMetricSimpleFields'
import { ExpressionBuilder } from './ExpressionBuilder'
import { useAddMetricModalState } from './useAddMetricModalState'

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
  const state = useAddMetricModalState(model, includedTables, columns, metric)
  const metricNameInputRef = useAutofocus<HTMLInputElement>(!metric)
  const metricLabelInputRef = useAutofocus<HTMLInputElement>(!!metric)

  const submit = async () => {
    const body = state.buildSubmitBody()
    if (!body) {
      return
    }
    state.setSaving(true)
    try {
      if (metric && putData) {
        await putData(`/api/semantic/models/${model.id}/metrics/${metric.id}`, body)
      } else {
        await postData(`/api/semantic/models/${model.id}/metrics`, body)
      }
      await onCreated()
    } finally {
      state.setSaving(false)
    }
  }

  return (
    <Modal
      open
      onClose={onClose}
      closeOnBackdrop={!state.saving}
      className={state.mode === 'custom' ? 'modal-card--metric' : 'modal-card--modeling'}
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
              ref={metricNameInputRef}
              value={state.name}
              onChange={(e) => state.setName(e.target.value)}
              disabled={state.saving || !!metric}
              autoComplete="off"
            />
          </div>
          <div className="form-group">
            <label htmlFor="metric-label">{t('modeling.metric_label_label')}</label>
            <input
              id="metric-label"
              ref={metricLabelInputRef}
              value={state.label}
              onChange={(e) => state.setLabel(e.target.value)}
              disabled={state.saving}
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
            className={`toggle-btn ${state.mode === 'simple' ? 'active' : ''}`}
            onClick={() => state.handleModeChange('simple')}
            disabled={state.saving}
            role="tab"
            aria-selected={state.mode === 'simple'}
          >
            {t('modeling.simple_metric')}
          </button>
          <button
            type="button"
            className={`toggle-btn ${state.mode === 'custom' ? 'active' : ''}`}
            onClick={() => state.handleModeChange('custom')}
            disabled={state.saving}
            role="tab"
            aria-selected={state.mode === 'custom'}
          >
            {t('modeling.custom_expression')}
          </button>
        </div>

        {state.mode === 'simple' ? (
          <AddMetricSimpleFields
            t={t}
            saving={state.saving}
            availableSchemas={state.availableSchemas}
            availableTables={state.availableTables}
            availableColumns={state.availableColumns}
            selectedSchema={state.selectedSchema}
            selectedTable={state.selectedTable}
            selectedColumn={state.selectedColumn}
            selectedAggregation={state.selectedAggregation}
            onSchemaChange={state.setSelectedSchema}
            onTableChange={state.setSelectedTable}
            onColumnChange={state.setSelectedColumn}
            onAggregationChange={state.setSelectedAggregation}
          />
        ) : (
          <div className="form-group" style={{ display: 'block', width: '100%' }}>
            <label htmlFor="metric-expression">{t('modeling.metric_expression_label')}</label>
            <ExpressionBuilder
              model={model}
              columns={columns}
              initialNode={state.astNode}
              initialText={state.expression}
              onChange={(node, textExpr) => {
                state.setExpression(textExpr)
                state.setAstNode(node)
              }}
              t={(key, vars) => t(key as TranslationKey, vars)}
            />
          </div>
        )}

        <div className="form-group">
          <label htmlFor="metric-format">{t('modeling.metric_format_label')}</label>
          <input
            id="metric-format"
            value={state.format}
            onChange={(e) => state.setFormat(e.target.value)}
            disabled={state.saving}
            placeholder="$#,##0.00"
            autoComplete="off"
          />
        </div>
        <div className="modal-actions">
          <button
            className="btn btn-secondary"
            type="button"
            onClick={onClose}
            disabled={state.saving}
          >
            {t('common.cancel')}
          </button>
          <button className="btn btn-primary" type="submit" disabled={!state.canSubmit}>
            {state.saving ? t('common.saving') : metric ? t('common.save') : t('common.create')}
          </button>
        </div>
      </form>
    </Modal>
  )
}
