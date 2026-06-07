import { useEffect, useState } from 'react'

import { apiGetPasswordPolicy } from '../../api/auth'
import { useT } from '../../i18n'
import type { PasswordPolicy } from '../../types/auth'
import { rulesFor, scorePassword, scoreToLevel } from './passwordStrength'

interface Props {
  password: string
  // Called whenever rules+score are recomputed so the parent form can gate the
  // submit button on policy compliance without duplicating logic.
  onValidityChange?: (info: {
    valid: boolean
    score: number
    policy: PasswordPolicy | null
  }) => void
}

// PasswordStrengthMeter fetches the live server policy on mount and renders a
// rule checklist + 3-bar strength meter that mirrors the backend scorer.
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
  const { level, cssClass } = scoreToLevel(score)

  const strengthLabel =
    password.length === 0
      ? ''
      : score <= 1
        ? t('auth.strength_weak')
        : score === 2
          ? t('auth.strength_medium')
          : t('auth.strength_strong')

  // Notify the parent on every change so the submit gate stays in sync.
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
    <div className="password-strength" aria-live="polite">
      <div className="strength-label-row">
        <span>{t('auth.strength_label')}</span>
        <span>{strengthLabel}</span>
      </div>
      <div className="strength-bars">
        <div className={`strength-bar ${level >= 1 ? cssClass : ''}`} />
        <div className={`strength-bar ${level >= 2 ? cssClass : ''}`} />
        <div className={`strength-bar ${level >= 3 ? cssClass : ''}`} />
      </div>
      <ul className="password-rules">
        {rules.map((r) => (
          <li key={r.key} className={`rule-item ${r.ok ? 'valid' : ''}`}>
            <span className="rule-bullet" />
            {r.label}
          </li>
        ))}
      </ul>
    </div>
  )
}
