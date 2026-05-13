import { useCallback, useEffect, useRef, useState } from 'react'

interface UseStreamingApiOptions {
  /** Delay in ms between each typed character (0 = instant) */
  typingSpeed?: number
  /** Maximum time to wait before aborting (ms) */
  timeout?: number
}

interface UseStreamingApiResult {
  /** Accumulated streamed text (with typing effect applied) */
  data: string | null
  /** True while a request is in-flight */
  loading: boolean
  /** Error message, if any */
  error: string | null
  /** Abort the current stream */
  abort: () => void
  /** Start a streaming request */
  start: (url: string, body?: unknown, headers?: Record<string, string>) => void
}

/**
 * Custom hook that handles SSE (Server-Sent Events) for streaming AI responses.
 *
 * - Tries `EventSource` first for GET-style endpoints.
 * - Falls back to `fetch` with a ReadableStream for POST bodies.
 * - If neither SSE nor streaming fetch works, falls back to a regular POST
 *   that resolves the entire body at once.
 * - Applies a typing effect so characters appear one-by-one.
 */
export default function useStreamingApi(
  options?: UseStreamingApiOptions,
): UseStreamingApiResult {
  const { typingSpeed = 8, timeout = 30_000 } = options ?? {}

  const [data, setData] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const abortRef = useRef<AbortController | null>(null)
  const typingTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const fullBufferRef = useRef('')
  const displayedRef = useRef('')
  const typingIndexRef = useRef(0)

  // Clear typing timer on unmount / abort
  const clearTyping = useCallback(() => {
    if (typingTimerRef.current) {
      clearTimeout(typingTimerRef.current)
      typingTimerRef.current = null
    }
  }, [])

  const abort = useCallback(() => {
    abortRef.current?.abort()
    abortRef.current = null
    clearTyping()
    setLoading(false)
  }, [clearTyping])

  // Typing effect loop
  const scheduleNextChar = useCallback(() => {
    const idx = typingIndexRef.current
    const full = fullBufferRef.current
    if (idx >= full.length) return // done

    // Advance one character (handle surrogate pairs)
    const next = idx + ((full.codePointAt(idx) ?? 0) > 0xffff ? 2 : 1)
    displayedRef.current = full.slice(0, Math.min(next, full.length))
    typingIndexRef.current = next
    setData(displayedRef.current)

    if (next < full.length) {
      typingTimerRef.current = setTimeout(scheduleNextChar, typingSpeed)
    }
  }, [typingSpeed])

  const startTyping = useCallback(
    (text: string) => {
      clearTyping()
      fullBufferRef.current = text
      displayedRef.current = ''
      typingIndexRef.current = 0
      if (text.length === 0) {
        setData(null)
        return
      }
      scheduleNextChar()
    },
    [clearTyping, scheduleNextChar],
  )

  // Expose abort via useEffect cleanup
  useEffect(() => () => abort(), [abort])

  /**
   * Stream from a URL using EventSource (GET only).
   * Returns true if EventSource was available and used.
   */
  const streamWithEventSource = useCallback(
    (url: string): boolean => {
      if (typeof EventSource === 'undefined') return false

      const es = new EventSource(url)
      let accumulated = ''

      es.onmessage = (event: MessageEvent) => {
        if (event.data === '[DONE]') {
          // Push remaining buffer instantly
          fullBufferRef.current = accumulated
          startTyping(accumulated)
          es.close()
          setLoading(false)
          return
        }
        accumulated += event.data
        fullBufferRef.current = accumulated
        // Only start typing once; subsequent chunks extend the buffer
        if (typingIndexRef.current === 0) {
          startTyping(accumulated)
        }
      }

      es.onerror = () => {
        es.close()
        // If nothing received, signal fallback
        if (accumulated.length === 0) return
        setLoading(false)
      }

      return true
    },
    [startTyping],
  )

  /**
   * Stream from a URL using fetch + ReadableStream (supports POST).
   * Returns true if the response is actually streaming (Transfer-Encoding: chunked).
   */
  const streamWithFetch = useCallback(
    async (url: string, init?: RequestInit) => {
      const controller = new AbortController()
      abortRef.current = controller

      const timeoutId = setTimeout(() => controller.abort(), timeout)
      setLoading(true)
      setError(null)
      setData(null)
      fullBufferRef.current = ''
      displayedRef.current = ''
      typingIndexRef.current = 0

      try {
        const response = await fetch(url, {
          ...init,
          signal: controller.signal,
        })

        if (!response.ok) {
          const text = await response.text()
          throw new Error(text || `HTTP ${response.status}`)
        }

        const reader = response.body?.getReader()
        if (!reader) throw new Error('ReadableStream not supported')

        const decoder = new TextDecoder()
        let accumulated = ''

        // eslint-disable-next-line no-constant-condition
        while (true) {
          const { done, value } = await reader.read()
          if (done) break

          const chunk = decoder.decode(value, { stream: true })
          accumulated += chunk
          fullBufferRef.current = accumulated

          if (typingIndexRef.current === 0) {
            startTyping(accumulated)
          }
        }

        // Finalize: push full buffer through typing if not already done
        fullBufferRef.current = accumulated
        if (typingIndexRef.current < accumulated.length) {
          // Speed up remaining to finish quickly
          const prev = typingSpeed
          // We just set the full data now
          setData(accumulated)
          typingIndexRef.current = accumulated.length
          displayedRef.current = accumulated
          void prev // unused but keeps typingSpeed dependency stable
        }
      } catch (err: unknown) {
        if (err instanceof DOMException && err.name === 'AbortError') {
          setError('Request aborted')
        } else {
          setError(err instanceof Error ? err.message : 'Stream failed')
        }
      } finally {
        clearTimeout(timeoutId)
        setLoading(false)
      }
    },
    [startTyping, typingSpeed, timeout],
  )

  /**
   * Fallback: regular POST that returns the full body at once.
   */
  const fallbackPost = useCallback(
    async (url: string, body: unknown) => {
      const controller = new AbortController()
      abortRef.current = controller

      const timeoutId = setTimeout(() => controller.abort(), timeout)
      setLoading(true)
      setError(null)
      setData(null)

      try {
        const response = await fetch(url, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(body),
          signal: controller.signal,
        })

        if (!response.ok) {
          const text = await response.text()
          throw new Error(text || `HTTP ${response.status}`)
        }

        const text = await response.text()
        startTyping(text)
        // Also set full data immediately for safety
        setTimeout(() => {
          setData(text)
          typingIndexRef.current = text.length
          displayedRef.current = text
          fullBufferRef.current = text
        }, 50)
      } catch (err: unknown) {
        if (err instanceof DOMException && err.name === 'AbortError') {
          setError('Request aborted')
        } else {
          setError(err instanceof Error ? err.message : 'Request failed')
        }
      } finally {
        clearTimeout(timeoutId)
        setLoading(false)
      }
    },
    [startTyping, timeout],
  )

  /**
   * Public API: start a streaming request.
   *
   * @param url   Endpoint to hit.
   * @param body  Optional POST body. When provided, uses fetch fallback
   *              (EventSource cannot send POST).
   */
  const start = useCallback(
    (url: string, body?: unknown, headers?: Record<string, string>) => {
      abort() // cancel any in-flight
      const hasHeaders = headers && Object.keys(headers).length > 0

      if (body === undefined && !hasHeaders) {
        // GET without custom headers — try EventSource first
        if (streamWithEventSource(url)) return
      }

      // POST or no EventSource → try fetch streaming
      if (body !== undefined) {
        void streamWithFetch(url, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json', ...headers },
          body: JSON.stringify(body),
        })
      } else {
        // GET without EventSource → fetch streaming
        void streamWithFetch(url, { method: 'GET', headers })
      }
    },
    [abort, streamWithEventSource, streamWithFetch],
  )

  return { data, loading, error, abort, start }
}
