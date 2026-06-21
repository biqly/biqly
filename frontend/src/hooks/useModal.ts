import { useCallback, useState } from 'react'

export function useModal<T = void>() {
  const [open, setOpen] = useState(false)
  const [data, setData] = useState<T | null>(null)

  const openModal = useCallback((item?: T) => {
    setData(item ?? null)
    setOpen(true)
  }, [])

  const closeModal = useCallback(() => {
    setOpen(false)
    setData(null)
  }, [])

  return { open, data, openModal, closeModal }
}
