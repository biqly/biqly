import { createContext } from 'react'

import type { I18nContextValue } from './locale'

export const I18nContext = createContext<I18nContextValue | null>(null)
