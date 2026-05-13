export function getRateColor(rate: number): string {
  if (rate >= 80) return 'var(--success)'
  if (rate >= 50) return 'var(--warning)'
  return 'var(--error)'
}
