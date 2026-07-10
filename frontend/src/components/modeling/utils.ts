import type { TranslationKey } from '../../i18n'
import type { ColumnRow, SemanticJoin, SemanticModelDetail, TableRow } from '../../types/semantic'
import type { JoinForm, JoinPayload } from './types'

export function tableKey(schema: string, table: string) {
  return `${schema}.${table}`
}

export function splitTableKey(key: string) {
  const idx = key.indexOf('.')
  if (idx === -1) {
    return { schema: '', table: key }
  }
  return { schema: key.slice(0, idx), table: key.slice(idx + 1) }
}

export function compareColumns(a: ColumnRow, b: ColumnRow, linked?: Set<string>) {
  const pk = Number(b.is_primary_key) - Number(a.is_primary_key)
  if (pk !== 0) {
    return pk
  }
  const fk = Number(b.is_foreign_key) - Number(a.is_foreign_key)
  if (fk !== 0) {
    return fk
  }
  if (linked) {
    const joinLinked = Number(linked.has(b.column_name)) - Number(linked.has(a.column_name))
    if (joinLinked !== 0) {
      return joinLinked
    }
  }
  return a.column_name.localeCompare(b.column_name)
}

export function columnOptions(columns: ColumnRow[], tableRef: string) {
  const { schema, table } = splitTableKey(tableRef)
  return columns
    .filter((c) => c.schema_name === schema && c.table_name === table)
    .sort((a, b) => compareColumns(a, b))
}

const DATA_TYPE_LABEL_KEYS: Record<string, TranslationKey> = {
  'timestamp with time zone': 'modeling.data_type_timestamptz',
  text: 'modeling.data_type_text',
  uuid: 'modeling.data_type_uuid',
  'user-defined': 'modeling.data_type_user_defined',
}

export function formatDataType(t: (key: TranslationKey) => string, dataType: string) {
  const key = DATA_TYPE_LABEL_KEYS[dataType.toLowerCase().trim()]
  return key ? t(key) : dataType
}

const RELATIONSHIP_LABEL_KEYS: Partial<Record<string, TranslationKey>> = {
  many_to_one: 'modeling.rel_many_to_one',
  one_to_many: 'modeling.rel_one_to_many',
  one_to_one: 'modeling.rel_one_to_one',
  many_to_many: 'modeling.rel_many_to_many',
}

export function relationshipLabel(
  t: (key: TranslationKey) => string,
  relationship: SemanticJoin['relationship'],
) {
  const key = RELATIONSHIP_LABEL_KEYS[relationship]
  return key ? t(key) : relationship
}

export function columnSelectHint(column: ColumnRow, t: (key: TranslationKey) => string) {
  const parts: string[] = []
  if (column.is_primary_key) {
    parts.push(t('modeling.pk_badge'))
  }
  if (column.is_foreign_key) {
    parts.push(t('modeling.fk_badge'))
  }
  parts.push(formatDataType(t, column.data_type))
  return parts.join(' · ')
}

export function columnSelectOptions(cols: ColumnRow[], t: (key: TranslationKey) => string) {
  return cols.map((column) => ({
    value: column.column_name,
    label: column.column_name,
    hint: columnSelectHint(column, t),
  }))
}

export function normalizeJoinDataType(dataType: string) {
  const type = dataType
    .toLowerCase()
    .replace(/\(.+\)/, '')
    .replace(/\s+/g, ' ')
    .trim()
  if (
    [
      'smallint',
      'int2',
      'integer',
      'int',
      'int4',
      'bigint',
      'int8',
      'serial',
      'serial4',
      'bigserial',
      'serial8',
    ].includes(type)
  ) {
    return 'integer'
  }
  if (
    [
      'text',
      'character varying',
      'varchar',
      'character',
      'char',
      'citext',
      'nvarchar',
      'nchar',
      'string',
    ].includes(type)
  ) {
    return 'text'
  }
  if (['boolean', 'bool'].includes(type)) {
    return 'boolean'
  }
  if (
    [
      'timestamp',
      'timestamp without time zone',
      'timestamp with time zone',
      'timestamptz',
      'datetime',
    ].includes(type)
  ) {
    return 'timestamp'
  }
  if (['date'].includes(type)) {
    return 'date'
  }
  if (
    [
      'numeric',
      'decimal',
      'double precision',
      'float',
      'float4',
      'float8',
      'real',
      'money',
    ].includes(type)
  ) {
    return 'decimal'
  }
  if (['json', 'jsonb'].includes(type)) {
    return 'json'
  }
  return type
}

export function columnsAreJoinCompatible(
  left: ColumnRow | null | undefined,
  right: ColumnRow | null | undefined,
) {
  if (!left || !right) {
    return false
  }
  return normalizeJoinDataType(left.data_type) === normalizeJoinDataType(right.data_type)
}

export function findColumn(columns: ColumnRow[], tableRef: string, columnName: string) {
  return (
    columnOptions(columns, tableRef).find((column) => column.column_name === columnName) ?? null
  )
}

export function firstCompatibleColumnName(
  columns: ColumnRow[],
  tableRef: string,
  sourceColumn: ColumnRow | null,
) {
  const options = columnOptions(columns, tableRef)
  if (!sourceColumn) {
    return options[0]?.column_name ?? ''
  }
  return options.find((column) => columnsAreJoinCompatible(sourceColumn, column))?.column_name ?? ''
}

export function defaultJoinForm(
  tables: TableRow[],
  columns: ColumnRow[],
  model: SemanticModelDetail | null,
): JoinForm {
  const base = model
    ? tableKey(model.base_schema, model.base_table)
    : tables[0]
      ? tableKey(tables[0].schema_name, tables[0].table_name)
      : ''
  const target = tables.find((tbl) => tableKey(tbl.schema_name, tbl.table_name) !== base)
  const toTable = target ? tableKey(target.schema_name, target.table_name) : base
  const fromColumn = columnOptions(columns, base)[0]?.column_name ?? ''
  const sourceColumn = findColumn(columns, base, fromColumn)
  return {
    fromTable: base,
    fromColumn,
    toTable,
    toColumn: firstCompatibleColumnName(columns, toTable, sourceColumn),
    joinType: 'LEFT',
    relationship: 'many_to_one',
  }
}

export function joinName(form: JoinForm) {
  const from = splitTableKey(form.fromTable)
  const to = splitTableKey(form.toTable)
  return `${from.table}_${form.fromColumn}_to_${to.table}_${form.toColumn}`
    .replace(/[^a-zA-Z0-9_]+/g, '_')
    .toLowerCase()
}

export function buildJoinPayload(form: JoinForm): JoinPayload {
  const from = splitTableKey(form.fromTable)
  const to = splitTableKey(form.toTable)
  return {
    name: joinName(form),
    from_schema: from.schema,
    from_table: from.table,
    from_column: form.fromColumn,
    to_schema: to.schema,
    to_table: to.table,
    to_column: form.toColumn,
    join_type: form.joinType,
    relationship: form.relationship,
  }
}

export function canSaveJoinForm(
  model: SemanticModelDetail | null,
  form: JoinForm,
  columns: ColumnRow[],
): boolean {
  if (!model || !form.fromTable || !form.fromColumn || !form.toTable || !form.toColumn) {
    return false
  }
  const fromCol = findColumn(columns, form.fromTable, form.fromColumn)
  const toCol = findColumn(columns, form.toTable, form.toColumn)
  return Boolean(fromCol && toCol && columnsAreJoinCompatible(fromCol, toCol))
}

export function patchJoinForm(
  prev: JoinForm,
  patch: Partial<JoinForm>,
  columns: ColumnRow[],
): JoinForm {
  const next = { ...prev, ...patch }
  if (patch.fromTable) {
    next.fromColumn = columnOptions(columns, patch.fromTable)[0]?.column_name ?? ''
  }
  const sourceColumn = findColumn(columns, next.fromTable, next.fromColumn)
  if (patch.fromTable || patch.fromColumn || patch.toTable) {
    const currentTarget = findColumn(columns, next.toTable, next.toColumn)
    if (!columnsAreJoinCompatible(sourceColumn, currentTarget)) {
      next.toColumn = firstCompatibleColumnName(columns, next.toTable, sourceColumn)
    }
  }
  return next
}

// Splits a dimension column_ref into its table key and column name, mirroring
// the backend generator's format (internal/semanticgen/generator.go columnRef):
// base-schema tables use `table.column`, others `schema.table.column`.
export function splitColumnRef(
  ref: string | undefined | null,
  baseSchema: string,
): { tableKey: string; column: string } | null {
  const parts = (ref ?? '').trim().split('.')
  if (parts.length === 3) {
    return { tableKey: tableKey(parts[0]!, parts[1]!), column: parts[2]! }
  }
  if (parts.length === 2) {
    return { tableKey: tableKey(baseSchema, parts[0]!), column: parts[1]! }
  }
  return null
}

// Mirrors the backend generator's semanticType (internal/semanticgen): the
// dimension type a raw column maps to when added to the model.
export function dimensionTypeFromDataType(dataType: string): string {
  switch (normalizeJoinDataType(dataType)) {
    case 'timestamp':
    case 'date':
      return 'date'
    case 'boolean':
      return 'boolean'
    case 'integer':
    case 'decimal':
      return 'number'
    default:
      return 'text'
  }
}

export function columnRefMatchesTable(
  ref: string | undefined | null,
  schema: string,
  table: string,
  baseSchema: string,
) {
  if (!ref) {
    return false
  }
  const r = ref.trim()
  if (!r) {
    return false
  }
  if (r.startsWith(`${schema}.${table}.`)) {
    return true
  }
  if (schema === baseSchema && r.startsWith(`${table}.`)) {
    return true
  }
  return false
}
