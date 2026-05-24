import {
  createContext,
  useCallback,
  useContext,
  useRef,
  useState,
  type ReactNode,
} from 'react'
import { ConfirmDialog } from '../components/ui/ConfirmDialog'

export interface ConfirmOptions {
  title: ReactNode
  message?: ReactNode
  confirmLabel?: string
  cancelLabel?: string
  variant?: 'danger' | 'warning' | 'default'
}

type ConfirmFn = (options: ConfirmOptions) => Promise<boolean>

interface PendingConfirm {
  options: ConfirmOptions
  resolve: (value: boolean) => void
}

const ConfirmContext = createContext<ConfirmFn | null>(null)

function useConfirmState() {
  const [pending, setPending] = useState<PendingConfirm | null>(null)
  const resolveRef = useRef<((value: boolean) => void) | null>(null)

  const confirm = useCallback((options: ConfirmOptions): Promise<boolean> => {
    return new Promise<boolean>((resolve) => {
      resolveRef.current = resolve
      setPending({ options, resolve })
    })
  }, [])

  const settle = useCallback((value: boolean) => {
    resolveRef.current?.(value)
    resolveRef.current = null
    setPending(null)
  }, [])

  const handleConfirm = useCallback(() => settle(true), [settle])
  const handleCancel = useCallback(() => settle(false), [settle])

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

export function ConfirmProvider({ children }: { children: ReactNode }) {
  const { confirm, dialog } = useConfirmState()
  return (
    <ConfirmContext.Provider value={confirm}>
      {children}
      {dialog}
    </ConfirmContext.Provider>
  )
}

export function useConfirm(): ConfirmFn {
  const confirm = useContext(ConfirmContext)
  if (!confirm) {
    throw new Error('useConfirm must be used within ConfirmProvider')
  }
  return confirm
}
