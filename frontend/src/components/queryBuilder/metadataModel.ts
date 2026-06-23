import type { ColumnRow, SemanticJoin, SemanticModelDetail, TableRow } from '../../types/semantic'

export interface TableOption {
  value: string
  label: string
  hint?: string
}

export type ColumnsByTable = Record<string, ColumnRow[]>

const NUMERIC_TYPES = [
  'bigint',
  'decimal',
  'double',
  'float',
  'int',
  'money',
  'numeric',
  'real',
  'serial',
]

export function metadataTableKey(schema: string, table: string): string {
  return `${schema}.${table}`
}

export function splitMetadataTableKey(key: string): { schema: string; table: string } {
  const dot = key.indexOf('.')
  if (dot < 0) {
    return { schema: '', table: key }
  }
  return { schema: key.slice(0, dot), table: key.slice(dot + 1) }
}

export function metadataTableOptions(tables: TableRow[]): TableOption[] {
  return tables
    .map((table) => ({
      value: metadataTableKey(table.schema_name, table.table_name),
      label: metadataTableKey(table.schema_name, table.table_name),
      hint: table.table_type,
    }))
    .sort((a, b) => a.label.localeCompare(b.label))
}

export function metadataJoinTableKeys(baseTableKey: string, joins: SemanticJoin[]): string[] {
  const keys = new Set<string>()
  if (baseTableKey) {
    keys.add(baseTableKey)
  }
  for (const join of joins) {
    if (join.from_table) {
      keys.add(metadataTableKey(join.from_schema ?? '', join.from_table))
    }
    if (join.to_table) {
      keys.add(metadataTableKey(join.to_schema ?? '', join.to_table))
    }
  }
  return [...keys].filter((key) => !key.startsWith('.'))
}

export function newMetadataJoin(baseTableKey: string): SemanticJoin {
  const { schema, table } = splitMetadataTableKey(baseTableKey)
  return {
    id: newJoinId(),
    name: '',
    from_schema: schema,
    from_table: table,
    from_column: '',
    to_schema: schema,
    to_table: '',
    to_column: '',
    join_type: 'LEFT',
    relationship: 'many_to_one',
    is_active: true,
  }
}

export function normalizeMetadataJoin(join: SemanticJoin): SemanticJoin {
  const name =
    join.name ||
    [join.from_table, join.from_column, join.to_table, join.to_column].filter(Boolean).join('_')
  return { ...join, name: name || join.id, is_active: join.is_active ?? true }
}

export function buildMetadataModel({
  datasourceId,
  baseTableKey,
  tables,
  columnsByTable,
  joins,
}: {
  datasourceId: string
  baseTableKey: string
  tables: TableRow[]
  columnsByTable: ColumnsByTable
  joins: SemanticJoin[]
}): SemanticModelDetail | null {
  const { schema: baseSchema, table: baseTable } = splitMetadataTableKey(baseTableKey)
  if (!datasourceId || !baseSchema || !baseTable) {
    return null
  }
  const tableByKey = new Map(
    tables.map((table) => [metadataTableKey(table.schema_name, table.table_name), table]),
  )
  if (!tableByKey.has(baseTableKey)) {
    return null
  }

  const includedKeys = metadataJoinTableKeys(baseTableKey, joins)
  const dimensions = includedKeys.flatMap((key) =>
    (columnsByTable[key] ?? []).map((column) => ({
      id: `metadata-dim:${key}.${column.column_name}`,
      model_id: 'metadata',
      name: metadataFieldName(column),
      label: column.column_name,
      column_ref: metadataColumnRef(column),
      type: semanticDimensionType(column.data_type),
      description: column.description,
      is_active: true,
    })),
  )

  const metrics = includedKeys.flatMap((key) => {
    const columns = columnsByTable[key] ?? []
    const countColumn = columns.find((column) => column.is_primary_key) ?? columns[0]
    const countMetric = countColumn
      ? [
          {
            id: `metadata-metric:${key}.row_count`,
            model_id: 'metadata',
            name: metadataMetricName('row_count', countColumn),
            label: `Row count (${key})`,
            expression: '*',
            aggregation: 'count',
            is_active: true,
          },
        ]
      : []
    const numericMetrics = columns.filter(isNumericColumn).flatMap((column) => [
      {
        id: `metadata-metric:${key}.${column.column_name}.sum`,
        model_id: 'metadata',
        name: metadataMetricName('sum', column),
        label: `Sum ${column.column_name} (${key})`,
        expression: metadataColumnRef(column),
        aggregation: 'sum',
        is_active: true,
      },
      {
        id: `metadata-metric:${key}.${column.column_name}.avg`,
        model_id: 'metadata',
        name: metadataMetricName('avg', column),
        label: `Avg ${column.column_name} (${key})`,
        expression: metadataColumnRef(column),
        aggregation: 'avg',
        is_active: true,
      },
    ])
    return [...countMetric, ...numericMetrics]
  })

  return {
    id: metadataModelId(datasourceId, baseTableKey),
    datasource_id: datasourceId,
    name: `metadata_${baseSchema}_${baseTable}`,
    label: `Metadata: ${baseTableKey}`,
    base_schema: baseSchema,
    base_table: baseTable,
    status: 'published',
    dimensions,
    metrics,
    joins: joins.map(normalizeMetadataJoin),
  }
}

export function metadataModelId(datasourceId: string, baseTableKey: string): string {
  return `metadata:${datasourceId}:${baseTableKey}`
}

export function metadataColumnRef(column: ColumnRow): string {
  return `${column.schema_name}.${column.table_name}.${column.column_name}`
}

function metadataFieldName(column: ColumnRow): string {
  return [column.schema_name, column.table_name, column.column_name].map(slugPart).join('__')
}

function metadataMetricName(prefix: string, column: ColumnRow): string {
  return [prefix, column.schema_name, column.table_name, column.column_name]
    .map(slugPart)
    .join('__')
}

function semanticDimensionType(dataType: string): string {
  const lower = dataType.toLowerCase()
  if (lower.includes('date') || lower.includes('time')) {
    return 'date'
  }
  if (lower.includes('bool')) {
    return 'boolean'
  }
  if (NUMERIC_TYPES.some((type) => lower.includes(type))) {
    return 'number'
  }
  return 'text'
}

function isNumericColumn(column: ColumnRow): boolean {
  const lower = column.data_type.toLowerCase()
  return NUMERIC_TYPES.some((type) => lower.includes(type))
}

function slugPart(value: string): string {
  return value.replace(/[^A-Za-z0-9_]/g, '_')
}

function newJoinId(): string {
  return typeof crypto !== 'undefined' && 'randomUUID' in crypto
    ? crypto.randomUUID()
    : `join-${Date.now()}-${Math.random().toString(36).slice(2)}`
}
