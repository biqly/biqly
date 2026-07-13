import { useAutofocus } from '../../hooks/useAutofocus'
import type { useT } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
import { modelingFormGroupClass } from '../../lib/formClasses'
import { modalActionsBorderedClass, modalModelingCardClass } from '../../lib/modalClasses'
import type {
  ColumnRow,
  SemanticDimension,
  SemanticMetric,
  SemanticModelDetail,
  TableRow,
} from '../../types/semantic'
import { Modal } from '../ui/Modal'
import { AddDimensionModal } from './AddDimensionModal'
import { AddMetricModal } from './AddMetricModal'
import { BaseSwapModal } from './BaseSwapModal'
import { EditDimensionModal } from './EditDimensionModal'
import { EnumValuesModal } from './EnumValuesModal'

export function ModelingModals({
  t,
  model,
  includedTables,
  columns,
  renameTarget,
  renameValue,
  savingRename,
  onRenameValueChange,
  onCloseRename,
  onSubmitRename,
  baseSwapOpen,
  baseSwapCandidates,
  savingBaseSwap,
  onCloseBaseSwap,
  onBaseSwapSubmit,
  addMetricOpen,
  editingMetric,
  onCloseMetricModal,
  onMetricCreated,
  postData,
  putData,
  addDimensionOpen,
  onCloseAddDimension,
  onDimensionCreated,
  editingDimension,
  onCloseEditDimension,
  onDimensionSaved,
  enumDimension,
  onCloseEnumDimension,
  onEnumSaved,
}: {
  t: ReturnType<typeof useT>
  model: SemanticModelDetail
  includedTables: TableRow[]
  columns: ColumnRow[]
  renameTarget: { title: string; subtitle?: string; current: string } | null
  renameValue: string
  savingRename: boolean
  onRenameValueChange: (v: string) => void
  onCloseRename: () => void
  onSubmitRename: () => void
  baseSwapOpen: boolean
  baseSwapCandidates: TableRow[]
  savingBaseSwap: boolean
  onCloseBaseSwap: () => void
  onBaseSwapSubmit: (schema: string, table: string) => Promise<void>
  addMetricOpen: boolean
  editingMetric: SemanticMetric | null
  onCloseMetricModal: () => void
  onMetricCreated: (isEdit: boolean) => Promise<void>
  postData: <T = unknown>(url: string, body: unknown) => Promise<T | null>
  putData: <T = unknown>(url: string, body: unknown) => Promise<T | null>
  addDimensionOpen: boolean
  onCloseAddDimension: () => void
  onDimensionCreated: () => Promise<void>
  editingDimension: SemanticDimension | null
  onCloseEditDimension: () => void
  onDimensionSaved: () => Promise<void>
  enumDimension: SemanticDimension | null
  onCloseEnumDimension: () => void
  onEnumSaved: () => Promise<void>
}) {
  const renameInputRef = useAutofocus<HTMLInputElement>(Boolean(renameTarget))

  return (
    <>
      {renameTarget && (
        <Modal
          open
          onClose={onCloseRename}
          className={modalModelingCardClass()}
          labelledBy="modeling-rename-title"
          title={renameTarget.title}
          subtitle={renameTarget.subtitle}
        >
          <form
            onSubmit={(event) => {
              event.preventDefault()
              onSubmitRename()
            }}
          >
            <div className={modelingFormGroupClass}>
              <label htmlFor="modeling-rename-value">{t('modeling.display_name_label')}</label>
              <input
                id="modeling-rename-value"
                ref={renameInputRef}
                value={renameValue}
                onChange={(event) => onRenameValueChange(event.target.value)}
                placeholder={renameTarget.current}
                disabled={savingRename}
              />
            </div>
            <div className={modalActionsBorderedClass()}>
              <button
                className={buttonClass('secondary')}
                type="button"
                onClick={onCloseRename}
                disabled={savingRename}
              >
                {t('common.cancel')}
              </button>
              <button
                className={buttonClass('primary')}
                type="submit"
                disabled={savingRename || !renameValue.trim()}
              >
                {savingRename ? t('common.saving') : t('common.update')}
              </button>
            </div>
          </form>
        </Modal>
      )}
      {baseSwapOpen && (
        <BaseSwapModal
          candidateTables={baseSwapCandidates}
          onCancel={onCloseBaseSwap}
          onSubmit={onBaseSwapSubmit}
          saving={savingBaseSwap}
          t={t}
        />
      )}
      {(addMetricOpen || editingMetric) && (
        <AddMetricModal
          model={model}
          includedTables={includedTables}
          columns={columns}
          metric={editingMetric ?? undefined}
          onClose={onCloseMetricModal}
          onCreated={() => onMetricCreated(!!editingMetric)}
          postData={postData}
          putData={putData}
          t={t}
        />
      )}
      {addDimensionOpen && (
        <AddDimensionModal
          model={model}
          includedTables={includedTables}
          columns={columns}
          onClose={onCloseAddDimension}
          onCreated={onDimensionCreated}
          postData={postData}
          t={t}
        />
      )}
      {editingDimension && (
        <EditDimensionModal
          model={model}
          includedTables={includedTables}
          columns={columns}
          dimension={editingDimension}
          onClose={onCloseEditDimension}
          onSaved={onDimensionSaved}
          putData={putData}
          t={t}
        />
      )}
      {enumDimension && (
        <EnumValuesModal
          modelId={model.id}
          dimension={enumDimension}
          onClose={onCloseEnumDimension}
          onSaved={onEnumSaved}
          putData={putData}
          t={t}
        />
      )}
    </>
  )
}
