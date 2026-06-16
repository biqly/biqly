import { useEffect, useState } from 'react'

import { apiGetPasswordPolicy } from '../../api/auth'
import { useT } from '../../i18n'
import { cn } from '../../lib/cn'
import type { PasswordPolicy } from '../../types/auth'
import { rulesFor, scorePassword, scoreToLevel } from './passwordStrength'

interface Props {
  password: string
  onValidityChange?: (info: {
    valid: boolean
    score: number
    policy: PasswordPolicy | null
  }) => void
}

const toneTextClass = {
  weak: 'text-error',
  medium: 'text-warning',
  strong: 'text-success',
} as const

const toneBarClass = {
  weak: 'bg-error',
  medium: 'bg-warning',
  strong: 'bg-success',
} as const

export default function PasswordStrengthMeter({ password, onValidityChange }: Props) {
  const t = useT()
  const [policy, setPolicy] = useState<PasswordPolicy | null>(null)

  useEffect(() => {
    let alive = true
    void apiGetPasswordPolicy().then((p) => {
      if (alive) {
        setPolicy(p)
      }
    })
    return () => {
      alive = false
    }
  }, [])

  const labels = {
    length: policy
      ? t('auth.rule_length_dynamic').replace('{n}', String(policy.min_length))
      : t('auth.rule_length'),
    upper: t('auth.rule_uppercase'),
    lower: t('auth.rule_lowercase'),
    digit: t('auth.rule_digit'),
    special: t('auth.rule_special'),
  }

  const rules = policy ? rulesFor(password, policy, labels) : []
  const allRulesOk = rules.length > 0 && rules.every((r) => r.ok)
  const score = scorePassword(password)
  const meetsScore = policy ? score >= policy.min_score : true
  const { level, tone } = scoreToLevel(score)

  const strengthLabel =
    password.length === 0
      ? ''
      : score <= 1
        ? t('auth.strength_weak')
        : score === 2
          ? t('auth.strength_medium')
          : t('auth.strength_strong')

  useEffect(() => {
    onValidityChange?.({
      valid: allRulesOk && meetsScore,
      score,
      policy,
    })
  }, [allRulesOk, meetsScore, score, policy, onValidityChange])

  if (password.length === 0 || !policy) {
    return null
  }

  return (
    <div className="border-border bg-card-raised/60 mt-2 rounded-lg border p-3" aria-live="polite">
      <div className="flex items-center justify-between gap-3 text-xs">
        <span className="text-foreground-muted">{t('auth.strength_label')}</span>
        <span className={cn('font-semibold', tone ? toneTextClass[tone] : 'text-foreground-muted')}>
          {strengthLabel}
        </span>
      </div>

      <div className="mt-2 flex gap-1" aria-hidden="true">
        {[1, 2, 3].map((segment) => (
          <div
            key={segment}
            className={cn(
              'bg-border h-1 flex-1 rounded-full transition-colors duration-200',
              segment <= level && tone ? toneBarClass[tone] : '',
            )}
          />
        ))}
      </div>

      <ul className="mt-3 grid grid-cols-1 gap-1.5 sm:grid-cols-2">
        {rules.map((rule) => (
          <li
            key={rule.key}
            className={cn(
              'flex min-w-0 items-center gap-2 text-xs leading-snug',
              rule.ok ? 'text-success' : 'text-foreground-muted',
            )}
          >
            <span
              className={cn(
                'text-micro inline-flex h-4 w-4 shrink-0 items-center justify-center rounded-full border font-bold',
                rule.ok
                  ? 'border-success/35 bg-success/10 text-success'
                  : 'border-border bg-transparent text-transparent',
              )}
              aria-hidden="true"
            >
              ✓
            </span>
            <span>{rule.label}</span>
          </li>
        ))}
      </ul>
    </div>
  )
}
