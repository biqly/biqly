// Full dictionary type assembled from the locale split.
//
// These are TYPE-ONLY imports: they are erased at compile time and create no
// runtime dependency, so `admin`/`auth` stay in their own lazily-loaded chunks
// while `TranslationKey` keeps full compile-time coverage of every key.
import type { core } from './en/core'
import type { admin } from './en/admin'
import type { auth } from './en/auth'

/** Widens string-literal values to `string` so other locales can differ. */
export type DictionaryShape<T> = {
  [K in keyof T]: T[K] extends string
    ? string
    : T[K] extends Record<string, unknown>
      ? DictionaryShape<T[K]>
      : T[K]
}

type FullEnglish = typeof core & { admin: typeof admin; auth: typeof auth }

/** The complete translation dictionary (core + admin + auth). */
export type Dictionary = DictionaryShape<FullEnglish>

export type CoreDictionary = DictionaryShape<typeof core>
export type AdminDictionary = DictionaryShape<typeof admin>
export type AuthDictionary = DictionaryShape<typeof auth>

export type LocaleSectionName = 'admin' | 'auth'
