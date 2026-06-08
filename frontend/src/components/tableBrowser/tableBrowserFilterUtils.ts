import { parseJsonStringArray } from '../../utils/record'

export function formatTableBrowserFilterValue(value: string): string {
  let raw = value
  if (value.startsWith('[') && value.endsWith(']')) {
    try {
      const arr = JSON.parse(value) as string[]
      if (arr.length > 1) {
        return arr.map((item) => `"${item}"`).join(' or ')
      }
      if (arr.length === 1 && arr[0]) {
        raw = arr[0]
      }
    } catch {
      // ignore
    }
  }
  return `"${raw}"`
}

export function parseFilterChips(value: string): string[] {
  if (value.startsWith('[') && value.endsWith(']')) {
    return parseJsonStringArray(value) ?? [value]
  }
  if (value) {
    return [value]
  }
  return []
}

export function tableBrowserOperatorLabel(op: string, labels: Record<string, string>): string {
  return labels[op] ?? op
}

export function buildFilterSaveValue(chips: string[], chipInputText: string): string | null {
  const finalChips = [...chips]
  const textVal = chipInputText.trim()
  if (textVal && !finalChips.includes(textVal)) {
    finalChips.push(textVal)
  }
  if (finalChips.length === 0) {
    return null
  }
  return finalChips.length > 1 ? JSON.stringify(finalChips) : (finalChips[0] ?? '')
}
