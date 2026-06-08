import { unknownToDisplayString } from '../../utils/formatters'
import type { ContextMenuState } from './types'

export function isContextMenuKey(key: string, shiftKey: boolean): boolean {
  return key === 'ContextMenu' || (shiftKey && key === 'F10')
}

export function buildContextMenuFromPointer(
  x: number,
  y: number,
  colName: string,
  value: unknown,
): ContextMenuState {
  return { x, y, colName, value: unknownToDisplayString(value) }
}

export function buildContextMenuFromCellRect(
  rect: Pick<DOMRect, 'left' | 'bottom'>,
  colName: string,
  value: unknown,
): ContextMenuState {
  return buildContextMenuFromPointer(rect.left, rect.bottom, colName, value)
}
