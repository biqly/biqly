import * as React from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useModal } from './useModal'

interface MockReact {
  __resetHooks: () => void
  __getStates: () => unknown[]
}

vi.mock('react', () => {
  let states: unknown[] = []
  let index = 0
  const setters: ((newVal: unknown) => void)[] = []

  const useStateMock = (initialVal: unknown) => {
    const curIdx = index++
    if (states.length <= curIdx) {
      states.push(initialVal)
      setters.push((newVal: unknown) => {
        states[curIdx] =
          typeof newVal === 'function'
            ? (newVal as (prev: unknown) => unknown)(states[curIdx])
            : newVal
      })
    }
    return [states[curIdx], setters[curIdx]]
  }

  return {
    useState: useStateMock,
    useCallback: <T>(fn: T): T => fn,
    __resetHooks: () => {
      states = []
      index = 0
    },
    __getStates: () => states,
  }
})

const mockReact = React as unknown as MockReact

describe('useModal', () => {
  beforeEach(() => {
    mockReact.__resetHooks()
  })

  it('initializes with open false and null data', () => {
    const { open, data } = useModal()
    expect(open).toBe(false)
    expect(data).toBeNull()
  })

  it('sets open to true and saves data on openModal', () => {
    const { openModal } = useModal<string>()

    openModal('test-item')

    const states = mockReact.__getStates()
    expect(states[0]).toBe(true)
    expect(states[1]).toBe('test-item')
  })

  it('sets open to false and clears data on closeModal', () => {
    const { closeModal } = useModal<string>()

    closeModal()

    const states = mockReact.__getStates()
    expect(states[0]).toBe(false)
    expect(states[1]).toBeNull()
  })
})
