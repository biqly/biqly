import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'

import type { ColumnRow, SemanticJoin, SemanticModelDetail, TableRow } from '../../types/semantic'
import {
  applyDragDelta,
  applyKeyboardMove,
  buildCardLayouts,
  buildCardSections,
  computeCanvasBounds,
  computeJoinPath,
  continuousZoomScale,
  exceedsDragThreshold,
  keyboardDeltaFromKey,
  layoutInitialPositions,
  panViewport,
  snapScaleNearest,
  zoomStep,
  zoomViewportAtPoint,
} from './canvasMath'
import { CARD_WIDTH, COL_LIMIT } from './constants'
import type { Pt, Viewport } from './types'
import { tableKey } from './utils'

const VISIBLE_COLUMNS_STORAGE_PREFIX = 'biqly.modeling.visibleColumns.'

// Per-model view preference: which columns are shown on each table card.
// Stored locally only (no backend), keyed by model id.
function loadVisibleByTable(modelId: string): Map<string, Set<string>> {
  const out = new Map<string, Set<string>>()
  if (!modelId || typeof window === 'undefined') {
    return out
  }
  try {
    const raw = window.localStorage.getItem(`${VISIBLE_COLUMNS_STORAGE_PREFIX}${modelId}`)
    if (!raw) {
      return out
    }
    const parsed: unknown = JSON.parse(raw)
    if (parsed && typeof parsed === 'object') {
      for (const [key, cols] of Object.entries(parsed as Record<string, unknown>)) {
        if (Array.isArray(cols)) {
          out.set(key, new Set(cols.filter((c): c is string => typeof c === 'string')))
        }
      }
    }
  } catch {
    // Ignore malformed / unavailable storage and fall back to defaults.
  }
  return out
}

function saveVisibleByTable(modelId: string, map: Map<string, Set<string>>) {
  if (!modelId || typeof window === 'undefined') {
    return
  }
  try {
    const obj: Record<string, string[]> = {}
    for (const [key, set] of map) {
      obj[key] = [...set]
    }
    window.localStorage.setItem(`${VISIBLE_COLUMNS_STORAGE_PREFIX}${modelId}`, JSON.stringify(obj))
  } catch {
    // Ignore quota / serialization failures; the preference is best-effort.
  }
}

export function useModelingCanvas(
  modelId: string,
  tableCards: TableRow[],
  columns: ColumnRow[],
  model: SemanticModelDetail | null,
  onCardClick?: (key: string) => void,
) {
  const [positions, setPositions] = useState<Record<string, Pt>>({})
  const [viewport, setViewport] = useState<Viewport>({ scale: 1, tx: 0, ty: 0 })
  const [visibleByTable, setVisibleByTable] = useState<Map<string, Set<string>>>(() =>
    loadVisibleByTable(modelId),
  )

  const viewportRef = useRef(viewport)
  const wrapRef = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    viewportRef.current = viewport
  }, [viewport])

  const cardLayouts = useMemo(() => {
    const joinColumns = new Map<string, Set<string>>()
    for (const join of (model?.joins ?? []).filter((j) => j.is_active !== false)) {
      const fromKey = tableKey(join.from_schema ?? model?.base_schema ?? '', join.from_table)
      const toKey = tableKey(join.to_schema ?? model?.base_schema ?? '', join.to_table)
      if (!joinColumns.has(fromKey)) {
        joinColumns.set(fromKey, new Set())
      }
      if (!joinColumns.has(toKey)) {
        joinColumns.set(toKey, new Set())
      }
      joinColumns.get(fromKey)!.add(join.from_column)
      joinColumns.get(toKey)!.add(join.to_column)
    }
    const sections = buildCardSections(tableCards, model)
    return buildCardLayouts(tableCards, columns, joinColumns, COL_LIMIT, sections, visibleByTable)
  }, [tableCards, columns, model, visibleByTable])

  useEffect(() => {
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setPositions({})
    setViewport({ scale: 1, tx: 0, ty: 0 })
    setVisibleByTable(loadVisibleByTable(modelId))
  }, [modelId])

  useEffect(() => {
    // Preserve any card the user has already positioned; only auto-place cards
    // that don't have a position yet. This keeps toggling column visibility
    // (which changes card heights, and thus cardLayouts) from snapping every
    // card back to the initial grid.
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setPositions((prev) => {
      const fresh = layoutInitialPositions(tableCards, cardLayouts)
      const merged: Record<string, Pt> = {}
      for (const key of Object.keys(fresh)) {
        merged[key] = prev[key] ?? fresh[key]!
      }
      return merged
    })
  }, [tableCards, cardLayouts])

  const setTableVisibleColumns = useCallback(
    (key: string, cols: string[]) => {
      setVisibleByTable((prev) => {
        const next = new Map(prev)
        next.set(key, new Set(cols))
        saveVisibleByTable(modelId, next)
        return next
      })
    },
    [modelId],
  )

  const canvasBounds = useMemo(
    () => computeCanvasBounds(tableCards, positions, cardLayouts),
    [tableCards, positions, cardLayouts],
  )

  const savedViewportRef = useRef<Viewport | null>(null)
  // Ends whatever drag/pan is currently active. Tracked so a mid-drag unmount
  // can force-end it (otherwise the window listeners leak and the card keeps
  // following the cursor).
  const activeDragCleanupRef = useRef<(() => void) | null>(null)

  const onCardDragStart = useCallback(
    (key: string) => (event: React.MouseEvent) => {
      if (event.button !== 0) {
        return
      }
      const target = event.target as HTMLElement
      if (target.closest('button')) {
        return
      }
      event.preventDefault()
      event.stopPropagation()
      const startX = event.clientX
      const startY = event.clientY
      const startPos = positions[key] ?? { x: 0, y: 0 }
      let moved = false
      const onMove = (ev: MouseEvent) => {
        const scale = viewportRef.current.scale
        const dx = ev.clientX - startX
        const dy = ev.clientY - startY
        if (exceedsDragThreshold(dx, dy)) {
          moved = true
        }
        setPositions((prev) => ({
          ...prev,
          [key]: applyDragDelta(startPos, dx, dy, scale),
        }))
      }
      const onUp = () => {
        window.removeEventListener('mousemove', onMove)
        window.removeEventListener('mouseup', onUp)
        // Also end on window blur so releasing the button outside the OS window
        // (multi-monitor / fast drag) doesn't leave the drag stuck.
        window.removeEventListener('blur', onUp)
        document.body.classList.remove('modeling-grabbing')
        activeDragCleanupRef.current = null
        if (!moved) {
          onCardClick?.(key)
        }
      }
      activeDragCleanupRef.current = onUp
      document.body.classList.add('modeling-grabbing')
      window.addEventListener('mousemove', onMove)
      window.addEventListener('mouseup', onUp)
      window.addEventListener('blur', onUp)
    },
    [positions, onCardClick],
  )

  const onCardKeyDown = useCallback(
    (key: string) => (event: React.KeyboardEvent) => {
      const delta = keyboardDeltaFromKey(event.key)
      if (!delta) {
        return
      }
      event.preventDefault()
      setPositions((prev) => {
        const cur = prev[key] ?? { x: 0, y: 0 }
        return {
          ...prev,
          [key]: applyKeyboardMove(cur, delta.dx, delta.dy, event.shiftKey),
        }
      })
    },
    [],
  )

  const onCanvasMouseDown = useCallback((event: React.MouseEvent) => {
    const target = event.target as HTMLElement
    if (target.closest('.modeling-table-card')) {
      return
    }
    if (event.button !== 0) {
      return
    }
    event.preventDefault()
    const startX = event.clientX
    const startY = event.clientY
    const startVP = viewportRef.current
    const onMove = (ev: MouseEvent) => {
      setViewport(panViewport(startVP, ev.clientX - startX, ev.clientY - startY))
    }
    const onUp = () => {
      window.removeEventListener('mousemove', onMove)
      window.removeEventListener('mouseup', onUp)
      window.removeEventListener('blur', onUp)
      document.body.classList.remove('modeling-panning')
      activeDragCleanupRef.current = null
    }
    activeDragCleanupRef.current = onUp
    document.body.classList.add('modeling-panning')
    window.addEventListener('mousemove', onMove)
    window.addEventListener('mouseup', onUp)
    window.addEventListener('blur', onUp)
  }, [])

  // Force-end any in-flight drag/pan if the hook unmounts mid-drag.
  useEffect(() => {
    return () => {
      activeDragCleanupRef.current?.()
    }
  }, [])

  useLayoutEffect(() => {
    const node = wrapRef.current
    if (!node) {
      return
    }
    const onWheel = (ev: WheelEvent) => {
      if (!ev.ctrlKey && !ev.metaKey && Math.abs(ev.deltaX) > Math.abs(ev.deltaY)) {
        return
      }
      ev.preventDefault()
      const rect = node.getBoundingClientRect()
      const cx = ev.clientX - rect.left
      const cy = ev.clientY - rect.top
      setViewport((vp) => {
        const newScale = continuousZoomScale(vp.scale, ev.deltaY)
        return zoomViewportAtPoint(vp, cx, cy, newScale)
      })
    }
    node.addEventListener('wheel', onWheel, { passive: false })
    return () => node.removeEventListener('wheel', onWheel)
  }, [])

  const zoomBy = useCallback((direction: 1 | -1) => {
    const node = wrapRef.current
    setViewport((vp) => {
      const newScale = zoomStep(vp.scale, direction)
      if (newScale === vp.scale) {
        return vp
      }
      if (!node) {
        return { ...vp, scale: newScale }
      }
      const rect = node.getBoundingClientRect()
      const cx = rect.width / 2
      const cy = rect.height / 2
      return zoomViewportAtPoint(vp, cx, cy, newScale)
    })
  }, [])

  const resetView = useCallback(() => setViewport({ scale: 1, tx: 0, ty: 0 }), [])

  const fitView = useCallback(() => {
    const node = wrapRef.current
    if (!node) {
      return
    }
    const rect = node.getBoundingClientRect()
    const padding = 40
    const scaleX = (rect.width - padding * 2) / Math.max(1, canvasBounds.width)
    const scaleY = (rect.height - padding * 2) / Math.max(1, canvasBounds.height)
    const scale = snapScaleNearest(Math.min(scaleX, scaleY, 1))
    setViewport({ scale, tx: padding, ty: padding })
  }, [canvasBounds.width, canvasBounds.height])

  const getJoinPath = useCallback(
    (join: SemanticJoin) => computeJoinPath(join, model?.base_schema ?? '', positions, cardLayouts),
    [model?.base_schema, positions, cardLayouts],
  )

  const panToKeys = useCallback(
    (keys: string[]) => {
      const node = wrapRef.current
      if (!node || keys.length === 0) {
        return
      }
      savedViewportRef.current = viewportRef.current
      const padding = 60
      let minX = Infinity
      let minY = Infinity
      let maxX = -Infinity
      let maxY = -Infinity
      for (const key of keys) {
        const pos = positions[key]
        if (!pos) {
          continue
        }
        const layout = cardLayouts.get(key)
        const h = layout?.height ?? 200
        minX = Math.min(minX, pos.x)
        minY = Math.min(minY, pos.y)
        maxX = Math.max(maxX, pos.x + CARD_WIDTH)
        maxY = Math.max(maxY, pos.y + h)
      }
      if (!isFinite(minX)) {
        return
      }
      const rect = node.getBoundingClientRect()
      const boxW = maxX - minX + padding * 2
      const boxH = maxY - minY + padding * 2
      const scaleX = rect.width / boxW
      const scaleY = rect.height / boxH
      const scale = snapScaleNearest(Math.min(scaleX, scaleY, 1))
      const tx = -minX * scale + (rect.width - (maxX - minX) * scale) / 2 + padding
      const ty = -minY * scale + (rect.height - (maxY - minY) * scale) / 2 + padding
      setViewport({ scale, tx, ty })
    },
    [positions, cardLayouts],
  )

  const restoreSavedViewport = useCallback(() => {
    const saved = savedViewportRef.current
    if (saved) {
      setViewport(saved)
      savedViewportRef.current = null
    }
  }, [])

  return {
    positions,
    viewport,
    wrapRef,
    cardLayouts,
    canvasBounds,
    onCardDragStart,
    onCardKeyDown,
    onCanvasMouseDown,
    zoomBy,
    resetView,
    fitView,
    getJoinPath,
    panToKeys,
    restoreSavedViewport,
    visibleByTable,
    setTableVisibleColumns,
  }
}

export type ModelingCanvasState = ReturnType<typeof useModelingCanvas>
