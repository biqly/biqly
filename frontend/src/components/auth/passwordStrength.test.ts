import { describe, expect, it } from 'vitest'

import type { PasswordPolicy } from '../../types/auth'
import { rulesFor, scorePassword, scoreToLevel } from './passwordStrength'

const defaultPolicy: PasswordPolicy = {
  min_length: 8,
  max_length: 128,
  require_upper: true,
  require_lower: true,
  require_digit: true,
  require_special: true,
  min_score: 2,
}

const labels = {
  length: 'len',
  upper: 'upper',
  lower: 'lower',
  digit: 'digit',
  special: 'special',
}

describe('scorePassword', () => {
  it('rates an empty password as 0', () => {
    expect(scorePassword('')).toBe(0)
  })

  it('penalizes obviously common passwords', () => {
    expect(scorePassword('password')).toBeLessThanOrEqual(1)
    expect(scorePassword('qwerty1!')).toBeLessThanOrEqual(2)
  })

  it('rewards length and class variety', () => {
    expect(scorePassword('GreenHorse-Volcano-7!Pier')).toBeGreaterThanOrEqual(3)
  })

  it('penalizes monotone sequences and runs', () => {
    expect(scorePassword('aaaa')).toBeLessThanOrEqual(1)
    expect(scorePassword('abcd1234')).toBeLessThanOrEqual(2)
  })

  it('caps at 4', () => {
    expect(scorePassword('correcthorsebatterystaple!2026')).toBeLessThanOrEqual(4)
  })
})

describe('rulesFor', () => {
  it('returns one rule per requirement plus length', () => {
    const rules = rulesFor('Aa1!aaaa', defaultPolicy, labels)
    expect(rules).toHaveLength(5)
    expect(rules.every((r) => r.ok)).toBe(true)
  })

  it('marks unmet length as failing', () => {
    const rules = rulesFor('Aa1!', defaultPolicy, labels)
    const lengthRule = rules.find((r) => r.key === 'length')
    expect(lengthRule?.ok).toBe(false)
  })

  it('skips classes not required by policy', () => {
    const policy: PasswordPolicy = {
      ...defaultPolicy,
      require_special: false,
      require_lower: false,
    }
    const rules = rulesFor('PASSWORD1', policy, labels)
    expect(rules.map((r) => r.key)).toEqual(['length', 'upper', 'digit'])
  })
})

describe('scoreToLevel', () => {
  it('maps scores to bar levels with weak/medium/strong classes', () => {
    expect(scoreToLevel(0)).toEqual({ level: 0, cssClass: '' })
    expect(scoreToLevel(1)).toEqual({ level: 1, cssClass: 'strength-bar--weak' })
    expect(scoreToLevel(2).cssClass).toBe('strength-bar--medium')
    expect(scoreToLevel(4).cssClass).toBe('strength-bar--strong')
  })
})
