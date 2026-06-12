import { useEffect, useState } from 'react'

/**
 * Debounced mirror of a value (Faz 4.3). Replaces the per-screen
 * setTimeout/clearTimeout effects around search inputs; 300ms is the
 * existing convention (UserListPage, QueryHistory).
 */
export function useDebouncedValue<T>(value: T, delayMs: number): T {
  const [debounced, setDebounced] = useState(value)

  useEffect(() => {
    const id = window.setTimeout(() => setDebounced(value), delayMs)
    return () => window.clearTimeout(id)
  }, [value, delayMs])

  return debounced
}
