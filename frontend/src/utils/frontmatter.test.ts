import { describe, expect, it } from 'vitest'

import { parseFrontmatter } from './frontmatter'

describe('parseFrontmatter', () => {
  it('parses a YAML block and strips it from the body', () => {
    const doc =
      '---\ntype: glossary\nterm: money transfer\naliases: [a, b]\n---\n\n# Title\n\nBody.'
    const res = parseFrontmatter(doc)
    expect(res.frontmatter).toEqual({
      type: 'glossary',
      term: 'money transfer',
      aliases: ['a', 'b'],
    })
    expect(res.raw).toContain('term: money transfer')
    expect(res.body).toBe('# Title\n\nBody.')
  })

  it('returns null frontmatter when the document has none', () => {
    const res = parseFrontmatter('# Just markdown')
    expect(res.frontmatter).toBeNull()
    expect(res.body).toBe('# Just markdown')
  })

  it('treats an unterminated or invalid block as plain markdown', () => {
    expect(parseFrontmatter('---\ntitle: x\nno end').frontmatter).toBeNull()
    const invalid = '---\n[: not yaml\n---\n\nBody'
    expect(parseFrontmatter(invalid).frontmatter).toBeNull()
    expect(parseFrontmatter(invalid).body).toBe(invalid)
  })
})
