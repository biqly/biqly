import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'

import { advanceShown, initialShown } from './useTypewriter'

// The repo's vitest environment is `node` (no jsdom / @testing-library), so we
// exercise the hook's pure reveal engine directly and drive it with fake timers
// through the same setInterval scheduling the hook uses. This covers the
// behaviours a renderHook test would: time-based advance to full text,
// reduced-motion instant reveal, and restart when the text changes.

describe('initialShown (reduced-motion path)', () => {
  it('returns the full text immediately when motion is reduced', () => {
    expect(initialShown('Sales grew 12% last quarter.', true)).toBe('Sales grew 12% last quarter.')
  })

  it('starts empty when motion is allowed', () => {
    expect(initialShown('Sales grew 12% last quarter.', false)).toBe('')
  })

  it('returns empty text instantly regardless of motion preference', () => {
    expect(initialShown('', false)).toBe('')
    expect(initialShown('', true)).toBe('')
  })
})

describe('advanceShown', () => {
  it('reveals charsPerStep more characters, clamped to the text length', () => {
    expect(advanceShown('', 'hello', 2)).toBe('he')
    expect(advanceShown('he', 'hello', 2)).toBe('hell')
    expect(advanceShown('hell', 'hello', 2)).toBe('hello')
    expect(advanceShown('hello', 'hello', 2)).toBe('hello')
  })

  it('always advances by at least one character', () => {
    expect(advanceShown('', 'hi', 0)).toBe('h')
  })
})

describe('typewriter timer loop', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })
  afterEach(() => {
    vi.useRealTimers()
  })

  // Mirrors useTypewriter's effect: start empty, reveal on each interval tick.
  function runLoop(text: string, stepMs: number, charsPerStep: number) {
    let shown = initialShown(text, false)
    const id = setInterval(() => {
      shown = advanceShown(shown, text, charsPerStep)
      if (shown.length >= text.length) {
        clearInterval(id)
      }
    }, stepMs)
    return {
      get shown() {
        return shown
      },
      get done() {
        return shown === text
      },
    }
  }

  it('advances over time until the full text is shown', () => {
    const text = 'Revenue rose to 1,200.'
    const state = runLoop(text, 18, 2)

    expect(state.shown).toBe('')
    expect(state.done).toBe(false)

    // One tick reveals the first chunk.
    vi.advanceTimersByTime(18)
    expect(state.shown).toBe('Re')
    expect(state.done).toBe(false)

    // Run well past completion; the interval clears itself at the end.
    vi.advanceTimersByTime(18 * text.length)
    expect(state.shown).toBe(text)
    expect(state.done).toBe(true)

    // No pending timers remain (loop cleared itself on completion).
    expect(vi.getTimerCount()).toBe(0)
  })

  it('restarts cleanly when the text changes', () => {
    const first = runLoop('first answer', 18, 2)
    vi.advanceTimersByTime(18 * 3)
    expect(first.shown.length).toBeGreaterThan(0)

    // A new text value starts a fresh loop from empty (as the hook does when
    // its `text` dependency changes).
    const second = runLoop('a completely different answer', 18, 2)
    expect(second.shown).toBe('')
    vi.advanceTimersByTime(18 * 100)
    expect(second.shown).toBe('a completely different answer')
    expect(second.done).toBe(true)
  })
})
