import type { TranslationKey } from '../../i18n'

export const PII_MASKING_STRATEGIES = ['partial', 'full'] as const
export type PIIMaskingStrategy = (typeof PII_MASKING_STRATEGIES)[number]
export const DEFAULT_PII_MASKING_STRATEGY: PIIMaskingStrategy = 'partial'

export function normalizePIIMaskingStrategy(strategy: string | null | undefined): PIIMaskingStrategy {
  return strategy === 'full' ? 'full' : DEFAULT_PII_MASKING_STRATEGY
}

export function piiMaskingStrategyLabelKey(strategy: string | null | undefined): TranslationKey {
  return normalizePIIMaskingStrategy(strategy) === 'full' ? 'admin.pii.strategy_full' : 'admin.pii.strategy_partial'
}

export function piiStrategyChanged(current: string | null | undefined, pending: string | undefined): boolean {
  if (pending == null) {
    return false
  }
  return normalizePIIMaskingStrategy(current) !== normalizePIIMaskingStrategy(pending)
}

export function shouldShowPIIConfirmAction({
  canEdit,
  reviewedBy,
  typeChanged,
  strategyChanged,
}: {
  canEdit: boolean
  reviewedBy: string | null | undefined
  typeChanged: boolean
  strategyChanged: boolean
}): boolean {
  if (!canEdit) {
    return false
  }
  return !reviewedBy || typeChanged || strategyChanged
}
