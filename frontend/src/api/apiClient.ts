import { getLocale } from '../i18n'
import { plainTextFromHTML } from '../utils/plainText'
import { csrfFetch } from './csrf'

export interface RequestOptions extends Omit<RequestInit, 'body'> {
  timeout?: number
  token?: string
  useAdminKey?: boolean
  validate?: (data: unknown) => unknown
}

function stripRequestOptions(init?: RequestInit & RequestOptions): RequestInit {
  if (!init) {
    return {}
  }
  const {
    timeout: _timeout,
    token: _token,
    useAdminKey: _useAdminKey,
    validate: _validate,
    ...rest
  } = init
  return rest
}

// Module-level access token kept in sync by AuthProvider. With JWT enforcement
// on the backend (BI_AUTH_ENABLED=true) every /api request must carry a Bearer
// token, so fetchJSON falls back to this when a call site does not pass one
// explicitly.
let globalAccessToken: string | null = null

export function setGlobalAccessToken(token: string | null): void {
  globalAccessToken = token
}

function parseResponseBody(text: string): unknown {
  if (!text) {
    return null
  }
  try {
    return JSON.parse(text)
  } catch {
    return text
  }
}

function mergeAbortSignals(
  timeoutController: AbortController,
  external?: AbortSignal | null,
): AbortSignal {
  if (!external) {
    return timeoutController.signal
  }
  const merged = new AbortController()
  external.addEventListener('abort', () => merged.abort())
  timeoutController.signal.addEventListener('abort', () => merged.abort())
  if (external.aborted || timeoutController.signal.aborted) {
    merged.abort()
  }
  return merged.signal
}

function buildFetchHeaders(
  init: (RequestInit & RequestOptions) | undefined,
  body: BodyInit | null | undefined,
): Headers {
  const headers = new Headers(init?.headers)
  if (body && !headers.has('Content-Type')) {
    headers.set('Content-Type', 'application/json')
  }
  if (!headers.has('X-Locale')) {
    headers.set('X-Locale', getLocale())
  }
  const bearer = init?.token ?? globalAccessToken
  if (bearer) {
    headers.set('Authorization', `Bearer ${bearer}`)
  }
  return headers
}

function fetchNetworkErrorMessage(
  err: unknown,
  timedOut: boolean,
  startedAt: number,
  timeout: number,
): string {
  if (!(err instanceof Error)) {
    return 'Network error'
  }
  const aborted = err instanceof DOMException && err.name === 'AbortError'
  if (aborted || err.message.includes('aborted')) {
    if (timedOut || Date.now() - startedAt >= timeout) {
      return 'Request timed out'
    }
    return 'Request aborted'
  }
  return err.message
}

function responseError(status: number, data: unknown): string {
  if (data && typeof data === 'object') {
    const obj = data as Record<string, unknown>
    const err = obj.error ?? obj.message
    if (typeof err === 'string' && err.trim()) {
      return err
    }
  }
  if (typeof data === 'string') {
    const plain = plainTextFromHTML(data)
    return plain ? `HTTP ${status}: ${plain}` : `HTTP ${status}`
  }
  return `HTTP ${status}`
}

export interface FetchJSONResult<T> {
  data: T | null
  status: number
  error: string | null
}

// ApiError carries the HTTP status so callers can tell auth failures (401/403)
// apart from transient conditions (429, 5xx, network=0) instead of parsing
// the message string.
export class ApiError extends Error {
  readonly status: number

  constructor(message: string, status: number) {
    super(message)
    this.name = 'ApiError'
    this.status = status
  }
}

export async function fetchJSON<T>(
  url: string,
  init?: RequestInit & RequestOptions,
): Promise<FetchJSONResult<T>> {
  const method = init?.method ?? 'GET'
  const body = init?.body
  const timeout = init?.timeout ?? 30_000

  const controller = new AbortController()
  let timedOut = false
  const startedAt = Date.now()
  const timeoutId = setTimeout(() => {
    timedOut = true
    controller.abort()
  }, timeout)

  const signal = mergeAbortSignals(controller, init?.signal)

  try {
    const headers = buildFetchHeaders(init, body)

    const res = await csrfFetch(url, {
      ...stripRequestOptions(init),
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

    const parsed = init?.validate ? (init.validate(data) as T) : (data as T)
    return { data: parsed, status: res.status, error: null }
  } catch (err) {
    return {
      data: null,
      status: 0,
      error: fetchNetworkErrorMessage(err, timedOut, startedAt, timeout),
    }
  } finally {
    clearTimeout(timeoutId)
  }
}

export async function apiFetch<T>(
  method: string,
  url: string,
  body?: unknown,
  options: RequestOptions = {},
): Promise<T> {
  const init: RequestInit & RequestOptions = {
    ...options,
    method,
    body: body ? JSON.stringify(body) : undefined,
  }
  const { data, status, error } = await fetchJSON<T>(url, init)
  if (error) {
    throw new ApiError(error, status)
  }
  if (data === null) {
    if (status >= 200 && status < 300) {
      return null as T
    }
    throw new ApiError(`Expected response data from ${url}`, status)
  }
  return data
}
