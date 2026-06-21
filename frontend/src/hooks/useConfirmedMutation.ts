import { useCallback } from 'react'

import { errorMessage } from '../utils/error'
import { type ConfirmOptions, useConfirm } from './useConfirm'
import { useToast } from './useToast'

export interface ConfirmedMutationOptions extends ConfirmOptions {
  successMessage?: string
  errorMessage?: string
}

export function useConfirmedMutation() {
  const confirm = useConfirm()
  const toast = useToast()

  const runConfirmedMutation = useCallback(
    async (
      mutationFn: () => Promise<unknown>,
      options: ConfirmedMutationOptions,
    ): Promise<boolean> => {
      const ok = await confirm({
        title: options.title,
        message: options.message,
        confirmLabel: options.confirmLabel,
        cancelLabel: options.cancelLabel,
        variant: options.variant ?? 'danger',
      })

      if (!ok) {
        return false
      }

      try {
        await mutationFn()
        if (options.successMessage) {
          toast.success(options.successMessage)
        }
        return true
      } catch (err) {
        const fallbackErr = options.errorMessage ?? errorMessage(err)
        toast.error(fallbackErr)
        return false
      }
    },
    [confirm, toast],
  )

  return runConfirmedMutation
}
