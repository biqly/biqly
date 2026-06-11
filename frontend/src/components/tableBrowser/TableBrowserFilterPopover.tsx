import { useEffect, useRef } from 'react'

import type { useT } from '../../i18n'
import { Select } from '../ui/Select'

export function TableBrowserFilterPopover({
  t,
  popoverField,
  popoverOperator,
  popoverChips,
  chipInputText,
  popoverCaseSensitive,
  editingFilterId,
  operatorOptions,
  filterFieldOpts,
  getDimensionLabel,
  onClose,
  onOperatorChange,
  onFieldChange,
  onChipInputChange,
  onAddChip,
  onRemoveChip,
  onCaseSensitiveChange,
  onSave,
}: {
  t: ReturnType<typeof useT>
  popoverField: string
  popoverOperator: string
  popoverChips: string[]
  chipInputText: string
  popoverCaseSensitive: boolean
  editingFilterId: string | null
  operatorOptions: { value: string; label: string }[]
  filterFieldOpts: { value: string; label: string }[]
  getDimensionLabel: (name: string) => string
  onClose: () => void
  onOperatorChange: (op: string) => void
  onFieldChange: (field: string) => void
  onChipInputChange: (text: string) => void
  onAddChip: (text: string) => void
  onRemoveChip: (index: number) => void
  onCaseSensitiveChange: (checked: boolean) => void
  onSave: () => void
}) {
  const rootRef = useRef<HTMLDivElement>(null)

  // Dismiss on outside click or Escape; the popover otherwise traps the page.
  useEffect(() => {
    const onPointerDown = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) {
        onClose()
      }
    }
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose()
      }
    }
    document.addEventListener('mousedown', onPointerDown)
    document.addEventListener('keydown', onKeyDown)
    return () => {
      document.removeEventListener('mousedown', onPointerDown)
      document.removeEventListener('keydown', onKeyDown)
    }
  }, [onClose])

  return (
    <div ref={rootRef} className="filter-popover" style={{ width: '18rem' }}>
      <div
        className="filter-popover-header"
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          borderBottom: 'none',
          paddingBottom: '0.2rem',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
          <button type="button" className="filter-popover-back" onClick={onClose}>
            ‹
          </button>
          <span style={{ fontSize: '0.86rem', fontWeight: 700 }}>
            {getDimensionLabel(popoverField)}
          </span>
        </div>
        <Select
          value={popoverOperator}
          onChange={onOperatorChange}
          options={operatorOptions}
          size="sm"
        />
      </div>

      {!editingFilterId && (
        <div className="filter-popover-row" style={{ marginTop: '0.1rem' }}>
          <label>{t('table_browser.column')}</label>
          <Select
            value={popoverField}
            onChange={onFieldChange}
            options={filterFieldOpts}
            size="sm"
          />
        </div>
      )}

      <div className="filter-popover-row">
        <label>{t('table_browser.value')}</label>
        <div
          className="chip-input-container"
          onClick={() => document.getElementById('chip-input-el')?.focus()}
        >
          {popoverChips.map((chip, idx) => (
            <span key={idx} className="chip-tag">
              {chip}
              <button
                type="button"
                className="chip-tag-close"
                onClick={(e) => {
                  e.stopPropagation()
                  onRemoveChip(idx)
                }}
              >
                ×
              </button>
            </span>
          ))}
          <input
            id="chip-input-el"
            type="text"
            value={chipInputText}
            onChange={(e) => onChipInputChange(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' || e.key === ',') {
                e.preventDefault()
                onAddChip(chipInputText)
              } else if (e.key === 'Backspace' && !chipInputText && popoverChips.length > 0) {
                onRemoveChip(popoverChips.length - 1)
              }
            }}
            placeholder={popoverChips.length === 0 ? t('table_browser.enter_value') : ''}
            className="chip-input-field"
          />
        </div>
      </div>

      <div
        style={{
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          marginTop: '0.2rem',
        }}
      >
        <div className="filter-popover-checkbox-row">
          <input
            type="checkbox"
            id="case-sensitive-cb"
            checked={popoverCaseSensitive}
            onChange={(e) => onCaseSensitiveChange(e.target.checked)}
          />
          <label htmlFor="case-sensitive-cb">{t('table_browser.case_sensitive')}</label>
        </div>
        <button
          type="button"
          className="filter-popover-btn"
          style={{ width: 'auto', padding: '0.35rem 0.85rem' }}
          onClick={onSave}
        >
          {editingFilterId ? t('table_browser.update_filter') : t('table_browser.add_filter')}
        </button>
      </div>
    </div>
  )
}
