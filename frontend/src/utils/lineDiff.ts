export interface DiffLine {
  type: 'same' | 'add' | 'del'
  text: string
}

/**
 * Compute a unified line diff between two texts using a standard LCS table.
 * Suitable for semantic model YAML exports (small files, a few hundred lines).
 */
export function diffLines(before: string, after: string): DiffLine[] {
  const a = before.split('\n')
  const b = after.split('\n')
  const n = a.length
  const m = b.length
  // lcs[i][j] = LCS length of a[i:] and b[j:]
  const lcs: number[][] = Array.from({ length: n + 1 }, () => new Array<number>(m + 1).fill(0))
  const cell = (i: number, j: number): number => lcs[i]?.[j] ?? 0
  for (let i = n - 1; i >= 0; i--) {
    for (let j = m - 1; j >= 0; j--) {
      const row = lcs[i]
      if (row) {
        row[j] = a[i] === b[j] ? cell(i + 1, j + 1) + 1 : Math.max(cell(i + 1, j), cell(i, j + 1))
      }
    }
  }
  const out: DiffLine[] = []
  let i = 0
  let j = 0
  while (i < n && j < m) {
    if (a[i] === b[j]) {
      out.push({ type: 'same', text: a[i] ?? '' })
      i++
      j++
    } else if (cell(i + 1, j) >= cell(i, j + 1)) {
      out.push({ type: 'del', text: a[i] ?? '' })
      i++
    } else {
      out.push({ type: 'add', text: b[j] ?? '' })
      j++
    }
  }
  for (; i < n; i++) {
    out.push({ type: 'del', text: a[i] ?? '' })
  }
  for (; j < m; j++) {
    out.push({ type: 'add', text: b[j] ?? '' })
  }
  return out
}
