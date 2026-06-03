import { describe, expect, it } from 'vitest'

import {
  DEFAULT_PII_MASKING_STRATEGY,
  piiMaskingStrategyLabelKey,
  normalizePIIMaskingStrategy,
  piiStrategyChanged,
  shouldShowPIIConfirmAction,
} from './piiDetectionPanelLogic'

describe('piiDetectionPanelLogic', () => {
  it('normalizes unknown masking strategies to the partial default', () => {
    expect(normalizePIIMaskingStrategy(null)).toBe(DEFAULT_PII_MASKING_STRATEGY)
    expect(normalizePIIMaskingStrategy('surprise')).toBe(DEFAULT_PII_MASKING_STRATEGY)
    expect(normalizePIIMaskingStrategy('full')).toBe('full')
  })

  it('shows the confirm action for reviewed rows when strategy changes', () => {
    expect(shouldShowPIIConfirmAction({ canEdit: true, reviewedBy: 'admin', typeChanged: false, strategyChanged: true })).toBe(
      true,
    )
  })

  it('hides the confirm action for unchanged reviewed rows', () => {
    expect(
      shouldShowPIIConfirmAction({ canEdit: true, reviewedBy: 'admin', typeChanged: false, strategyChanged: false }),
    ).toBe(false)
  })

  it('detects pending strategy changes against normalized values', () => {
    expect(piiStrategyChanged('full', 'partial')).toBe(true)
    expect(piiStrategyChanged(null, 'partial')).toBe(false)
  })

  it('maps masking strategies to i18n label keys', () => {
    expect(piiMaskingStrategyLabelKey('full')).toBe('admin.pii.strategy_full')
    expect(piiMaskingStrategyLabelKey('partial')).toBe('admin.pii.strategy_partial')
  })
})
