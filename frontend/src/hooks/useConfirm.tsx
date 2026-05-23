import { useCallback, useRef, useState } from 'react'
import { ConfirmDialog } from '../components/ui/ConfirmDialog'
import type { ReactNode } from 'react'

interface ConfirmOptions {
  title: ReactNode
  message?: ReactNode
  confirmLabel?: string
  cancelLabel?: string
  variant?: 'danger' | 'warning' | 'default'
}

interface PendingConfirm {
  options: ConfirmOptions
  resolve: (value: boolean) => void
}

export function useConfirm() {
  const [pending, setPending] = useState<PendingConfirm | null>(null)
  const resolveRef = useRef<((value: boolean) => void) | null>(null)

  const confirm = useCallback((options: ConfirmOptions): Promise<boolean> => {
    return new Promise<boolean>((resolve) => {
      resolveRef.current = resolve
      setPending({ options, resolve })
    })
  }, [])

  const handleConfirm = useCallback(() => {
    resolveRef.current?.(true)
    resolveRef.current = null
    setPending(null)
  }, [])

  const handleCancel = useCallback(() => {
    resolveRef.current?.(false)
    resolveRef.current = null
    setPending(null)
  }, [])

  const dialog = pending ? (
    <ConfirmDialog
      open
      title={pending.options.title}
      message={pending.options.message}
      confirmLabel={pending.options.confirmLabel}
      cancelLabel={pending.options.cancelLabel}
      variant={pending.options.variant}
      onConfirm={handleConfirm}
      onCancel={handleCancel}
    />
  ) : null

  return { confirm, dialog }
}
