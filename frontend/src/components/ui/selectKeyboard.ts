import type { KeyboardEvent } from 'react'

import type { SelectOption } from './Select'

export interface SelectKeyboardContext {
  disabled: boolean
  open: boolean
  setOpen: (open: boolean | ((prev: boolean) => boolean)) => void
  closeAndFocus: () => void
  activeIndex: number
  setActiveIndex: (index: number | ((prev: number) => number)) => void
  displayOptions: SelectOption[]
  findNextEnabled: (start: number, direction: 1 | -1) => number
  pickByIndex: (idx: number) => void
  searchRef: React.RefObject<HTMLInputElement | null>
}

export function handleSelectTriggerKeyDown(e: KeyboardEvent, ctx: SelectKeyboardContext): void {
  if (ctx.disabled) {
    return
  }
  if (!ctx.open) {
    if (e.key === 'Enter' || e.key === ' ' || e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault()
      ctx.setOpen(true)
    }
    return
  }
  if (e.key === 'Escape' || e.key === 'Tab') {
    e.preventDefault()
    ctx.closeAndFocus()
    return
  }
  if (e.key === 'ArrowDown') {
    e.preventDefault()
    ctx.setActiveIndex((i) => ctx.findNextEnabled(i < 0 ? -1 : i, 1))
    ctx.searchRef.current?.blur()
    return
  }
  if (e.key === 'ArrowUp') {
    e.preventDefault()
    ctx.setActiveIndex((i) => ctx.findNextEnabled(i < 0 ? ctx.displayOptions.length : i, -1))
    ctx.searchRef.current?.blur()
    return
  }
  if (e.key === 'Home') {
    e.preventDefault()
    ctx.setActiveIndex(ctx.findNextEnabled(-1, 1))
    return
  }
  if (e.key === 'End') {
    e.preventDefault()
    ctx.setActiveIndex(ctx.findNextEnabled(ctx.displayOptions.length, -1))
    return
  }
  if (e.key === 'Enter' || e.key === ' ') {
    e.preventDefault()
    if (ctx.activeIndex >= 0) {
      ctx.pickByIndex(ctx.activeIndex)
    }
  }
}
