import { describe, expect, it } from 'vitest'

import {
  acquireCornerId,
  cornerOwnerId,
  HANDOFF_WINDOW_MS,
  registerCorner,
  shouldShowImmediately,
  subscribeCorner,
  unregisterCorner,
} from './loadingScreenRegistry'

describe('loadingScreenRegistry', () => {
  it('makes the first-mounted instance the owner and hands over on unmount', () => {
    const a = acquireCornerId()
    const b = acquireCornerId()

    registerCorner(a)
    registerCorner(b)
    expect(cornerOwnerId()).toBe(a)

    unregisterCorner(a)
    expect(cornerOwnerId()).toBe(b)

    unregisterCorner(b)
    expect(cornerOwnerId()).toBe(0)
  })

  it('notifies subscribers on register/unregister', () => {
    let calls = 0
    const unsubscribe = subscribeCorner(() => {
      calls++
    })
    const id = acquireCornerId()
    registerCorner(id)
    unregisterCorner(id)
    unsubscribe()
    expect(calls).toBe(2)
  })

  it('skips the entry delay while another pill is active or just after it hid', () => {
    const id = acquireCornerId()
    registerCorner(id)
    expect(shouldShowImmediately()).toBe(true)

    unregisterCorner(id)
    // lastHiddenAt was just stamped — still within the handoff window.
    expect(shouldShowImmediately()).toBe(true)
    // Outside the window (simulated clock) the delay applies again.
    expect(shouldShowImmediately(Date.now() + HANDOFF_WINDOW_MS + 1)).toBe(false)
  })
})
