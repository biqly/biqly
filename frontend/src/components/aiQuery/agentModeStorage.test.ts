// @vitest-environment jsdom
//
// Exercises the Agent Mode toggle's localStorage persistence
// (loadAgentModeEnabled/saveAgentModeEnabled). This suite needs jsdom for
// `window`, so it is split out from the default `node` test environment
// (see FollowUpSuggestionsSection.test.tsx for the same per-file docblock
// pattern). jsdom in this project doesn't implement window.localStorage out
// of the box, so a minimal in-memory Storage stand-in is installed before
// each test — the same round-trip a real browser's same-origin persistence
// provides across a remount/reload.
import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import { loadAgentModeEnabled, saveAgentModeEnabled } from './agentModeStorage'

const STORAGE_KEY = 'biqly.aiQuery.agentMode'

class FakeStorage implements Pick<Storage, 'getItem' | 'setItem' | 'removeItem' | 'clear'> {
  private store = new Map<string, string>()
  getItem(key: string): string | null {
    return this.store.has(key) ? (this.store.get(key) ?? null) : null
  }
  setItem(key: string, value: string): void {
    this.store.set(key, value)
  }
  removeItem(key: string): void {
    this.store.delete(key)
  }
  clear(): void {
    this.store.clear()
  }
}

beforeEach(() => {
  Object.defineProperty(window, 'localStorage', { value: new FakeStorage(), configurable: true })
})

afterEach(() => {
  window.localStorage.clear()
})

describe('Agent Mode toggle persistence', () => {
  it('defaults to disabled with nothing in storage', () => {
    expect(window.localStorage.getItem(STORAGE_KEY)).toBeNull()
    expect(loadAgentModeEnabled()).toBe(false)
  })

  it('round-trips enabled across a reload/remount', () => {
    saveAgentModeEnabled(true)
    // A fresh call to loadAgentModeEnabled simulates the state a remounted
    // AIQuery component would read via useState(loadAgentModeEnabled).
    expect(loadAgentModeEnabled()).toBe(true)
  })

  it('round-trips disabled across a reload/remount', () => {
    saveAgentModeEnabled(true)
    saveAgentModeEnabled(false)
    expect(loadAgentModeEnabled()).toBe(false)
  })

  it('persists under its own storage key, independent of other toggles', () => {
    saveAgentModeEnabled(true)
    expect(window.localStorage.getItem(STORAGE_KEY)).toBe('true')
    expect(window.localStorage.getItem('biqly.aiQuery.autoFindSkills')).toBeNull()
  })

  it('falls back to the default when localStorage throws', () => {
    Object.defineProperty(window, 'localStorage', {
      value: {
        getItem: () => {
          throw new Error('storage disabled')
        },
        clear: () => {
          // no-op: satisfies the afterEach cleanup below.
        },
      },
      configurable: true,
    })
    expect(loadAgentModeEnabled()).toBe(false)
  })
})
