import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react'
import { ToastViewport } from '../components/ui/Toast'

export type ToastVariant = 'success' | 'error' | 'info' | 'warning'

export interface ToastOptions {
  title?: ReactNode
  message?: ReactNode
  variant?: ToastVariant
  /** Auto-dismiss delay in ms. Pass 0 to keep until dismissed. */
  duration?: number
}

export interface ToastItem extends ToastOptions {
  id: number
  variant: ToastVariant
}

export interface ToastApi {
  show: (options: ToastOptions) => number
  success: (message: ReactNode, options?: Omit<ToastOptions, 'variant' | 'message'>) => number
  error: (message: ReactNode, options?: Omit<ToastOptions, 'variant' | 'message'>) => number
  info: (message: ReactNode, options?: Omit<ToastOptions, 'variant' | 'message'>) => number
  warning: (message: ReactNode, options?: Omit<ToastOptions, 'variant' | 'message'>) => number
  dismiss: (id: number) => void
}

const DEFAULT_DURATION = 5000

const ToastContext = createContext<ToastApi | null>(null)

function useToastState(): { api: ToastApi; toasts: ToastItem[]; dismiss: (id: number) => void } {
  const [toasts, setToasts] = useState<ToastItem[]>([])
  const idRef = useRef(0)
  const timers = useRef(new Map<number, ReturnType<typeof setTimeout>>())

  const dismiss = useCallback((id: number) => {
    const timer = timers.current.get(id)
    if (timer) {
      clearTimeout(timer)
      timers.current.delete(id)
    }
    setToasts((prev) => prev.filter((t) => t.id !== id))
  }, [])

  const show = useCallback(
    (options: ToastOptions): number => {
      idRef.current += 1
      const id = idRef.current
      const item: ToastItem = { id, variant: options.variant ?? 'info', ...options }
      setToasts((prev) => [...prev, item])
      const duration = options.duration ?? DEFAULT_DURATION
      if (duration > 0) {
        timers.current.set(
          id,
          setTimeout(() => dismiss(id), duration),
        )
      }
      return id
    },
    [dismiss],
  )

  const api = useMemo<ToastApi>(() => {
    const variantHelper =
      (variant: ToastVariant) =>
      (message: ReactNode, options?: Omit<ToastOptions, 'variant' | 'message'>) =>
        show({ ...options, message, variant })
    return {
      show,
      success: variantHelper('success'),
      error: variantHelper('error'),
      info: variantHelper('info'),
      warning: variantHelper('warning'),
      dismiss,
    }
  }, [show, dismiss])

  const timersRef = timers
  useEffect(() => {
    const map = timersRef.current
    return () => {
      map.forEach((timer) => clearTimeout(timer))
      map.clear()
    }
  }, [timersRef])

  return { api, toasts, dismiss }
}

export function ToastProvider({ children }: { children: ReactNode }) {
  const { api, toasts, dismiss } = useToastState()
  return (
    <ToastContext.Provider value={api}>
      {children}
      <ToastViewport toasts={toasts} onDismiss={dismiss} />
    </ToastContext.Provider>
  )
}

export function useToast(): ToastApi {
  const api = useContext(ToastContext)
  if (!api) {
    throw new Error('useToast must be used within ToastProvider')
  }
  return api
}
