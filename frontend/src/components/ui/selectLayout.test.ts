import { describe, expect, it } from 'vitest'

import { resolveSelectPopoverCoords, selectPopoverFixedStyle } from './selectLayout'

const pageSizeOptions = [
  { value: '5', label: '5' },
  { value: '10', label: '10' },
  { value: '25', label: '25' },
  { value: '50', label: '50' },
]

function rect(overrides: Partial<DOMRect> = {}): DOMRect {
  return {
    x: 24,
    y: 850,
    top: 850,
    left: 24,
    right: 104,
    bottom: 880,
    width: 80,
    height: 30,
    toJSON: () => ({}),
    ...overrides,
  }
}

describe('resolveSelectPopoverCoords', () => {
  it('opens downward when there is room below the trigger', () => {
    const coords = resolveSelectPopoverCoords(
      rect({ top: 200, bottom: 230 }),
      pageSizeOptions,
      11.5,
      288,
      900,
    )

    expect(coords.placement).toBe('down')
    expect(coords.top).toBe(236)
    expect(coords.bottom).toBeUndefined()
  })

  it('anchors upward from the trigger bottom when near the viewport edge', () => {
    const trigger = rect()
    const viewportHeight = 900
    const coords = resolveSelectPopoverCoords(trigger, pageSizeOptions, 11.5, 288, viewportHeight)

    expect(coords.placement).toBe('up')
    expect(coords.top).toBeUndefined()
    expect(coords.bottom).toBe(viewportHeight - trigger.top + 6)
  })

  it('never renders the menu narrower than a wide trigger (edges stay aligned)', () => {
    // A wide trigger (600px) with short options must not be capped below its
    // own width — otherwise the open menu reads as misaligned under it.
    const wide = rect({ left: 100, right: 700, width: 600, top: 200, bottom: 230 })
    const coords = resolveSelectPopoverCoords(wide, pageSizeOptions, 12.5, 288, 900)

    expect(coords.left).toBe(100)
    expect(coords.width).toBeGreaterThanOrEqual(600)
  })
})

describe('selectPopoverFixedStyle', () => {
  it('uses bottom positioning for upward placement', () => {
    const style = selectPopoverFixedStyle({
      left: 24,
      width: 80,
      placement: 'up',
      bottom: 56,
    })

    expect(style).toEqual({
      position: 'fixed',
      left: 24,
      width: 80,
      bottom: 56,
    })
    expect(style.top).toBeUndefined()
  })

  it('uses top positioning for downward placement', () => {
    const style = selectPopoverFixedStyle({
      left: 24,
      width: 80,
      placement: 'down',
      top: 236,
    })

    expect(style).toEqual({
      position: 'fixed',
      left: 24,
      width: 80,
      top: 236,
    })
    expect(style.bottom).toBeUndefined()
  })
})
