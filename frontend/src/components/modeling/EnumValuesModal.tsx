import { useState } from 'react'

import type { useT } from '../../i18n'
import type { EnumMapping, SemanticDimension } from '../../types/semantic'
import { Modal } from '../ui/Modal'

export interface EnumValuesModalProps {
  modelId: string
  dimension: SemanticDimension
  onClose: () => void
  onSaved: () => void | Promise<void>
  putData: <T = unknown>(url: string, body: unknown) => Promise<T | null>
  t: ReturnType<typeof useT>
}

interface EnumRow {
  raw_value: string
  label: string
  description: string
}

function toRows(values: EnumMapping[] | undefined): EnumRow[] {
  if (!values || values.length === 0) {
    return [{ raw_value: '', label: '', description: '' }]
  }
  return values.map((v) => ({
    raw_value: v.raw_value,
    label: v.label,
    description: v.description ?? '',
  }))
}

export function EnumValuesModal({
  modelId,
  dimension,
  onClose,
  onSaved,
  putData,
  t,
}: EnumValuesModalProps) {
  const [rows, setRows] = useState<EnumRow[]>(() => toRows(dimension.enum_values))
  const [saving, setSaving] = useState(false)

  const updateRow = (index: number, patch: Partial<EnumRow>) => {
    setRows((current) => current.map((row, i) => (i === index ? { ...row, ...patch } : row)))
  }

  const addRow = () =>
    setRows((current) => [...current, { raw_value: '', label: '', description: '' }])

  const removeRow = (index: number) => {
    setRows((current) => current.filter((_, i) => i !== index))
  }

  const save = async () => {
    if (saving) {
      return
    }
    const values = rows
      .map((row, i) => ({
        raw_value: row.raw_value.trim(),
        label: row.label.trim(),
        description: row.description.trim(),
        sort_order: i,
      }))
      .filter((row) => row.raw_value !== '' && row.label !== '')
    setSaving(true)
    try {
      await putData(`/api/semantic/models/${modelId}/dimensions/${dimension.id}/enums`, { values })
      await onSaved()
    } finally {
      setSaving(false)
    }
  }

  return (
    <Modal
      open
      onClose={onClose}
      className="modal-card--modeling"
      labelledBy="modeling-enum-title"
      title={t('modeling.enum_values_title')}
      subtitle={dimension.column_ref}
    >
      <form
        onSubmit={(event) => {
          event.preventDefault()
          void save()
        }}
      >
        <p className="text-[0.85rem] text-foreground-faint mb-3">
          {t('modeling.enum_values_help')}
        </p>
        <div className="flex flex-col gap-2 mb-3">
          {rows.map((row, index) => (
            <div
              className="grid grid-cols-[1fr_1fr_1.5fr_auto] gap-2 items-center [&_input]:w-full"
              key={index}
            >
              <input
                aria-label={t('modeling.enum_raw_value')}
                placeholder={t('modeling.enum_raw_value')}
                value={row.raw_value}
                onChange={(event) => updateRow(index, { raw_value: event.target.value })}
                disabled={saving}
              />
              <input
                aria-label={t('modeling.enum_label')}
                placeholder={t('modeling.enum_label')}
                value={row.label}
                onChange={(event) => updateRow(index, { label: event.target.value })}
                disabled={saving}
              />
              <input
                aria-label={t('modeling.enum_description')}
                placeholder={t('modeling.enum_description')}
                value={row.description}
                onChange={(event) => updateRow(index, { description: event.target.value })}
                disabled={saving}
              />
              <button
                type="button"
                className="modeling-delete-btn"
                onClick={() => removeRow(index)}
                title={t('common.delete')}
                disabled={saving}
              >
                ×
              </button>
            </div>
          ))}
        </div>
        <button
          type="button"
          className="btn btn-secondary btn-sm"
          onClick={addRow}
          disabled={saving}
        >
          {t('modeling.enum_add_value')}
        </button>
        <div className="modal-actions">
          <button className="btn btn-secondary" type="button" onClick={onClose} disabled={saving}>
            {t('common.cancel')}
          </button>
          <button className="btn btn-primary" type="submit" disabled={saving}>
            {saving ? t('common.saving') : t('common.save')}
          </button>
        </div>
      </form>
    </Modal>
  )
}
