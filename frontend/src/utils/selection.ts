/**
 * Pure id-set selection transitions (Faz 5,
 * tasks/frontend-table-pagination-standardization.md). Extracted from the
 * hand-rolled Set juggling in RolesPanel; consumed via hooks/useRowSelection.
 */

export function toggleId(ids: ReadonlySet<string>, id: string): Set<string> {
  const next = new Set(ids)
  if (next.has(id)) {
    next.delete(id)
  } else {
    next.add(id)
  }
  return next
}

/** Add or remove a whole group at once (the select-all checkbox path). */
export function setIds(
  ids: ReadonlySet<string>,
  group: readonly string[],
  on: boolean,
): Set<string> {
  const next = new Set(ids)
  for (const id of group) {
    if (on) {
      next.add(id)
    } else {
      next.delete(id)
    }
  }
  return next
}

export function sameIdSet(a: ReadonlySet<string>, b: ReadonlySet<string>): boolean {
  if (a.size !== b.size) {
    return false
  }
  for (const id of a) {
    if (!b.has(id)) {
      return false
    }
  }
  return true
}

/** Drives the select-all checkbox: 'all' → checked, 'some' → indeterminate, 'none' → unchecked. */
export function selectionStateFor(
  ids: ReadonlySet<string>,
  group: readonly string[],
): 'none' | 'some' | 'all' {
  if (group.length === 0) {
    return 'none'
  }
  let count = 0
  for (const id of group) {
    if (ids.has(id)) {
      count++
    }
  }
  if (count === 0) {
    return 'none'
  }
  return count === group.length ? 'all' : 'some'
}
