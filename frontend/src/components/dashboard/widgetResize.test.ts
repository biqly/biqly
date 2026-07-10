import { describe, expect, it } from 'vitest'

import { heightFromDrag, spanFromDrag, widgetHeightPx } from './widgetResize'

describe('widgetHeightPx', () => {
  it('maps legacy size classes to their pixel heights', () => {
    expect(widgetHeightPx('small')).toBe(180)
    expect(widgetHeightPx('medium')).toBe(300)
    expect(widgetHeightPx('large')).toBe(440)
  })

  it('passes through numeric heights, clamped to bounds', () => {
    expect(widgetHeightPx(250)).toBe(250)
    expect(widgetHeightPx(10)).toBe(140)
    expect(widgetHeightPx(5000)).toBe(900)
  })
})

describe('spanFromDrag', () => {
  it('snaps the drag delta to grid columns', () => {
    // 1200px container → 100px per column.
    expect(spanFromDrag(6, 210, 1200)).toBe(8)
    expect(spanFromDrag(6, -160, 1200)).toBe(4)
    expect(spanFromDrag(6, 30, 1200)).toBe(6)
  })

  it('clamps to the 2..12 span range', () => {
    expect(spanFromDrag(6, 10000, 1200)).toBe(12)
    expect(spanFromDrag(6, -10000, 1200)).toBe(2)
  })
})

describe('heightFromDrag', () => {
  it('adds the delta and clamps to bounds', () => {
    expect(heightFromDrag(300, 50)).toBe(350)
    expect(heightFromDrag(300, -400)).toBe(140)
    expect(heightFromDrag(800, 400)).toBe(900)
  })
})
