import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'

import type { ColumnRow, SemanticJoin, SemanticModelDetail, TableRow } from '../../types/semantic'
import {
  applyDragDelta,
  applyKeyboardMove,
  buildCardLayouts,
  computeCanvasBounds,
  computeJoinPath,
  keyboardDeltaFromKey,
  layoutInitialPositions,
  panViewport,
  snapScaleNearest,
  zoomStep,
  zoomViewportAtPoint,
} from './canvasMath'
import { COL_LIMIT } from './constants'
import type { CardLayout, Pt, Viewport } from './types'
import { tableKey } from './utils'

export function useModelingCanvas(
  modelId: string,
  tableCards: TableRow[],
  columns: ColumnRow[],
  model: SemanticModelDetail | null,
) {
  const [positions, setPositions] = useState<Record<string, Pt>>({})
  const [viewport, setViewport] = useState<Viewport>({ scale: 1, tx: 0, ty: 0 })

  const viewportRef = useRef(viewport)
  viewportRef.current = viewport
  const wrapRef = useRef<HTMLDivElement | null>(null)

  const cardLayouts = useMemo(() => {
    const joinColumns = new Map<string, Set<string>>()
    for (const join of (model?.joins ?? []).filter((j) => j.is_active !== false)) {
      const fromKey = tableKey(join.from_schema || model?.base_schema || '', join.from_table)
      const toKey = tableKey(join.to_schema || model?.base_schema || '', join.to_table)
      if (!joinColumns.has(fromKey)) {
        joinColumns.set(fromKey, new Set())
      }
      if (!joinColumns.has(toKey)) {
        joinColumns.set(toKey, new Set())
      }
      joinColumns.get(fromKey)!.add(join.from_column)
      joinColumns.get(toKey)!.add(join.to_column)
    }
    return buildCardLayouts(tableCards, columns, joinColumns, COL_LIMIT)
  }, [tableCards, columns, model])

  useEffect(() => {
    setPositions({})
    setViewport({ scale: 1, tx: 0, ty: 0 })
  }, [modelId])

  useEffect(() => {
    setPositions(layoutInitialPositions(tableCards, cardLayouts))
  }, [tableCards, cardLayouts])

  const canvasBounds = useMemo(
    () => computeCanvasBounds(tableCards, positions, cardLayouts),
    [tableCards, positions, cardLayouts],
  )

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
      const onMove = (ev: MouseEvent) => {
        const scale = viewportRef.current.scale
        const dx = ev.clientX - startX
        const dy = ev.clientY - startY
        setPositions((prev) => ({
          ...prev,
          [key]: applyDragDelta(startPos, dx, dy, scale),
        }))
      }
      const onUp = () => {
        window.removeEventListener('mousemove', onMove)
        window.removeEventListener('mouseup', onUp)
        document.body.classList.remove('modeling-grabbing')
      }
      document.body.classList.add('modeling-grabbing')
      window.addEventListener('mousemove', onMove)
      window.addEventListener('mouseup', onUp)
    },
    [positions],
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
      document.body.classList.remove('modeling-panning')
    }
    document.body.classList.add('modeling-panning')
    window.addEventListener('mousemove', onMove)
    window.addEventListener('mouseup', onUp)
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
        const direction: 1 | -1 = ev.deltaY < 0 ? 1 : -1
        const newScale = zoomStep(vp.scale, direction)
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
  }
}

export type ModelingCanvasState = ReturnType<typeof useModelingCanvas>
