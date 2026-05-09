import { useState, useCallback } from 'react'

type Method = 'GET' | 'POST' | 'PUT' | 'PATCH' | 'DELETE'

async function request<T>(method: Method, url: string, body?: unknown): Promise<{ data: T | null; error: string | null }> {
  try {
    const res = await fetch(url, {
      method,
      headers: body ? { 'Content-Type': 'application/json' } : undefined,
      body: body ? JSON.stringify(body) : undefined,
    })
    const text = await res.text()
    const data = text ? JSON.parse(text) : null
    if (!res.ok) {
      return { data: null, error: data?.error || `HTTP ${res.status}` }
    }
    return { data: data as T, error: null }
  } catch (err) {
    return { data: null, error: err instanceof Error ? err.message : 'Network error' }
  }
}

export function useApi() {
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const call = useCallback(async <T = any>(method: Method, url: string, body?: unknown): Promise<T | null> => {
    setLoading(true)
    setError(null)
    const { data, error: err } = await request<T>(method, url, body)
    if (err) setError(err)
    setLoading(false)
    return data
  }, [])

  const get = useCallback(<T = any>(url: string) => call<T>('GET', url), [call])
  const postData = useCallback(<T = any>(url: string, body: unknown) => call<T>('POST', url, body), [call])
  const patchData = useCallback(<T = any>(url: string, body: unknown) => call<T>('PATCH', url, body), [call])
  const deleteData = useCallback(<T = any>(url: string) => call<T>('DELETE', url), [call])

  return { get, postData, patchData, deleteData, loading, error }
}
