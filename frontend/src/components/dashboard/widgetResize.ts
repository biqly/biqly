// Pure helpers for the dashboard builder's drag-to-resize handles: widths
// snap to the 12-column grid, heights are free pixels within sane bounds.

export const MIN_WIDGET_SPAN = 2
export const MAX_WIDGET_SPAN = 12
export const MIN_WIDGET_HEIGHT = 140
export const MAX_WIDGET_HEIGHT = 900

export type WidgetHeight = 'small' | 'medium' | 'large' | number

/** widgetHeightPx resolves a widget height (legacy size class or free pixel
 * value) to pixels. */
export function widgetHeightPx(h: WidgetHeight): number {
  if (typeof h === 'number') {
    return Math.min(MAX_WIDGET_HEIGHT, Math.max(MIN_WIDGET_HEIGHT, Math.round(h)))
  }
  switch (h) {
    case 'small':
      return 180
    case 'large':
      return 440
    default:
      return 300
  }
}

/** spanFromDrag converts a horizontal drag delta into a new column span,
 * snapped to the 12-column grid of the given container width. */
export function spanFromDrag(startSpan: number, deltaPx: number, containerWidth: number): number {
  const colWidth = containerWidth > 0 ? containerWidth / MAX_WIDGET_SPAN : 1
  const deltaCols = Math.round(deltaPx / colWidth)
  return Math.min(MAX_WIDGET_SPAN, Math.max(MIN_WIDGET_SPAN, startSpan + deltaCols))
}

/** heightFromDrag converts a vertical drag delta into a new pixel height. */
export function heightFromDrag(startHeightPx: number, deltaPx: number): number {
  return Math.min(
    MAX_WIDGET_HEIGHT,
    Math.max(MIN_WIDGET_HEIGHT, Math.round(startHeightPx + deltaPx)),
  )
}
