import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'

import type { SelectOption } from './Select'
import { resolveSelectPopoverLayout } from './selectLayout'

interface PopoverPos {
  left: number
  top: number
  width: number
  maxHeight: number
  placement: 'down' | 'up'
}

export function useSelectDropdown<T extends string>({
  value,
  onChange,
  options,
  searchable,
  size,
}: {
  value: T
  onChange: (value: T) => void
  options: SelectOption<T>[]
  searchable: boolean
  size: 'sm' | 'md'
}) {
  const [open, setOpen] = useState(false)
  const [activeIndex, setActiveIndex] = useState(-1)
  const [popover, setPopover] = useState<PopoverPos | null>(null)
  const [search, setSearch] = useState('')

  const rootRef = useRef<HTMLDivElement>(null)
  const triggerRef = useRef<HTMLButtonElement>(null)
  const listRef = useRef<HTMLUListElement>(null)
  const searchRef = useRef<HTMLInputElement>(null)

  const displayOptions = useMemo(() => {
    if (!searchable) {
      return options
    }
    const q = search.trim().toLowerCase()
    if (!q) {
      return options
    }
    return options.filter((o) => {
      const hay = `${o.label} ${o.hint ?? ''} ${o.value}`.toLowerCase()
      return hay.includes(q)
    })
  }, [options, search, searchable])

  const selectedIndex = useMemo(() => options.findIndex((o) => o.value === value), [options, value])
  const selected = selectedIndex >= 0 ? options[selectedIndex] : null

  const closeAndFocus = useCallback(() => {
    setOpen(false)
    triggerRef.current?.focus()
  }, [])

  const pickByIndex = useCallback(
    (idx: number) => {
      const opt = displayOptions[idx]
      if (!opt || opt.disabled) {
        return
      }
      onChange(opt.value)
      closeAndFocus()
    },
    [displayOptions, onChange, closeAndFocus],
  )

  const findNextEnabled = useCallback(
    (start: number, direction: 1 | -1): number => {
      if (displayOptions.length === 0) {
        return -1
      }
      let i = start
      for (const _ of displayOptions) {
        i = (i + direction + displayOptions.length) % displayOptions.length
        const opt = displayOptions[i]
        if (opt && !opt.disabled) {
          return i
        }
      }
      return -1
    },
    [displayOptions],
  )

  const updatePosition = useCallback(() => {
    if (!triggerRef.current) {
      return
    }
    const rect = triggerRef.current.getBoundingClientRect()
    const viewportH = window.innerHeight
    const spaceBelow = viewportH - rect.bottom - 12
    const spaceAbove = rect.top - 12
    const desired = 288
    const placement: 'down' | 'up' = spaceBelow < 220 && spaceAbove > spaceBelow ? 'up' : 'down'
    const maxHeight = Math.max(
      160,
      Math.min(desired, placement === 'down' ? spaceBelow : spaceAbove),
    )
    const top = placement === 'down' ? rect.bottom + 6 : Math.max(8, rect.top - 6 - maxHeight)
    const { left, width } = resolveSelectPopoverLayout(rect, options, size === 'sm' ? 11.5 : 12.5)
    setPopover({ left, top, width, maxHeight, placement })
  }, [options, size])

  useLayoutEffect(() => {
    if (!open) {
      return
    }
    updatePosition()
  }, [open, updatePosition])

  useEffect(() => {
    if (!open) {
      return
    }
    const handle = () => updatePosition()
    window.addEventListener('resize', handle)
    window.addEventListener('scroll', handle, true)
    return () => {
      window.removeEventListener('resize', handle)
      window.removeEventListener('scroll', handle, true)
    }
  }, [open, updatePosition])

  useEffect(() => {
    if (!open) {
      return
    }
    const onDown = (e: MouseEvent) => {
      const target = e.target as Node
      const inRoot = rootRef.current?.contains(target)
      const inList = listRef.current?.contains(target)
      if (!inRoot && !inList) {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', onDown)
    return () => document.removeEventListener('mousedown', onDown)
  }, [open])

  useEffect(() => {
    if (!open) {
      // eslint-disable-next-line react-hooks/set-state-in-effect
      setSearch('')
      setActiveIndex(-1)
      return
    }
    const first = findNextEnabled(-1, 1)
    const curIdx = displayOptions.findIndex((o) => o.value === value)
    const cur = curIdx >= 0 ? displayOptions[curIdx] : undefined
    setActiveIndex(cur && !cur.disabled ? curIdx : first)
    if (searchable) {
      requestAnimationFrame(() => searchRef.current?.focus())
    }
  }, [open, displayOptions, value, findNextEnabled, searchable])

  useEffect(() => {
    if (!open || !searchable) {
      return
    }
    const first = findNextEnabled(-1, 1)
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setActiveIndex(first)
  }, [search, open, searchable, findNextEnabled])

  useEffect(() => {
    if (!open || activeIndex < 0) {
      return
    }
    const node = listRef.current?.querySelector<HTMLElement>(`[data-index="${activeIndex}"]`)
    node?.scrollIntoView({ block: 'nearest' })
  }, [open, activeIndex])

  return {
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
  }
}
