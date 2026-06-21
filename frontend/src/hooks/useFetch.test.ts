import * as React from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useFetch } from './useFetch'

interface MockReact {
  __resetHooks: () => void
  __getStates: () => unknown[]
  __getEffects: () => { fn: () => void | (() => void); deps: unknown[] }[]
}

vi.mock('react', () => {
  let states: unknown[] = []
  let index = 0
  const setters: ((newVal: unknown) => void)[] = []
  let effects: { fn: () => void | (() => void); deps: unknown[] }[] = []

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

  const useEffectMock = (fn: () => void | (() => void), deps: unknown[]) => {
    effects.push({ fn, deps })
  }

  return {
    useState: useStateMock,
    useEffect: useEffectMock,
    useCallback: <T>(fn: T): T => fn,
    __resetHooks: () => {
      states = []
      index = 0
      effects = []
    },
    __getStates: () => states,
    __getEffects: () => effects,
  }
})

const mockReact = React as unknown as MockReact

describe('useFetch', () => {
  beforeEach(() => {
    mockReact.__resetHooks()
  })

  it('runs initial fetch when enabled', async () => {
    const fetcher = (signal: AbortSignal) => {
      expect(signal).toBeInstanceOf(AbortSignal)
      return Promise.resolve('data-loaded')
    }

    useFetch(fetcher)

    const effects = mockReact.__getEffects()
    expect(effects).toHaveLength(1)

    // Trigger the effect manually
    const cleanup = effects[0]?.fn()

    // Allow async promise execution to complete
    await new Promise((resolve) => {
      setTimeout(resolve, 0)
    })

    const states = mockReact.__getStates()
    expect(states[0]).toBe('data-loaded') // data set
    expect(states[1]).toBe(false) // loading is false
    expect(states[2]).toBeNull() // error is null

    if (cleanup) {
      cleanup()
    }
  })

  it('does not run fetch and resets state when disabled', () => {
    const fetcher = () => Promise.resolve('data')
    useFetch(fetcher, [], { enabled: false })

    const effects = mockReact.__getEffects()
    expect(effects).toHaveLength(1)

    // Run the effect
    const cleanup = effects[0]?.fn()

    const states = mockReact.__getStates()
    expect(states[0]).toBeNull() // data is null
    expect(states[1]).toBe(false) // loading is false
    expect(states[2]).toBeNull() // error is null

    if (cleanup) {
      cleanup()
    }
  })

  it('captures error messages on failure', async () => {
    const fetcher = () => Promise.reject(new Error('fetch error'))

    useFetch(fetcher)

    const effects = mockReact.__getEffects()
    const cleanup = effects[0]?.fn()

    await new Promise((resolve) => {
      setTimeout(resolve, 0)
    })

    const states = mockReact.__getStates()
    expect(states[0]).toBeNull() // data is null
    expect(states[1]).toBe(false) // loading is false
    expect(states[2]).toBe('fetch error') // error captured

    if (cleanup) {
      cleanup()
    }
  })
})
