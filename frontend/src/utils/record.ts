export function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value)
}

export function parseJsonRecord(text: string): Record<string, unknown> | null {
  try {
    const parsed: unknown = JSON.parse(text)
    return isRecord(parsed) ? parsed : null
  } catch {
    return null
  }
}

export function parseJsonStringArray(text: string): string[] | null {
  try {
    const parsed: unknown = JSON.parse(text)
    if (!Array.isArray(parsed)) {
      return null
    }
    return parsed.map((item) => String(item))
  } catch {
    return null
  }
}
