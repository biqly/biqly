import { useEffect, useRef } from 'react'

export function useAutofocus<T extends HTMLElement>(enabled = true) {
  const ref = useRef<T>(null)
  useEffect(() => {
    if (enabled) {
      ref.current?.focus()
    }
  }, [enabled])
  return ref
}
