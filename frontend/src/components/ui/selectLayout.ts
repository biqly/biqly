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
