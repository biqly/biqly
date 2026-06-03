const CSRF_COOKIE_NAME = 'csrf_token'
const AUTH_CSRF_BOOTSTRAP_PATH = '/api/auth/me'
const SAFE_METHODS = new Set(['GET', 'HEAD', 'OPTIONS', 'TRACE'])

function readCookie(name: string): string | null {
  const prefix = `${name}=`
  const cookie = document.cookie
    .split(';')
    .map((part) => part.trim())
    .find((part) => part.startsWith(prefix))

  return cookie ? decodeURIComponent(cookie.slice(prefix.length)) : null
}

async function ensureCSRFToken(): Promise<string> {
  let token = readCookie(CSRF_COOKIE_NAME)
  if (token) {
    return token
  }

  await fetch(AUTH_CSRF_BOOTSTRAP_PATH, {
    method: 'GET',
    credentials: 'same-origin',
  })

  token = readCookie(CSRF_COOKIE_NAME)
  if (!token) {
    throw new Error('Unable to initialize CSRF token')
  }

  return token
}

function mergeHeaders(headers: HeadersInit | undefined, csrfToken: string): Headers {
  const merged = new Headers(headers)
  merged.set('X-CSRF-Token', csrfToken)
  return merged
}

export async function csrfFetch(
  input: RequestInfo | URL,
  init: RequestInit = {},
): Promise<Response> {
  const method = (init.method || 'GET').toUpperCase()
  if (SAFE_METHODS.has(method)) {
    return fetch(input, { ...init, credentials: init.credentials ?? 'same-origin' })
  }

  const token = await ensureCSRFToken()
  return fetch(input, {
    ...init,
    credentials: init.credentials ?? 'same-origin',
    headers: mergeHeaders(init.headers, token),
  })
}
