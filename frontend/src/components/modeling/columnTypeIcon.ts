import { normalizeJoinDataType } from './utils'

export type ColumnTypeKind =
  'number' | 'text' | 'boolean' | 'date' | 'timestamp' | 'json' | 'array' | 'other'

export interface ColumnTypeIcon {
  kind: ColumnTypeKind
  glyph: string
}

const GLYPHS: Record<ColumnTypeKind, string> = {
  number: '123',
  text: 'A-Z',
  boolean: '✓',
  date: '▦',
  timestamp: '◷',
  json: '{}',
  array: '[]',
  other: '·',
}

export function columnTypeIcon(dataType: string): ColumnTypeIcon {
  if (/\[\]\s*$/.test(dataType.trim())) {
    return { kind: 'array', glyph: GLYPHS.array }
  }

  const bucket = normalizeJoinDataType(dataType)
  const kind: ColumnTypeKind =
    bucket === 'integer' || bucket === 'decimal'
      ? 'number'
      : bucket === 'text'
        ? 'text'
        : bucket === 'boolean'
          ? 'boolean'
          : bucket === 'date'
            ? 'date'
            : bucket === 'timestamp'
              ? 'timestamp'
              : bucket === 'json'
                ? 'json'
                : 'other'

  return { kind, glyph: GLYPHS[kind] }
}
