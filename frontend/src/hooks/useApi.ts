import { useCallback, useRef, useState } from 'react'

import type { RequestOptions } from '../api/apiClient'
import { apiFetch } from '../api/apiClient'
import { resolveAdminApiKey } from '../utils/env'

export type Method = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'
type BodyMethod = 'postData' | 'putData' | 'patchData'

export async function request<T>(
  method: Method,
  url: string,
  body?: unknown,
  options?: RequestOptions,
): Promise<{ data: T | null; error: string | null }> {
  try {
    const data = await apiFetch<T>(method, url, body, options)
    return { data, error: null }
  } catch (err) {
    return { data: null, error: err instanceof Error ? err.message : 'Unknown error' }
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
    async <T = unknown>(
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
      if (err) {
        setError(err)
      }
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
    <T = unknown>(url: string, options?: RequestOptions) => call<T>('GET', url, undefined, options),
    [call],
  )
  const postData = useCallback(
    <T = unknown>(url: string, body: unknown, options?: RequestOptions) =>
      call<T>('POST', url, body, options),
    [call],
  )
  const patchData = useCallback(
    <T = unknown>(url: string, body: unknown, options?: RequestOptions) =>
      call<T>('PATCH', url, body, options),
    [call],
  )
  const putData = useCallback(
    <T = unknown>(url: string, body: unknown, options?: RequestOptions) =>
      call<T>('PUT', url, body, options),
    [call],
  )
  const deleteData = useCallback(
    <T = unknown>(url: string, options?: RequestOptions) =>
      call<T>('DELETE', url, undefined, options),
    [call],
  )

  return { get, postData, putData, patchData, deleteData, loading, error, abort }
}

function withAdminHeaders(options?: RequestOptions): RequestOptions {
  return { ...options, useAdminKey: true }
}

/**
 * useAdminApi is a convenience wrapper that automatically attaches the
 * BI_ADMIN_API_KEY as a Bearer token in the Authorization header.
 * All eval/history/regression endpoints require this header.
 */
export function useAdminApi() {
  const api = useApi()

  const bodyRequest = useCallback(
    <T = unknown>(method: BodyMethod, url: string, body: unknown, options?: RequestOptions) =>
      api[method]<T>(url, body, withAdminHeaders(options)),
    [api],
  )

  const get = useCallback(
    <T = unknown>(url: string, options?: RequestOptions) =>
      api.get<T>(url, withAdminHeaders(options)),
    [api],
  )
  const postData = useCallback(
    <T = unknown>(url: string, body: unknown, options?: RequestOptions) =>
      bodyRequest<T>('postData', url, body, options),
    [bodyRequest],
  )
  const putData = useCallback(
    <T = unknown>(url: string, body: unknown, options?: RequestOptions) =>
      bodyRequest<T>('putData', url, body, options),
    [bodyRequest],
  )
  const patchData = useCallback(
    <T = unknown>(url: string, body: unknown, options?: RequestOptions) =>
      bodyRequest<T>('patchData', url, body, options),
    [bodyRequest],
  )
  const deleteData = useCallback(
    <T = unknown>(url: string, options?: RequestOptions) =>
      api.deleteData<T>(url, withAdminHeaders(options)),
    [api],
  )

  const configured = resolveAdminApiKey().length > 0

  return { ...api, get, postData, putData, patchData, deleteData, configured }
}
