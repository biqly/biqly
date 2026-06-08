import { useId, useMemo } from 'react'

import { useT } from '../../i18n'
import { handleSelectTriggerKeyDown } from './selectKeyboard'
import { SelectPopover } from './SelectPopover'
import { SelectTrigger } from './SelectTrigger'
import { useSelectDropdown } from './useSelectDropdown'

export interface SelectOption<T extends string = string> {
  value: T
  label: string
  hint?: string
  count?: number
  disabled?: boolean
}

interface SelectProps<T extends string = string> {
  value: T
  onChange: (value: T) => void
  options: SelectOption<T>[]
  placeholder?: string
  header?: string
  disabled?: boolean
  id?: string
  name?: string
  ariaLabel?: string
  className?: string
  size?: 'sm' | 'md'
  showHintInTrigger?: boolean
  searchable?: boolean
}

export function Select<T extends string = string>({
  value,
  onChange,
  options,
  placeholder,
  header,
  disabled = false,
  id,
  name,
  ariaLabel,
  className,
  size = 'md',
  showHintInTrigger = false,
  searchable = false,
}: SelectProps<T>) {
  const t = useT()
  const reactId = useId()
  const baseId = id ?? `sel-${reactId}`

  const {
    open,
    setOpen,
    activeIndex,
    setActiveIndex,
    popover,
    search,
    setSearch,
    rootRef,
    triggerRef,
    listRef,
    searchRef,
    displayOptions,
    selected,
    closeAndFocus,
    pickByIndex,
    findNextEnabled,
  } = useSelectDropdown({ value, onChange, options, searchable, size })

  const keyboardCtx = useMemo(
    () => ({
      disabled,
      open,
      setOpen,
      closeAndFocus,
      activeIndex,
      setActiveIndex,
      displayOptions,
      findNextEnabled,
      pickByIndex,
      searchRef,
    }),
    [
      disabled,
      open,
      setOpen,
      closeAndFocus,
      activeIndex,
      setActiveIndex,
      displayOptions,
      findNextEnabled,
      pickByIndex,
      searchRef,
    ],
  )

  const onTriggerKeyDown = (e: React.KeyboardEvent) => {
    handleSelectTriggerKeyDown(e, keyboardCtx)
  }

  const selectedOption = selected ?? null
  const triggerLabel = selectedOption
    ? selectedOption.label
    : (placeholder ?? t('common.select_placeholder'))

  return (
    <div ref={rootRef} className={['ui-select', className].filter(Boolean).join(' ')}>
      <SelectTrigger
        baseId={baseId}
        name={name}
        ariaLabel={ariaLabel}
        open={open}
        disabled={disabled}
        selected={selectedOption}
        placeholder={triggerLabel}
        showHintInTrigger={showHintInTrigger}
        size={size}
        triggerRef={triggerRef}
        onToggle={() => {
          if (!disabled) {
            setOpen((o) => !o)
          }
        }}
        onKeyDown={onTriggerKeyDown}
      />
      {open && popover && (
        <SelectPopover
          baseId={baseId}
          popover={popover}
          header={header}
          searchable={searchable}
          search={search}
          setSearch={setSearch}
          searchRef={searchRef}
          displayOptions={displayOptions}
          value={value}
          activeIndex={activeIndex}
          setActiveIndex={setActiveIndex}
          pickByIndex={pickByIndex}
          keyboardCtx={keyboardCtx}
          listRef={listRef}
          searchPlaceholder={`${t('common.search')}…`}
          emptyLabel={t('common.no_options')}
        />
      )}
    </div>
  )
}
