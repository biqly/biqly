import { describe, expect, it } from 'vitest'

import { joinDataTypesCompatible, normalizeJoinDataType } from './joinCompatibility'

describe('normalizeJoinDataType', () => {
  it('groups integer variants', () => {
    expect(normalizeJoinDataType('smallint')).toBe('integer')
    expect(normalizeJoinDataType('BIGINT')).toBe('integer')
    expect(normalizeJoinDataType('bigserial')).toBe('integer')
  })

  it('groups numerics separately from integers', () => {
    expect(normalizeJoinDataType('numeric(12,2)')).toBe('decimal')
    expect(normalizeJoinDataType('double precision')).toBe('decimal')
    expect(normalizeJoinDataType('real')).toBe('decimal')
  })

  it('groups text-likes including citext', () => {
    expect(normalizeJoinDataType('character varying(255)')).toBe('text')
    expect(normalizeJoinDataType('citext')).toBe('text')
    expect(normalizeJoinDataType('text')).toBe('text')
  })

  it('keeps uuid in its own group', () => {
    expect(normalizeJoinDataType('uuid')).toBe('uuid')
    expect(normalizeJoinDataType('uniqueidentifier')).toBe('uuid')
  })

  it('groups timestamps and dates separately', () => {
    expect(normalizeJoinDataType('timestamp with time zone')).toBe('timestamp')
    expect(normalizeJoinDataType('timestamptz')).toBe('timestamp')
    expect(normalizeJoinDataType('date')).toBe('date')
  })

  it('returns empty string for unknown types', () => {
    expect(normalizeJoinDataType('hstore')).toBe('')
    expect(normalizeJoinDataType('')).toBe('')
  })
})

describe('joinDataTypesCompatible', () => {
  it('allows same-group pairs', () => {
    expect(joinDataTypesCompatible('integer', 'bigint')).toBe(true)
    expect(joinDataTypesCompatible('text', 'citext')).toBe(true)
    expect(joinDataTypesCompatible('uuid', 'uuid')).toBe(true)
    expect(joinDataTypesCompatible('jsonb', 'json')).toBe(true)
    expect(joinDataTypesCompatible('bool', 'boolean')).toBe(true)
  })

  it('allows integers joined to numerics', () => {
    expect(joinDataTypesCompatible('integer', 'numeric(10,2)')).toBe(true)
    expect(joinDataTypesCompatible('double precision', 'bigint')).toBe(true)
  })

  it('allows date joined to timestamp', () => {
    expect(joinDataTypesCompatible('date', 'timestamp with time zone')).toBe(true)
    expect(joinDataTypesCompatible('timestamp', 'date')).toBe(true)
  })

  it('rejects cross-group pairs', () => {
    expect(joinDataTypesCompatible('date', 'uuid')).toBe(false)
    expect(joinDataTypesCompatible('uuid', 'text')).toBe(false)
    expect(joinDataTypesCompatible('boolean', 'integer')).toBe(false)
    expect(joinDataTypesCompatible('jsonb', 'text')).toBe(false)
    expect(joinDataTypesCompatible('timestamp', 'integer')).toBe(false)
  })

  it('fails open for unknown types', () => {
    expect(joinDataTypesCompatible('hstore', 'uuid')).toBe(true)
    expect(joinDataTypesCompatible('ltree', 'hstore')).toBe(true)
    expect(joinDataTypesCompatible('', 'text')).toBe(true)
  })
})
