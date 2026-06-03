import { describe, expect, it, vi } from 'vitest'

import { createJobWaiter } from './jobWaiter'

describe('createJobWaiter', () => {
  it('resolves with the result on settleComplete and invokes onComplete once', async () => {
    const onComplete = vi.fn()
    const onError = vi.fn()
    const promise = new Promise<unknown | null>((resolve) => {
      const waiter = createJobWaiter(resolve, { onComplete, onError })
      waiter.settleComplete({ ok: true })
      waiter.settleComplete({ ok: false })
    })
    await expect(promise).resolves.toEqual({ ok: true })
    expect(onComplete).toHaveBeenCalledTimes(1)
    expect(onError).not.toHaveBeenCalled()
  })

  it('resolves null on settleError and invokes onError once', async () => {
    const onError = vi.fn()
    const promise = new Promise<unknown | null>((resolve) => {
      const waiter = createJobWaiter(resolve, { onError })
      waiter.settleError('failed')
      waiter.settleError('again')
    })
    await expect(promise).resolves.toBeNull()
    expect(onError).toHaveBeenCalledTimes(1)
    expect(onError).toHaveBeenCalledWith('failed')
  })

  it('resolves null on settleDismiss without invoking callbacks', async () => {
    const onComplete = vi.fn()
    const onError = vi.fn()
    const promise = new Promise<unknown | null>((resolve) => {
      const waiter = createJobWaiter(resolve, { onComplete, onError })
      waiter.settleDismiss()
      waiter.settleError('late')
    })
    await expect(promise).resolves.toBeNull()
    expect(onComplete).not.toHaveBeenCalled()
    expect(onError).not.toHaveBeenCalled()
  })
})
