import type { TranslationKey } from '../i18n'

/** Safe error-to-string: returns `Error.message` for Error instances, otherwise `String(value)`. */
export function errorMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e)
}

// Structural check for apiClient's ApiError. Kept duck-typed (instead of
// importing the class) so this module stays runtime-dependency-free: several
// pure hook tests mock 'react' and must not transitively load the i18n
// module graph that apiClient pulls in.
function errorStatus(e: unknown): number | null {
  if (
    e instanceof Error &&
    'status' in e &&
    typeof (e as { status: unknown }).status === 'number'
  ) {
    return (e as { status: number }).status
  }
  return null
}

/**
 * User-facing error text: maps transport-level failures (rate limit, server
 * error, network) to translated messages instead of leaking raw API strings
 * like "too_many_requests" or "internal server error" into banners and toasts.
 */
export function friendlyErrorMessage(t: (key: TranslationKey) => string, e: unknown): string {
  const status = errorStatus(e)
  if (status !== null) {
    if (status === 429) {
      return t('common.error_too_many_requests')
    }
    if (status >= 500) {
      return t('common.error_server')
    }
    if (status === 0) {
      return t('common.error_network')
    }
  }
  const msg = errorMessage(e)
  return msg.trim() ? msg : t('common.unknown_error')
}
