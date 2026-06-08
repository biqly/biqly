import type { TranslationKey } from '../../i18n/locale'

type SignInT = (key: TranslationKey) => string

export function sessionExpiredBanner(
  reason: string | null,
  t: SignInT,
): { title: string; body: string } | null {
  if (!reason) {
    return null
  }

  const titleKey =
    reason === 'idle'
      ? 'session_expired_title_idle'
      : reason === 'absolute'
        ? 'session_expired_title_absolute'
        : reason === 'revoked'
          ? 'session_expired_title_revoked'
          : 'session_expired_title_generic'

  const bodyKey =
    reason === 'idle'
      ? 'session_expired_idle'
      : reason === 'absolute'
        ? 'session_expired_absolute'
        : reason === 'revoked'
          ? 'session_expired_revoked'
          : 'session_expired_generic'

  return {
    title: t(`auth.${titleKey}`),
    body: t(`auth.${bodyKey}`),
  }
}
