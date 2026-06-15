import type { SelectOption } from './Select'

const CHECK_AND_PADDING_PX = 56
const MIN_POPOVER_WIDTH_PX = 160
const MAX_POPOVER_WIDTH_PX = 480

function optionText(opt: SelectOption): string {
  return opt.hint ? `${opt.label} ${opt.hint}` : opt.label
}

export function measureSelectOptionsWidth(options: SelectOption[], fontSizePx: number): number {
  if (options.length === 0) {
    return MIN_POPOVER_WIDTH_PX
  }

  if (typeof document !== 'undefined') {
    const canvas = document.createElement('canvas')
    const ctx = canvas.getContext('2d')
    if (ctx) {
      ctx.font = `${fontSizePx}px ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", sans-serif`
      let max = 0
      for (const opt of options) {
        let w = ctx.measureText(opt.label).width
        if (opt.hint) {
          w += ctx.measureText(` ${opt.hint}`).width
        }
        if (typeof opt.count === 'number') {
          w += 36
        }
        max = Math.max(max, w)
      }
      return Math.ceil(max) + CHECK_AND_PADDING_PX
    }
  }

  const longest = options.reduce((m, o) => Math.max(m, optionText(o).length), 0)
  return longest * (fontSizePx * 0.62) + CHECK_AND_PADDING_PX
}

export function resolveSelectPopoverLayout(
  triggerRect: DOMRect,
  options: SelectOption[],
  fontSizePx: number,
): { left: number; width: number } {
  const viewportW = typeof window !== 'undefined' ? window.innerWidth : 1200
  const maxW = Math.min(MAX_POPOVER_WIDTH_PX, viewportW - 16)
  const contentW = measureSelectOptionsWidth(options, fontSizePx)
  const width = Math.min(maxW, Math.max(triggerRect.width, contentW, MIN_POPOVER_WIDTH_PX))
  let left = triggerRect.left
  if (left + width > viewportW - 8) {
    left = Math.max(8, viewportW - 8 - width)
  }
  return { left, width }
}

export type SelectPopoverPlacement = 'down' | 'up'

export interface SelectPopoverCoords {
  left: number
  top?: number
  bottom?: number
  width: number
  maxHeight: number
  placement: SelectPopoverPlacement
}

const VIEWPORT_MARGIN_PX = 12
const POPOVER_GAP_PX = 6
const FLIP_THRESHOLD_PX = 220
const DEFAULT_DESIRED_HEIGHT_PX = 288
const MIN_POPOVER_HEIGHT_PX = 160

export function resolveSelectPopoverCoords(
  triggerRect: DOMRect,
  options: SelectOption[],
  fontSizePx: number,
  desiredHeight: number = DEFAULT_DESIRED_HEIGHT_PX,
  viewportHeight: number = typeof window !== 'undefined' ? window.innerHeight : 800,
): SelectPopoverCoords {
  const { left, width } = resolveSelectPopoverLayout(triggerRect, options, fontSizePx)
  const spaceBelow = viewportHeight - triggerRect.bottom - VIEWPORT_MARGIN_PX
  const spaceAbove = triggerRect.top - VIEWPORT_MARGIN_PX
  const placement: SelectPopoverPlacement =
    spaceBelow < FLIP_THRESHOLD_PX && spaceAbove > spaceBelow ? 'up' : 'down'
  const maxHeight = Math.max(
    MIN_POPOVER_HEIGHT_PX,
    Math.min(desiredHeight, placement === 'down' ? spaceBelow : spaceAbove),
  )

  if (placement === 'down') {
    return {
      left,
      width,
      maxHeight,
      placement,
      top: triggerRect.bottom + POPOVER_GAP_PX,
    }
  }

  return {
    left,
    width,
    maxHeight,
    placement,
    bottom: viewportHeight - triggerRect.top + POPOVER_GAP_PX,
  }
}

export function selectPopoverFixedStyle(
  coords: Pick<SelectPopoverCoords, 'left' | 'top' | 'bottom' | 'width' | 'placement'>,
): {
  position: 'fixed'
  left: number
  width: number
  top?: number
  bottom?: number
} {
  const base = {
    position: 'fixed' as const,
    left: coords.left,
    width: coords.width,
  }
  if (coords.placement === 'up' && coords.bottom != null) {
    return { ...base, bottom: coords.bottom }
  }
  return { ...base, top: coords.top ?? 0 }
}
