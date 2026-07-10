// @vitest-environment jsdom
//
// jsdom in this project doesn't implement window.localStorage, so a minimal
// in-memory Storage stand-in is installed per test (same pattern as
// agentModeStorage.test.ts).
import { afterEach, beforeEach, describe, expect, it } from 'vitest'

import { loadPinnedIds, savePinnedIds, togglePinnedId } from './conversationPins'

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

describe('conversationPins', () => {
  it('round-trips pinned ids through localStorage', () => {
    savePinnedIds(new Set(['a', 'b']))
    const loaded = loadPinnedIds()
    expect([...loaded].sort()).toEqual(['a', 'b'])
  })

  it('returns an empty set when nothing is stored', () => {
    expect(loadPinnedIds().size).toBe(0)
  })

  it('ignores corrupt storage', () => {
    window.localStorage.setItem('biqly.aiQuery.pinnedConversations', '{not json')
    expect(loadPinnedIds().size).toBe(0)
  })

  it('toggles immutably', () => {
    const base = new Set(['a'])
    const added = togglePinnedId(base, 'b')
    expect([...added].sort()).toEqual(['a', 'b'])
    expect([...base]).toEqual(['a'])
    const removed = togglePinnedId(added, 'a')
    expect([...removed]).toEqual(['b'])
  })
})
