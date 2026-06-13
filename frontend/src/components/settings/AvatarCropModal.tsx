import React, { useCallback, useEffect, useRef, useState } from 'react'

import { useT } from '../../i18n'
import { legacyButtonClass } from '../../lib/buttonClasses'
import { cn } from '../../lib/cn'
import {
  modalActionsClass,
  modalAvatarCardClass,
  modalBackdropClass,
  modalBodyClass,
  modalCloseClass,
  modalHeaderClass,
  modalSubtitleClass,
  modalTitleClass,
} from '../../lib/modalClasses'

interface AvatarCropModalProps {
  imageSrc: string
  onClose: () => void
  onSave: (croppedBase64: string) => Promise<void>
}

export function AvatarCropModal({ imageSrc, onClose, onSave }: AvatarCropModalProps) {
  const t = useT()
  const canvasRef = useRef<HTMLCanvasElement>(null)

  const [imgElement, setImgElement] = useState<HTMLImageElement | null>(null)
  const [zoom, setZoom] = useState(1.0)
  const [minZoom, setMinZoom] = useState(1.0)
  const [maxZoom, setMaxZoom] = useState(3.0)
  const [pan, setPan] = useState({ x: 0, y: 0 })
  const [saving, setSaving] = useState(false)
  const [isDragging, setIsDragging] = useState(false)

  const startOffset = useRef({ x: 0, y: 0 })

  useEffect(() => {
    const img = new Image()
    img.onload = () => {
      const baseZoom = 200 / Math.min(img.width, img.height)
      setZoom(baseZoom)
      setMinZoom(baseZoom)
      setMaxZoom(baseZoom * 4.0)
      setPan({ x: 0, y: 0 })
      setImgElement(img)
    }
    img.src = imageSrc
  }, [imageSrc])

  const clampPan = (x: number, y: number, currentZoom: number) => {
    if (!imgElement) {
      return { x, y }
    }
    const w = imgElement.width * currentZoom
    const h = imgElement.height * currentZoom
    const minX = 100 - w / 2
    const maxX = w / 2 - 100
    const minY = 100 - h / 2
    const maxY = h / 2 - 100

    return {
      x: Math.min(Math.max(x, minX), maxX),
      y: Math.min(Math.max(y, minY), maxY),
    }
  }

  const draw = useCallback(() => {
    const canvas = canvasRef.current
    if (!canvas || !imgElement) {
      return
    }
    const ctx = canvas.getContext('2d')
    if (!ctx) {
      return
    }

    ctx.clearRect(0, 0, canvas.width, canvas.height)

    // 1. Draw image centered, panned, zoomed
    ctx.save()
    ctx.translate(canvas.width / 2 + pan.x, canvas.height / 2 + pan.y)
    ctx.scale(zoom, zoom)
    ctx.drawImage(imgElement, -imgElement.width / 2, -imgElement.height / 2)
    ctx.restore()

    // 2. Draw crop mask overlay (darken outside 200px crop circle)
    ctx.save()
    ctx.fillStyle = 'rgba(9, 9, 11, 0.7)'
    ctx.beginPath()
    ctx.rect(0, 0, canvas.width, canvas.height)
    ctx.arc(canvas.width / 2, canvas.height / 2, 100, 0, Math.PI * 2, true)
    ctx.fill()
    ctx.restore()

    // 3. Draw crop border
    ctx.beginPath()
    ctx.arc(canvas.width / 2, canvas.height / 2, 100, 0, Math.PI * 2)
    ctx.strokeStyle = 'var(--accent, #6366f1)'
    ctx.lineWidth = 2.5
    ctx.stroke()
  }, [imgElement, pan.x, pan.y, zoom])

  useEffect(() => {
    draw()
  }, [draw])

  const handleMouseDown = (e: React.MouseEvent<HTMLCanvasElement>) => {
    if (!imgElement) {
      return
    }
    setIsDragging(true)
    startOffset.current = { x: e.clientX - pan.x, y: e.clientY - pan.y }
  }

  const handleMouseMove = (e: React.MouseEvent<HTMLCanvasElement>) => {
    if (!isDragging || !imgElement) {
      return
    }
    const nextPan = clampPan(
      e.clientX - startOffset.current.x,
      e.clientY - startOffset.current.y,
      zoom,
    )
    setPan(nextPan)
  }

  const handleMouseUpOrLeave = () => {
    setIsDragging(false)
  }

  const handleTouchStart = (e: React.TouchEvent<HTMLCanvasElement>) => {
    if (!imgElement || e.touches.length !== 1) {
      return
    }
    setIsDragging(true)
    const touch = e.touches[0]
    if (!touch) {
      return
    }
    startOffset.current = { x: touch.clientX - pan.x, y: touch.clientY - pan.y }
  }

  const handleTouchMove = (e: React.TouchEvent<HTMLCanvasElement>) => {
    if (!isDragging || !imgElement || e.touches.length !== 1) {
      return
    }
    const touch = e.touches[0]
    if (!touch) {
      return
    }
    const nextPan = clampPan(
      touch.clientX - startOffset.current.x,
      touch.clientY - startOffset.current.y,
      zoom,
    )
    setPan(nextPan)
  }

  const handleTouchEnd = () => {
    setIsDragging(false)
  }

  const handleZoomChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const nextZoom = parseFloat(e.target.value)
    setZoom(nextZoom)
    setPan((prev) => clampPan(prev.x, prev.y, nextZoom))
  }

  const handleSave = async () => {
    if (!imgElement) {
      return
    }
    setSaving(true)
    try {
      const saveCanvas = document.createElement('canvas')
      saveCanvas.width = 200
      saveCanvas.height = 200
      const sCtx = saveCanvas.getContext('2d')
      if (!sCtx) {
        throw new Error('Could not create save canvas context')
      }

      sCtx.save()
      sCtx.translate(100 + pan.x, 100 + pan.y)
      sCtx.scale(zoom, zoom)
      sCtx.drawImage(imgElement, -imgElement.width / 2, -imgElement.height / 2)
      sCtx.restore()

      const base64 = saveCanvas.toDataURL('image/jpeg', 0.85)
      await onSave(base64)
    } catch (err) {
      console.error('Failed to crop avatar:', err)
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className={modalBackdropClass()} onClick={onClose}>
      <div className={modalAvatarCardClass()} onClick={(e) => e.stopPropagation()}>
        <div className={modalHeaderClass()}>
          <h2 className={modalTitleClass()}>{t('settings.profile_picture_crop_title')}</h2>
          <button type="button" className={modalCloseClass()} onClick={onClose}>
            ×
          </button>
        </div>
        <div className={cn(modalBodyClass(), 'items-center justify-items-center gap-5')}>
          <p className={cn(modalSubtitleClass(), 'w-full text-center')}>
            {t('settings.profile_picture_crop_desc')}
          </p>
          <div
            style={{
              position: 'relative',
              width: '320px',
              height: '320px',
              background: '#09090b',
              borderRadius: '0.5rem',
              overflow: 'hidden',
              cursor: isDragging ? 'grabbing' : 'grab',
              border: '1px solid var(--border)',
            }}
          >
            <canvas
              ref={canvasRef}
              width={320}
              height={320}
              onMouseDown={handleMouseDown}
              onMouseMove={handleMouseMove}
              onMouseUp={handleMouseUpOrLeave}
              onMouseLeave={handleMouseUpOrLeave}
              onTouchStart={handleTouchStart}
              onTouchMove={handleTouchMove}
              onTouchEnd={handleTouchEnd}
              style={{
                display: 'block',
                width: '300%',
                height: '100%',
                position: 'absolute',
                top: 0,
                left: 0,
              }}
            />
            {/* Direct style correction for exact canvas viewport centering */}
            <style>{`
              canvas {
                left: 0 !important;
                width: 100% !important;
              }
            `}</style>
          </div>

          <div style={{ width: '100%', padding: '0 0.5rem' }}>
            <input
              type="range"
              min={minZoom}
              max={maxZoom}
              step={0.001}
              value={zoom}
              onChange={handleZoomChange}
              style={{
                width: '100%',
                height: '5px',
                borderRadius: '999px',
                background: 'var(--border)',
                accentColor: 'var(--accent, #6366f1)',
                cursor: 'pointer',
              }}
            />
          </div>
        </div>
        <div className={modalActionsClass()}>
          <button
            type="button"
            className={legacyButtonClass('btn btn-secondary btn-sm')}
            onClick={onClose}
            disabled={saving}
          >
            {t('common.cancel')}
          </button>
          <button
            type="button"
            className={legacyButtonClass('btn btn-primary btn-sm')}
            onClick={() => {
              void handleSave()
            }}
            disabled={saving || !imgElement}
          >
            {saving ? '…' : t('common.save')}
          </button>
        </div>
      </div>
    </div>
  )
}
