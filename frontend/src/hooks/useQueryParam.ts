import { useCallback, useSyncExternalStore } from 'react'

/**
 * Lightweight URL search-param state. Reads the current value for `key`
 * from `window.location.search` and writes back via `history.replaceState`
 * (so dropdown changes don't pollute the history stack). Cross-component
 * consumers stay in sync through `popstate` and a custom `urlchange` event.
 *
 * Empty/null values are stripped from the URL so the address bar stays tidy.
 */
const URL_CHANGE_EVENT = 'biqly:urlchange'

function subscribe(callback: () => void) {
  window.addEventListener('popstate', callback)
  window.addEventListener(URL_CHANGE_EVENT, callback)
  return () => {
    window.removeEventListener('popstate', callback)
    window.removeEventListener(URL_CHANGE_EVENT, callback)
  }
}

function getSnapshot(): string {
  return window.location.search
}

function getServerSnapshot(): string {
  return ''
}

/** Subscribes to URL search changes from useQueryParam and browser navigation. */
export function useUrlSearch(): string {
  return useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot)
}

export function useQueryParam(key: string): [string, (next: string) => void] {
  const search = useSyncExternalStore(subscribe, getSnapshot, getServerSnapshot)
  const value = new URLSearchParams(search).get(key) ?? ''

  const setValue = useCallback(
    (next: string) => {
      const params = new URLSearchParams(window.location.search)
      const current = params.get(key) ?? ''
      if (current === next) {
        return
      }
      if (next === '') {
        params.delete(key)
      } else {
        params.set(key, next)
      }
      const qs = params.toString()
      const url = `${window.location.pathname}${qs ? `?${qs}` : ''}${window.location.hash}`
      window.history.replaceState(window.history.state, '', url)
      window.dispatchEvent(new Event(URL_CHANGE_EVENT))
    },
    [key],
  )

  return [value, setValue]
}
