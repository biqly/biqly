function hasUnsupportedEmailText(value: string): boolean {
  for (const char of value) {
    const code = char.charCodeAt(0)
    if (char === '<' || char === '>' || code <= 0x1f) {
      return true
    }
  }
  return false
}

export function isValidEmailFormat(raw: string): boolean {
  const email = raw.trim()
  if (!email) {
    return false
  }
  if (hasUnsupportedEmailText(email)) {
    return false
  }
  if (/\s/.test(email)) {
    return false
  }
  if ((email.match(/@/g) ?? []).length !== 1) {
    return false
  }

  const at = email.lastIndexOf('@')
  const local = email.slice(0, at)
  const domain = email.slice(at + 1)
  if (!local || !domain) {
    return false
  }
  if (!domain.includes('.')) {
    return false
  }
  if (domain.split('.').some((part) => part.length === 0)) {
    return false
  }

  return true
}
