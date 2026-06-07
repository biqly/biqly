const CSRF_HEADER_NAME = 'X-CSRF-Token'
const AUTH_CSRF_BOOTSTRAP_PATH = '/api/auth/csrf'
const SAFE_METHODS = new Set(['GET', 'HEAD', 'OPTIONS', 'TRACE'])

let cachedCSRFToken: string | null = null

function captureCSRFTokenFromResponse(response: Response): void {
  const token = response.headers.get(CSRF_HEADER_NAME)
  if (token) {
    cachedCSRFToken = token
  }
}

async function ensureCSRFToken(): Promise<string> {
  if (cachedCSRFToken) {
    return cachedCSRFToken
  }

  const response = await fetch(AUTH_CSRF_BOOTSTRAP_PATH, {
    method: 'GET',
    credentials: 'same-origin',
  })
  captureCSRFTokenFromResponse(response)

  if (!cachedCSRFToken) {
    throw new Error('Unable to initialize CSRF token')
  }

  return cachedCSRFToken
}

function mergeHeaders(headers: HeadersInit | undefined, csrfToken: string): Headers {
  const merged = new Headers(headers)
  merged.set(CSRF_HEADER_NAME, csrfToken)
  return merged
}

export async function csrfFetch(
  input: RequestInfo | URL,
  init: RequestInit = {},
): Promise<Response> {
  const method = (init.method || 'GET').toUpperCase()
  if (SAFE_METHODS.has(method)) {
    const response = await fetch(input, { ...init, credentials: init.credentials ?? 'same-origin' })
    captureCSRFTokenFromResponse(response)
    return response
  }

  const token = await ensureCSRFToken()
  return fetch(input, {
    ...init,
    credentials: init.credentials ?? 'same-origin',
    headers: mergeHeaders(init.headers, token),
  })
}
