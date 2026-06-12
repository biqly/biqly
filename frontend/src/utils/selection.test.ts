import { describe, expect, it } from 'vitest'

import { sameIdSet, selectionStateFor, setIds, toggleId } from './selection'

describe('toggleId', () => {
  it('adds a missing id and removes a present one, without mutating the input', () => {
    const base = new Set(['a'])
    const added = toggleId(base, 'b')
    expect([...added].sort()).toEqual(['a', 'b'])
    const removed = toggleId(added, 'a')
    expect([...removed]).toEqual(['b'])
    expect([...base]).toEqual(['a'])
  })
})

describe('setIds', () => {
  const base = new Set(['a', 'b'])

  it('adds a whole group when on', () => {
    expect([...setIds(base, ['b', 'c', 'd'], true)].sort()).toEqual(['a', 'b', 'c', 'd'])
  })

  it('removes a whole group when off', () => {
    expect([...setIds(base, ['b', 'c'], false)]).toEqual(['a'])
  })
})

describe('sameIdSet', () => {
  it('compares by content (the RolesPanel dirty check)', () => {
    expect(sameIdSet(new Set(['a', 'b']), new Set(['b', 'a']))).toBe(true)
    expect(sameIdSet(new Set(['a']), new Set(['a', 'b']))).toBe(false)
    expect(sameIdSet(new Set(['a', 'c']), new Set(['a', 'b']))).toBe(false)
    expect(sameIdSet(new Set(), new Set())).toBe(true)
  })
})

describe('selectionStateFor', () => {
  const group = ['a', 'b', 'c']

  it('maps to none / some / all for the select-all checkbox', () => {
    expect(selectionStateFor(new Set(), group)).toBe('none')
    expect(selectionStateFor(new Set(['a']), group)).toBe('some')
    expect(selectionStateFor(new Set(['a', 'b', 'c']), group)).toBe('all')
  })

  it('treats an empty group as none', () => {
    expect(selectionStateFor(new Set(['a']), [])).toBe('none')
  })

  it('ignores ids outside the group', () => {
    expect(selectionStateFor(new Set(['x', 'y']), group)).toBe('none')
  })
})
