import type { Dictionary, LocaleSectionName } from './locales/dictionary'
import { core as enCore } from './locales/en/core'
import { core as trCore } from './locales/tr/core'

export type { Dictionary, LocaleSectionName } from './locales/dictionary'

const STORAGE_KEY = 'biqly_locale'

export type Locale = 'en' | 'tr'
export const SUPPORTED_LOCALES: Locale[] = ['en', 'tr']
export const DEFAULT_LOCALE: Locale = 'tr'
export const FALLBACK_LOCALE: Locale = 'en'
export const LOCALE_OPTIONS: Record<Locale, { label: string; short: string; languageTag: string }> =
  {
    en: { label: 'English', short: 'EN', languageTag: 'en-US' },
    tr: { label: 'Türkçe', short: 'TR', languageTag: 'tr-TR' },
  }

export function localeLanguageTag(locale: Locale): string {
  return LOCALE_OPTIONS[locale].languageTag
}

type LeafKeys<T, Prefix extends string = ''> = {
  [K in keyof T & string]: T[K] extends string
    ? `${Prefix}${K}`
    : T[K] extends Record<string, unknown>
      ? LeafKeys<T[K], `${Prefix}${K}.`>
      : never
}[keyof T & string]

export type TranslationKey = LeafKeys<Dictionary>

type PartialDict = Record<string, unknown>

const dictionaries: Record<Locale, PartialDict> = {
  en: { ...enCore },
  tr: { ...trCore },
}

const sectionLoaders: Record<Locale, Record<LocaleSectionName, () => Promise<PartialDict>>> = {
  en: {
    admin: () => import('./locales/en/admin').then((m) => ({ admin: m.admin })),
    auth: () => import('./locales/en/auth').then((m) => ({ auth: m.auth })),
  },
  tr: {
    admin: () => import('./locales/tr/admin').then((m) => ({ admin: m.admin })),
    auth: () => import('./locales/tr/auth').then((m) => ({ auth: m.auth })),
  },
}

const loadedSections = new Set<string>()
const inFlight = new Map<string, Promise<void>>()
const listeners = new Set<() => void>()

function notify() {
  listeners.forEach((l) => l())
}

export function subscribeLocaleChanges(listener: () => void): () => void {
  listeners.add(listener)
  return () => {
    listeners.delete(listener)
  }
}

export function loadLocaleSection(locale: Locale, section: LocaleSectionName): Promise<void> {
  const key = `${locale}:${section}`
  if (loadedSections.has(key)) {
    return Promise.resolve()
  }
  const existing = inFlight.get(key)
  if (existing) {
    return existing
  }
  const p = sectionLoaders[locale][section]()
    .then((mod) => {
      dictionaries[locale] = { ...dictionaries[locale], ...mod }
      loadedSections.add(key)
      inFlight.delete(key)
      notify()
    })
    .catch((err) => {
      inFlight.delete(key)
      throw err
    })
  inFlight.set(key, p)
  return p
}

export function sectionReady(locale: Locale, section: LocaleSectionName): boolean {
  return loadedSections.has(`${locale}:${section}`)
}

function readLocaleFromStorage(): Locale | null {
  if (typeof window === 'undefined') {
    return null
  }
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY)
    if (raw && (SUPPORTED_LOCALES as string[]).includes(raw)) {
      return raw as Locale
    }
  } catch {
    /* ignore */
  }
  return null
}

function detectBrowserLocale(): Locale {
  if (typeof navigator === 'undefined') {
    return DEFAULT_LOCALE
  }
  const language = typeof navigator.language === 'string' ? navigator.language : ''
  const languages = Array.isArray(navigator.languages)
    ? navigator.languages.filter((lang): lang is string => typeof lang === 'string')
    : []
  const candidates: string[] = [language, ...languages]
  for (const raw of candidates) {
    if (!raw) {
      continue
    }
    const base = raw.toLowerCase().split(/[-_]/)[0] as Locale
    if ((SUPPORTED_LOCALES as string[]).includes(base)) {
      return base
    }
  }
  return DEFAULT_LOCALE
}

let currentLocale: Locale = readLocaleFromStorage() ?? detectBrowserLocale()

export function applyLocaleSideEffects(locale: Locale) {
  currentLocale = locale
  if (typeof document !== 'undefined') {
    document.documentElement.lang = locale
  }
  try {
    if (typeof window !== 'undefined') {
      window.localStorage.setItem(STORAGE_KEY, locale)
    }
  } catch {
    /* ignore */
  }
}

applyLocaleSideEffects(currentLocale)

export function getLocale(): Locale {
  return currentLocale
}

export function lookup(dict: PartialDict, key: string): string | undefined {
  const parts = key.split('.')
  let cur: unknown = dict
  for (const p of parts) {
    if (cur && typeof cur === 'object' && p in (cur as Record<string, unknown>)) {
      cur = (cur as Record<string, unknown>)[p]
    } else {
      return undefined
    }
  }
  return typeof cur === 'string' ? cur : undefined
}

function interpolate(template: string, params?: Record<string, string | number>): string {
  if (!params || !template.includes('{{')) {
    return template
  }
  return template.replace(/\{\{\s*([\w]+)\s*\}\}/g, (_, key: string) => {
    const v = params[key]
    return v === undefined ? `{{${key}}}` : String(v)
  })
}

export function translate(
  locale: Locale,
  key: TranslationKey,
  params?: Record<string, string | number>,
): string {
  const primary = lookup(dictionaries[locale], key)
  if (primary !== undefined) {
    return interpolate(primary, params)
  }
  if (locale !== FALLBACK_LOCALE) {
    const fallback = lookup(dictionaries[FALLBACK_LOCALE], key)
    if (fallback !== undefined) {
      return interpolate(fallback, params)
    }
  }
  return key
}

export interface I18nContextValue {
  locale: Locale
  setLocale: (loc: Locale) => void
  t: (key: TranslationKey, params?: Record<string, string | number>) => string
  supported: readonly Locale[]
}

export type TFunction = I18nContextValue['t']

export type LooseTFunction = (key: string, params?: Record<string, string | number>) => string
