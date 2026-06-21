import { useState } from 'react'

import type { useT } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
import { modelingFormGroupClass } from '../../lib/formClasses'
import { modalActionsBorderedClass, modalModelingCardClass } from '../../lib/modalClasses'
import { modelingEmptyClass } from '../../lib/modelingClasses'
import type { TableRow } from '../../types/semantic'
import { Modal } from '../ui/Modal'
import { Select } from '../ui/Select'
export interface BaseSwapModalProps {
  candidateTables: TableRow[]
  onCancel: () => void
  onSubmit: (schema: string, table: string) => Promise<void>
  saving: boolean
  t: ReturnType<typeof useT>
}

export function BaseSwapModal({
  candidateTables,
  onCancel,
  onSubmit,
  saving,
  t,
}: BaseSwapModalProps) {
  const options = candidateTables
  const [picked, setPicked] = useState<string>(() =>
    options[0] ? `${options[0].schema_name}.${options[0].table_name}` : '',
  )

  return (
    <Modal
      open
      onClose={saving ? () => undefined : onCancel}
      closeOnBackdrop={!saving}
      className={modalModelingCardClass()}
      labelledBy="modeling-base-swap-title"
      title={t('modeling.change_base_title')}
      subtitle={t('modeling.pick_new_base')}
    >
      {options.length === 0 ? (
        <p className={modelingEmptyClass}>{t('modeling.no_alternative_base')}</p>
      ) : (
        <div className={modelingFormGroupClass}>
          <Select
            value={picked}
            onChange={setPicked}
            disabled={saving}
            options={options.map((tbl) => {
              const key = `${tbl.schema_name}.${tbl.table_name}`
              return {
                value: key,
                label: `${tbl.label ?? tbl.table_name} — ${tbl.schema_name}.${tbl.table_name}`,
              }
            })}
          />
        </div>
      )}
      <div className={modalActionsBorderedClass()}>
        <button
          className={buttonClass('secondary')}
          type="button"
          onClick={onCancel}
          disabled={saving}
        >
          {t('common.cancel')}
        </button>
        <button
          className={buttonClass('danger')}
          type="button"
          disabled={saving || !picked}
          onClick={() => {
            const parts = picked.split('.')
            const schema = parts[0] ?? ''
            const table = parts.slice(1).join('.')
            if (!schema || !table) {
              return
            }
            void onSubmit(schema, table)
          }}
        >
          {saving ? t('common.saving') : t('modeling.remove_table_action')}
        </button>
      </div>
    </Modal>
  )
}
