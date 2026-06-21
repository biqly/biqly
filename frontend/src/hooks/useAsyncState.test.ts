import * as React from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useAsyncState } from './useAsyncState'

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

describe('useAsyncState', () => {
  beforeEach(() => {
    mockReact.__resetHooks()
  })

  it('initializes with correct default state', () => {
    const state = useAsyncState()
    expect(state.loading).toBe(false)
    expect(state.saving).toBe(false)
    expect(state.error).toBeNull()
  })

  it('handles successful loading task', async () => {
    const state = useAsyncState()
    const task = () => Promise.resolve('result-value')

    const promise = state.run(task)

    // Verify state matches during run
    const activeStates = mockReact.__getStates()
    expect(activeStates[0]).toBe(true) // loading set to true
    expect(activeStates[1]).toBe(false) // saving is false
    expect(activeStates[2]).toBeNull() // error is null

    const result = await promise
    expect(result).toBe('result-value')

    const finalStates = mockReact.__getStates()
    expect(finalStates[0]).toBe(false) // loading set back to false
  })

  it('handles successful saving task', async () => {
    const state = useAsyncState({ useSaving: true })
    const task = () => Promise.resolve('result-value')

    const promise = state.run(task)

    // Verify state matches during run
    const activeStates = mockReact.__getStates()
    expect(activeStates[0]).toBe(false) // loading is false
    expect(activeStates[1]).toBe(true) // saving set to true
    expect(activeStates[2]).toBeNull() // error is null

    const result = await promise
    expect(result).toBe('result-value')

    const finalStates = mockReact.__getStates()
    expect(finalStates[1]).toBe(false) // saving set back to false
  })

  it('captures error messages on failure', async () => {
    const state = useAsyncState()
    const task = () => Promise.reject(new Error('something went wrong'))

    const result = await state.run(task)
    expect(result).toBeNull()

    const finalStates = mockReact.__getStates()
    expect(finalStates[0]).toBe(false) // loading is false
    expect(finalStates[2]).toBe('something went wrong') // error message captured
  })
})
