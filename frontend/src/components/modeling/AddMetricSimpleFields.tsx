import type { useT } from '../../i18n'
import { modelingFormGroupClass } from '../../lib/formClasses'
import { modalFormRowClass } from '../../lib/modalClasses'
import type { ColumnRow, TableRow } from '../../types/semantic'
import { Select } from '../ui/Select'

const METRIC_AGGREGATION_OPTIONS = [
  { value: 'count', label: 'count' },
  { value: 'count_distinct', label: 'count_distinct' },
  { value: 'sum', label: 'sum' },
  { value: 'avg', label: 'avg' },
  { value: 'min', label: 'min' },
  { value: 'max', label: 'max' },
] as const

type StandardAggregation = (typeof METRIC_AGGREGATION_OPTIONS)[number]['value']

export function AddMetricSimpleFields({
  t,
  saving,
  availableSchemas,
  availableTables,
  availableColumns,
  selectedSchema,
  selectedTable,
  selectedColumn,
  selectedAggregation,
  onSchemaChange,
  onTableChange,
  onColumnChange,
  onAggregationChange,
}: {
  t: ReturnType<typeof useT>
  saving: boolean
  availableSchemas: string[]
  availableTables: TableRow[]
  availableColumns: ColumnRow[]
  selectedSchema: string
  selectedTable: string
  selectedColumn: string
  selectedAggregation: StandardAggregation
  onSchemaChange: (schema: string) => void
  onTableChange: (table: string) => void
  onColumnChange: (column: string) => void
  onAggregationChange: (agg: StandardAggregation) => void
}) {
  return (
    <>
      <div className={modalFormRowClass()}>
        <div className={modelingFormGroupClass}>
          <label htmlFor="metric-schema">{t('modeling.pick_schema')}</label>
          <Select
            id="metric-schema"
            name="schema"
            value={selectedSchema}
            onChange={onSchemaChange}
            disabled={saving}
            options={availableSchemas.map((s) => ({ value: s, label: s }))}
          />
        </div>
        <div className={modelingFormGroupClass}>
          <label htmlFor="metric-table">{t('modeling.pick_table')}</label>
          <Select
            id="metric-table"
            name="table"
            value={selectedTable}
            onChange={onTableChange}
            disabled={saving}
            options={availableTables.map((tbl) => ({
              value: tbl.table_name,
              label: tbl.label ?? tbl.table_name,
            }))}
          />
        </div>
      </div>
      <div className={modalFormRowClass()}>
        <div className={modelingFormGroupClass}>
          <label htmlFor="metric-column">{t('modeling.pick_column')}</label>
          <Select
            id="metric-column"
            name="column"
            value={selectedColumn}
            onChange={onColumnChange}
            disabled={saving}
            options={availableColumns.map((col) => ({
              value: col.column_name,
              label: `${col.column_name} (${col.data_type})`,
            }))}
          />
        </div>
        <div className={modelingFormGroupClass}>
          <label htmlFor="metric-aggregation">{t('modeling.metric_aggregation_label')}</label>
          <Select
            id="metric-aggregation"
            name="aggregation"
            value={selectedAggregation}
            onChange={onAggregationChange}
            disabled={saving}
            options={[...METRIC_AGGREGATION_OPTIONS]}
          />
        </div>
      </div>
    </>
  )
}
