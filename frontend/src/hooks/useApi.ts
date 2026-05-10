import { useState, useCallback, useRef } from 'react'

type Method = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'

interface RequestOptions {
  timeout?: number
  signal?: AbortSignal
}

async function request<T>(
  method: Method,
  url: string,
  body?: unknown,
  options?: RequestOptions,
): Promise<{ data: T | null; error: string | null }> {
  const controller = new AbortController()
  const timeoutId = setTimeout(
    () => controller.abort(),
    options?.timeout ?? 30_000,
  )

  const signal = options?.signal
    ? ((() => {
        const merged = new AbortController()
        options?.signal?.addEventListener('abort', () => merged.abort())
        controller.signal.addEventListener('abort', () => merged.abort())
        return merged.signal
      })())
    : controller.signal

  try {
    const res = await fetch(url, {
      method,
      headers: body ? { 'Content-Type': 'application/json' } : undefined,
      body: body ? JSON.stringify(body) : undefined,
      signal,
    })
    const text = await res.text()
    const data = text ? JSON.parse(text) : null
    if (!res.ok) {
      return { data: null, error: data?.error || `HTTP ${res.status}` }
    }
    return { data: data as T, error: null }
  } catch (err) {
    const message = err instanceof Error ? err.message : 'Network error'
    return {
      data: null,
      error: message.includes('aborted') ? 'Request timed out' : message,
    }
  } finally {
    clearTimeout(timeoutId)
  }
}

export function useApi() {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const controllerRef = useRef<AbortController | null>(null)
  const inFlightRef = useRef(0)

  const abort = useCallback(() => {
    controllerRef.current?.abort()
    controllerRef.current = null
  }, [])

  const call = useCallback(
    async <T = any>(
      method: Method,
      url: string,
      body?: unknown,
      options?: RequestOptions,
    ): Promise<T | null> => {
      inFlightRef.current++
      setLoading(true)
      setError(null)
      const controller = new AbortController()
      controllerRef.current = controller
      const mergedSignal = { ...options, signal: controller.signal }
      const { data, error: err } = await request<T>(method, url, body, mergedSignal)
      if (err) setError(err)
      inFlightRef.current--
      if (inFlightRef.current === 0) {
        setLoading(false)
        controllerRef.current = null
      }
      return data
    },
    [],
  )

  const get = useCallback(
    <T = any>(url: string, options?: RequestOptions) => call<T>('GET', url, undefined, options),
    [call],
  )
  const postData = useCallback(
    <T = any>(url: string, body: unknown, options?: RequestOptions) =>
      call<T>('POST', url, body, options),
    [call],
  )
  const patchData = useCallback(
    <T = any>(url: string, body: unknown, options?: RequestOptions) =>
      call<T>('PATCH', url, body, options),
    [call],
  )
  const putData = useCallback(
    <T = any>(url: string, body: unknown, options?: RequestOptions) =>
      call<T>('PUT', url, body, options),
    [call],
  )
  const deleteData = useCallback(
    <T = any>(url: string, options?: RequestOptions) => call<T>('DELETE', url, undefined, options),
    [call],
  )

  return { get, postData, putData, patchData, deleteData, loading, error, abort }
}
