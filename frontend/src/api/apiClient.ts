import { csrfFetch } from './csrf'
import { resolveAdminApiKey } from '../utils/env'
import { getLocale } from '../i18n'
import { plainTextFromHTML } from '../utils/plainText'

export interface RequestOptions extends Omit<RequestInit, 'body'> {
  timeout?: number
  token?: string
  useAdminKey?: boolean
}

function parseResponseBody(text: string): unknown {
  if (!text) return null
  try {
    return JSON.parse(text)
  } catch {
    return text
  }
}

function responseError(status: number, data: unknown): string {
  if (data && typeof data === 'object' && data !== null) {
    const obj = data as Record<string, unknown>
    const err = obj.error ?? obj.message
    if (typeof err === 'string' && err.trim()) return err
  }
  if (typeof data === 'string') {
    const plain = plainTextFromHTML(data)
    return plain ? `HTTP ${status}: ${plain}` : `HTTP ${status}`
  }
  return `HTTP ${status}`
}

export type FetchJSONResult<T> = { data: T | null; status: number; error: string | null }

export async function fetchJSON<T>(
  url: string,
  init?: RequestInit & RequestOptions
): Promise<FetchJSONResult<T>> {
  const method = init?.method ?? 'GET'
  const body = init?.body
  const timeout = init?.timeout ?? 30_000

  const controller = new AbortController()
  let didTimeout = false
  const timeoutId = setTimeout(
    () => {
      didTimeout = true
      controller.abort()
    },
    timeout,
  )

  const signal = init?.signal
    ? (() => {
        const merged = new AbortController()
        init.signal?.addEventListener('abort', () => merged.abort())
        controller.signal.addEventListener('abort', () => merged.abort())
        if (init.signal?.aborted || controller.signal.aborted) merged.abort()
        return merged.signal
      })()
    : controller.signal

  try {
    const headers = new Headers(init?.headers)
    if (body && !headers.has('Content-Type')) {
      headers.set('Content-Type', 'application/json')
    }
    if (!headers.has('X-Locale')) {
      headers.set('X-Locale', getLocale())
    }
    if (init?.token) {
      headers.set('Authorization', `Bearer ${init.token}`)
    } else if (init?.useAdminKey) {
      const adminKey = resolveAdminApiKey()
      if (adminKey) {
        headers.set('Authorization', `Bearer ${adminKey}`)
      }
    }

    const res = await csrfFetch(url, {
      ...init,
      method,
      headers,
      body,
      signal,
    })

    const text = await res.text()
    const data = parseResponseBody(text)

    if (!res.ok) {
      return { data: null, status: res.status, error: responseError(res.status, data) }
    }

    if (typeof data === 'string') {
      return { data: null, status: res.status, error: `Expected JSON response from ${url}` }
    }

    return { data: data as T, status: res.status, error: null }
  } catch (err) {
    let errMsg = 'Network error'
    if (err instanceof Error) {
      const aborted = err instanceof DOMException && err.name === 'AbortError'
      if (aborted || err.message.includes('aborted')) {
        errMsg = didTimeout ? 'Request timed out' : 'Request aborted'
      } else {
        errMsg = err.message
      }
    }
    return { data: null, status: 0, error: errMsg }
  } finally {
    clearTimeout(timeoutId)
  }
}

export async function apiFetch<T>(
  method: string,
  url: string,
  body?: unknown,
  options: RequestOptions = {}
): Promise<T> {
  const init: RequestInit & RequestOptions = {
    ...options,
    method,
    body: body ? JSON.stringify(body) : undefined,
  }
  const { data, error } = await fetchJSON<T>(url, init)
  if (error) {
    throw new Error(error)
  }
  if (data === null) {
    throw new Error(`Expected response data from ${url}`)
  }
  return data
}
