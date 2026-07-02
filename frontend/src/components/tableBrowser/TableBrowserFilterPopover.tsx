import { useEffect, useReducer, useRef } from 'react'

import type { useT } from '../../i18n'
import { buttonClass } from '../../lib/buttonClasses'
import { cn } from '../../lib/cn'
import { Select } from '../ui/Select'
import {
  chipInputContainerClass,
  chipInputFieldClass,
  chipTagClass,
  chipTagCloseClass,
  filterPopoverAnchoredClass,
  filterPopoverBtnClass,
  filterPopoverCheckboxInputClass,
  filterPopoverCheckboxLabelClass,
  filterPopoverCheckboxRowClass,
  filterPopoverClass,
  filterPopoverCloseClass,
  filterPopoverFooterClass,
  filterPopoverHeaderClass,
  filterPopoverHintClass,
  filterPopoverRowClass,
  filterPopoverRowLabelClass,
  filterPopoverTitleClass,
} from './tableBrowserClasses'

const POPOVER_WIDTH_PX = 320

function getPopoverPosition(anchorEl: HTMLElement) {
  const rect = anchorEl.getBoundingClientRect()
  let left = rect.left
  left = Math.min(left, window.innerWidth - POPOVER_WIDTH_PX - 8)
  left = Math.max(8, left)
  return { top: rect.bottom + 5, left }
}

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
  anchorEl,
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
  anchorEl?: HTMLElement | null
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
  const valueInputRef = useRef<HTMLInputElement>(null)
  const [, rerenderAnchor] = useReducer((count: number) => count + 1, 0)
  const position = anchorEl ? getPopoverPosition(anchorEl) : null
  const isAnchored = Boolean(anchorEl && position)

  useEffect(() => {
    valueInputRef.current?.focus()
  }, [])

  useEffect(() => {
    if (!anchorEl) {
      return
    }
    const update = () => {
      rerenderAnchor()
    }
    window.addEventListener('scroll', update, true)
    window.addEventListener('resize', update)
    return () => {
      window.removeEventListener('scroll', update, true)
      window.removeEventListener('resize', update)
    }
  }, [anchorEl])

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
    <div
      ref={rootRef}
      role="dialog"
      aria-label={
        editingFilterId
          ? t('table_browser.popover_title_edit')
          : t('table_browser.popover_title_add')
      }
      className={cn(isAnchored ? filterPopoverAnchoredClass : filterPopoverClass)}
      style={
        isAnchored && position
          ? { top: position.top, left: position.left, width: '20rem' }
          : { width: '20rem' }
      }
    >
      <div className={filterPopoverHeaderClass}>
        <span className={filterPopoverTitleClass}>
          {editingFilterId
            ? t('table_browser.popover_title_edit')
            : t('table_browser.popover_title_add')}
        </span>
        <button
          type="button"
          className={filterPopoverCloseClass}
          onClick={onClose}
          aria-label={t('common.close')}
        >
          ×
        </button>
      </div>

      <div className={filterPopoverRowClass}>
        <label className={filterPopoverRowLabelClass} htmlFor="filter-popover-column">
          {t('table_browser.column')}
        </label>
        <Select
          id="filter-popover-column"
          value={popoverField}
          onChange={onFieldChange}
          options={filterFieldOpts}
          size="sm"
        />
      </div>

      <div className={filterPopoverRowClass}>
        <label className={filterPopoverRowLabelClass} htmlFor="filter-popover-operator">
          {t('table_browser.operator')}
        </label>
        <Select
          id="filter-popover-operator"
          value={popoverOperator}
          onChange={onOperatorChange}
          options={operatorOptions}
          size="sm"
        />
      </div>

      <div className={filterPopoverRowClass}>
        <label className={filterPopoverRowLabelClass} htmlFor="chip-input-el">
          {t('table_browser.value')}
        </label>
        <div className={chipInputContainerClass} onClick={() => valueInputRef.current?.focus()}>
          {popoverChips.map((chip, idx) => (
            <span key={idx} className={chipTagClass}>
              {chip}
              <button
                type="button"
                className={chipTagCloseClass}
                aria-label={t('table_browser.remove_filter')}
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
            ref={valueInputRef}
            type="text"
            value={chipInputText}
            onChange={(e) => onChipInputChange(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Enter' || e.key === ',') {
                e.preventDefault()
                if (e.key === 'Enter' && !chipInputText.trim() && popoverChips.length > 0) {
                  onSave()
                  return
                }
                onAddChip(chipInputText)
              } else if (e.key === 'Backspace' && !chipInputText && popoverChips.length > 0) {
                onRemoveChip(popoverChips.length - 1)
              }
            }}
            placeholder={popoverChips.length === 0 ? t('table_browser.enter_value') : ''}
            className={chipInputFieldClass}
          />
        </div>
        <span className={filterPopoverHintClass}>{t('table_browser.value_hint')}</span>
      </div>

      <div className={filterPopoverFooterClass}>
        <div className={filterPopoverCheckboxRowClass}>
          <input
            type="checkbox"
            id="case-sensitive-cb"
            checked={popoverCaseSensitive}
            onChange={(e) => onCaseSensitiveChange(e.target.checked)}
            className={filterPopoverCheckboxInputClass}
          />
          <label htmlFor="case-sensitive-cb" className={filterPopoverCheckboxLabelClass}>
            {t('table_browser.case_sensitive')}
          </label>
        </div>
        <div className="flex items-center gap-2">
          <button
            type="button"
            className={buttonClass('ghost', { size: 'sm', autoWidth: true })}
            onClick={onClose}
          >
            {t('common.cancel')}
          </button>
          <button type="button" className={filterPopoverBtnClass} onClick={onSave}>
            {editingFilterId ? t('table_browser.update_filter') : t('table_browser.add_filter')}
          </button>
        </div>
      </div>
    </div>
  )
}
