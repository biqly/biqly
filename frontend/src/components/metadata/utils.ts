import type { TranslationKey } from '../../i18n'
import type { ColumnRow } from '../../types/semantic'

export type MetadataTFunction = (
  key: TranslationKey,
  params?: Record<string, string | number>,
) => string

export type MetadataEditingKind = 'table' | 'column'

export interface MetadataEditingState {
  kind: MetadataEditingKind
  id: string
  value: string
}

export const DESC_TEXTAREA_MAX_ROWS = 24
export const DESC_SOFT_WRAP_CHARS = 72

export function columnKeySuffix(c: ColumnRow, t: MetadataTFunction): string | null {
  const parts: string[] = []
  if (c.is_primary_key) {
    parts.push(t('metadata.col_pk'))
  }
  if (c.is_foreign_key) {
    if (c.referenced_table && c.referenced_column) {
      const refSchema = c.referenced_schema?.trim()
      const crossSchema = refSchema && refSchema !== c.schema_name
      const target = crossSchema
        ? `${refSchema}.${c.referenced_table}.${c.referenced_column}`
        : `${c.referenced_table}.${c.referenced_column}`
      parts.push(t('metadata.col_fk_target', { target }))
    } else {
      parts.push(t('metadata.col_fk'))
    }
  }
  if (parts.length === 0) {
    return null
  }
  return parts.join(', ')
}

export function textareaRowsForDescription(text: string | null | undefined): number {
  const raw = text ?? ''
  if (!raw.trim()) {
    return 1
  }
  const parts = raw.split('\n')
  let rows = 0
  for (const line of parts) {
    rows += line.length === 0 ? 1 : Math.max(1, Math.ceil(line.length / DESC_SOFT_WRAP_CHARS))
  }
  return Math.min(DESC_TEXTAREA_MAX_ROWS, Math.max(1, rows))
}
