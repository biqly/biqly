import { useCallback, useState } from 'react'

export function useArrayState<T>(initial: T[] = []) {
  const [items, setItems] = useState<T[]>(initial)

  const add = useCallback((item: T) => {
    setItems((prev) => [...prev, item])
  }, [])

  const update = useCallback((index: number, updater: T | ((item: T | undefined) => T)) => {
    setItems((prev) =>
      prev.map((item, i) => {
        if (i !== index) {
          return item
        }
        return typeof updater === 'function'
          ? (updater as (item: T | undefined) => T)(item)
          : updater
      }),
    )
  }, [])

  const remove = useCallback((index: number) => {
    setItems((prev) => prev.filter((_, i) => i !== index))
  }, [])

  return { items, setItems, add, update, remove }
}
