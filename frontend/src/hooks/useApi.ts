import { useCallback, useRef, useState } from 'react'

import type { RequestOptions } from '../api/apiClient'
import { apiFetch } from '../api/apiClient'

export type Method = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'

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
 * useAdminApi routes admin/eval calls through the same JWT bearer as useApi
 * (super_admin or ai:settings). The legacy BI_ADMIN_API_KEY browser injection
 * path was removed — see tasks/05-devops-helm-config.md.
 */
export function useAdminApi() {
  const api = useApi()
  const {
    get: baseGet,
    postData: basePost,
    putData: basePut,
    patchData: basePatch,
    deleteData: baseDelete,
  } = api

  const get = useCallback(
    <T = unknown>(url: string, options?: RequestOptions) =>
      baseGet<T>(url, withAdminHeaders(options)),
    [baseGet],
  )
  const postData = useCallback(
    <T = unknown>(url: string, body: unknown, options?: RequestOptions) =>
      basePost<T>(url, body, withAdminHeaders(options)),
    [basePost],
  )
  const putData = useCallback(
    <T = unknown>(url: string, body: unknown, options?: RequestOptions) =>
      basePut<T>(url, body, withAdminHeaders(options)),
    [basePut],
  )
  const patchData = useCallback(
    <T = unknown>(url: string, body: unknown, options?: RequestOptions) =>
      basePatch<T>(url, body, withAdminHeaders(options)),
    [basePatch],
  )
  const deleteData = useCallback(
    <T = unknown>(url: string, options?: RequestOptions) =>
      baseDelete<T>(url, withAdminHeaders(options)),
    [baseDelete],
  )

  const configured = true

  return { ...api, get, postData, putData, patchData, deleteData, configured }
}
