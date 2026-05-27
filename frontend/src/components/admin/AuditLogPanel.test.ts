import { describe, expect, it } from 'vitest'
import { AUDIT_PAGE_SIZE_OPTIONS, DEFAULT_AUDIT_PAGE_SIZE } from './AuditLogPanel'

describe('AuditLogPanel pagination defaults', () => {
  it('uses a paginated first page by default', () => {
    expect(DEFAULT_AUDIT_PAGE_SIZE).toBe(10)
    expect(AUDIT_PAGE_SIZE_OPTIONS[0]).toBe(DEFAULT_AUDIT_PAGE_SIZE)
  })
})
