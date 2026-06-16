import { type FocusEvent, type MouseEvent, useId, useState } from 'react'
import { createPortal } from 'react-dom'

import { cn } from '../../lib/cn'
import {
  tableBrowserCellPopoverClass,
  tableBrowserCellValueClass,
  tableBrowserCellValueMultilineClass,
} from './tableBrowserClasses'

interface TableBrowserCellValueProps {
  value: string
  className?: string
  multiline?: boolean
}

interface PopoverPlacement {
  top: number
  left: number
}

function placementFor(rect: DOMRect): PopoverPlacement {
  const margin = 12
  const width = Math.min(448, window.innerWidth - margin * 2)
  const left = Math.min(Math.max(rect.left, margin), window.innerWidth - width - margin)
  const top = rect.bottom + 8
  return { top, left }
}

export function TableBrowserCellValue({
  value,
  className,
  multiline = false,
}: TableBrowserCellValueProps) {
  const popoverId = useId()
  const [placement, setPlacement] = useState<PopoverPlacement | null>(null)

  const showPopover = (target: HTMLElement) => {
    if (!value) {
      return
    }
    const hasOverflow =
      target.scrollWidth > target.clientWidth + 1 || target.scrollHeight > target.clientHeight + 1
    if (!hasOverflow && value.length < 48) {
      return
    }
    setPlacement(placementFor(target.getBoundingClientRect()))
  }

  const handleMouseEnter = (event: MouseEvent<HTMLSpanElement>) => {
    showPopover(event.currentTarget)
  }

  const handleFocus = (event: FocusEvent<HTMLSpanElement>) => {
    showPopover(event.currentTarget)
  }

  const hidePopover = () => {
    setPlacement(null)
  }

  return (
    <>
      <span
        className={cn(
          tableBrowserCellValueClass,
          multiline && tableBrowserCellValueMultilineClass,
          className,
        )}
        tabIndex={value ? 0 : undefined}
        aria-describedby={placement ? popoverId : undefined}
        onMouseEnter={handleMouseEnter}
        onMouseLeave={hidePopover}
        onFocus={handleFocus}
        onBlur={hidePopover}
      >
        {value}
      </span>
      {placement &&
        createPortal(
          <div
            id={popoverId}
            role="tooltip"
            className={tableBrowserCellPopoverClass}
            style={{ top: placement.top, left: placement.left }}
          >
            {value}
          </div>,
          document.body,
        )}
    </>
  )
}
