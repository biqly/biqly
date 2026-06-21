import { beforeEach, describe, expect, it, vi } from 'vitest'

import { useConfirmedMutation } from './useConfirmedMutation'

const confirmMock = vi.fn()
const toastMock = {
  success: vi.fn(),
  error: vi.fn(),
}

vi.mock('./useConfirm', () => ({
  useConfirm: () => confirmMock,
}))

vi.mock('./useToast', () => ({
  useToast: () => toastMock,
}))

vi.mock('react', () => ({
  useCallback: <T>(fn: T): T => fn,
}))

describe('useConfirmedMutation', () => {
  beforeEach(() => {
    vi.resetAllMocks()
  })

  it('skips execution if user rejects the confirmation dialog', async () => {
    confirmMock.mockResolvedValue(false)
    const mutationFn = vi.fn().mockResolvedValue('success')
    const runConfirmedMutation = useConfirmedMutation()

    const result = await runConfirmedMutation(mutationFn, {
      title: 'Delete Item',
      message: 'Are you sure?',
    })

    expect(confirmMock).toHaveBeenCalledWith({
      title: 'Delete Item',
      message: 'Are you sure?',
      confirmLabel: undefined,
      cancelLabel: undefined,
      variant: 'danger',
    })
    expect(mutationFn).not.toHaveBeenCalled()
    expect(result).toBe(false)
  })

  it('runs mutation and triggers success toast on approval', async () => {
    confirmMock.mockResolvedValue(true)
    const mutationFn = vi.fn().mockResolvedValue('success')
    const runConfirmedMutation = useConfirmedMutation()

    const result = await runConfirmedMutation(mutationFn, {
      title: 'Delete Item',
      successMessage: 'Deleted successfully',
    })

    expect(mutationFn).toHaveBeenCalled()
    expect(toastMock.success).toHaveBeenCalledWith('Deleted successfully')
    expect(result).toBe(true)
  })

  it('triggers error toast on mutation failure', async () => {
    confirmMock.mockResolvedValue(true)
    const mutationFn = vi.fn().mockRejectedValue(new Error('api failure'))
    const runConfirmedMutation = useConfirmedMutation()

    const result = await runConfirmedMutation(mutationFn, {
      title: 'Delete Item',
    })

    expect(mutationFn).toHaveBeenCalled()
    expect(toastMock.error).toHaveBeenCalledWith('api failure')
    expect(result).toBe(false)
  })
})
