import type { PasswordPolicy } from '../../types/auth'

// Mirror of the Go-side PasswordScore so client and server agree on the gate.
// Returns 0..4 where 0 is empty/unscorable and 4 is strong.
export function scorePassword(password: string): number {
  if (!password) {
    return 0
  }
  const length = [...password].length
  let classes = 0
  let hasUpper = false,
    hasLower = false,
    hasDigit = false,
    hasSpecial = false
  for (const ch of password) {
    if (/[A-ZÇĞİÖŞÜ]/.test(ch)) {
      hasUpper = true
    } else if (/[a-zçğıöşü]/.test(ch)) {
      hasLower = true
    } else if (/[0-9]/.test(ch)) {
      hasDigit = true
    } else {
      hasSpecial = true
    }
  }
  for (const c of [hasUpper, hasLower, hasDigit, hasSpecial]) {
    if (c) {
      classes++
    }
  }

  let score = 0
  if (length >= 16) {
    score += 2
  } else if (length >= 12) {
    score += 1
  }
  if (classes === 4) {
    score += 2
  } else if (classes === 3) {
    score += 1
  }
  if (length >= 20) {
    score++
  }

  if (hasRunOrSequence(password)) {
    score--
  }
  if (isObviouslyWeak(password)) {
    score -= 2
  }

  if (score < 0) {
    score = 0
  }
  if (score > 4) {
    score = 4
  }
  return score
}

function hasRunOrSequence(password: string): boolean {
  const runes = [...password.toLowerCase()]
  if (runes.length < 4) {
    return false
  }
  let run = 1
  for (let i = 1; i < runes.length; i++) {
    const cur = runes[i] ?? ''
    const prev = runes[i - 1] ?? ''
    if (cur === prev) {
      run++
      if (run >= 4) {
        return true
      }
    } else {
      run = 1
    }
  }
  let seq = 1
  for (let i = 1; i < runes.length; i++) {
    const cur = runes[i] ?? ''
    const prev = runes[i - 1] ?? ''
    if (cur.charCodeAt(0) === prev.charCodeAt(0) + 1) {
      seq++
      if (seq >= 4) {
        return true
      }
    } else {
      seq = 1
    }
  }
  return false
}

// Smaller client-side blocklist; full check happens server-side. Keep this tiny
// so we don't ship a giant wordlist to every page load.
const CLIENT_WEAK_PREFIXES = [
  'password',
  'qwerty',
  'letmein',
  'welcome',
  'admin',
  'login',
  'monkey',
  '12345',
  '123456',
  'iloveyou',
  'biqly',
  'abi',
  'master',
]

function isObviouslyWeak(password: string): boolean {
  const normalized = password.toLowerCase().replace(/[^a-z]/g, '')
  return CLIENT_WEAK_PREFIXES.some((w) => normalized.startsWith(w))
}

export interface RuleStatus {
  key: string
  label: string
  ok: boolean
}

export interface RuleLabels {
  length: string
  upper: string
  lower: string
  digit: string
  special: string
}

// rulesFor builds the dynamic per-policy checklist that the strength meter
// renders. Labels stay translatable through the caller via the t() prop.
export function rulesFor(
  password: string,
  policy: PasswordPolicy,
  labels: RuleLabels,
): RuleStatus[] {
  const length = [...password].length
  const rules: RuleStatus[] = [
    { key: 'length', label: labels.length, ok: length >= policy.min_length },
  ]
  if (policy.require_upper) {
    rules.push({ key: 'upper', label: labels.upper, ok: /[A-ZÇĞİÖŞÜ]/.test(password) })
  }
  if (policy.require_lower) {
    rules.push({ key: 'lower', label: labels.lower, ok: /[a-zçğıöşü]/.test(password) })
  }
  if (policy.require_digit) {
    rules.push({ key: 'digit', label: labels.digit, ok: /[0-9]/.test(password) })
  }
  if (policy.require_special) {
    rules.push({
      key: 'special',
      label: labels.special,
      ok: /[^A-Za-z0-9ÇĞİÖŞÜçğıöşü]/.test(password),
    })
  }
  return rules
}

// Bucketed score → level (1..3) maps onto the existing 3-segment strength bar.
export function scoreToLevel(score: number): { level: 0 | 1 | 2 | 3; cssClass: string } {
  if (score === 0) {
    return { level: 0, cssClass: '' }
  }
  if (score <= 1) {
    return { level: 1, cssClass: 'strength-bar--weak' }
  }
  if (score === 2) {
    return { level: 2, cssClass: 'strength-bar--medium' }
  }
  if (score === 3) {
    return { level: 3, cssClass: 'strength-bar--medium' }
  }
  return { level: 3, cssClass: 'strength-bar--strong' }
}
