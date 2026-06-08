import { useContext, useEffect, useState } from 'react'

import { I18nContext } from './context'
import { FALLBACK_LOCALE, loadLocaleSection, type LocaleSectionName, sectionReady } from './locale'

export function useI18n() {
  const ctx = useContext(I18nContext)
  if (!ctx) {
    throw new Error('useI18n must be used within an <I18nProvider>')
  }
  return ctx
}

export function useT() {
  return useI18n().t
}

export function useLocale() {
  const { locale, setLocale } = useI18n()
  return [locale, setLocale] as const
}

export function useLocaleSection(section: LocaleSectionName): boolean {
  const { locale } = useI18n()
  const sectionKey = `${locale}:${section}`
  const syncReady = sectionReady(locale, section) && sectionReady(FALLBACK_LOCALE, section)
  const [loadState, setLoadState] = useState<{ key: string; ready: boolean }>({
    key: '',
    ready: false,
  })

  useEffect(() => {
    let cancelled = false
    void Promise.all([
      loadLocaleSection(locale, section),
      loadLocaleSection(FALLBACK_LOCALE, section),
    ]).then(() => {
      if (!cancelled) {
        setLoadState({ key: sectionKey, ready: true })
      }
    })
    return () => {
      cancelled = true
    }
  }, [locale, section, sectionKey])

  return syncReady || (loadState.key === sectionKey && loadState.ready)
}
