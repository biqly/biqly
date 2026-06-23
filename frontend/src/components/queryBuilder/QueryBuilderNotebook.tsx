import type { SemanticModelDetail } from '../../types/semantic'
import type { MetadataTFunction } from '../metadata/utils'
import { Select } from '../ui/Select'
import { CteStep } from './CteStep'
import { FieldsStep } from './FieldsStep'
import { FilterStep } from './FilterStep'
import { HavingStep } from './HavingStep'
import type { TableOption } from './metadataModel'
import { NotebookStep } from './NotebookStep'
import { qbNotebookClass, qbTagBase, qbTagBlueClass } from './queryBuilderClasses'
import { QueryBuilderNotebookJoins } from './QueryBuilderNotebookJoins'
import {
  QueryBuilderNotebookToolbar,
  QueryBuilderVisualizeFooter,
} from './QueryBuilderNotebookToolbar'
import { SortStep } from './SortStep'
import { SummarizeStep } from './SummarizeStep'
import type { CTERow, FilterRow, HavingRow, SelectItem, WindowFuncRow } from './types'
import type { filterFieldOptions } from './utils'
import { dimFieldOptions, dimOptionsForGroupRow, metricFieldOptions } from './utils'
import { WindowFuncStep } from './WindowFuncStep'

export function QueryBuilderNotebook({
  modelDetail,
  sourceMode,
  baseTableKey,
  tableOptions,
  includedTableOptions,
  columnOptionsByTable,
  metadataJoinsEditable,
  onBaseTableChange,
  onAddMetadataJoin,
  onUpdateMetadataJoin,
  onRemoveMetadataJoin,
  filters,
  filterFieldOpts,
  updateFilter,
  removeFilter,
  addFilter,
  setFilters,
  isSummarized,
  selectItems,
  dimensions,
  metrics,
  updateSelectItem,
  removeSelectItem,
  addSelectItem,
  groupBy,
  addMetricSelectItem,
  updateGroupByRow,
  removeGroupByRow,
  addGroupByRow,
  setIsSummarized,
  setGroupBy,
  orderBy,
  orderDir,
  orderByOpts,
  setOrderBy,
  setOrderDir,
  limit,
  setLimit,
  mode,
  having,
  metricOptsHaving,
  updateHaving,
  removeHaving,
  addHaving,
  setHaving,
  windowFunctions,
  updateWindowFunc,
  removeWindowFunc,
  addWindowFunc,
  clearWindowFunctions,
  ctes,
  updateCTE,
  removeCTE,
  addCTE,
  clearCtes,
  toggleSummarize,
  loading,
  runQuery,
  sqlVisible,
  onToggleSql,
  fieldLabelMode,
  t,
}: {
  modelDetail: SemanticModelDetail
  sourceMode: 'semantic' | 'metadata'
  baseTableKey: string
  tableOptions: TableOption[]
  includedTableOptions: TableOption[]
  columnOptionsByTable: Record<string, { value: string; label: string; hint?: string }[]>
  metadataJoinsEditable: boolean
  onBaseTableChange: (value: string) => void
  onAddMetadataJoin: () => void
  onUpdateMetadataJoin: (
    index: number,
    join: NonNullable<SemanticModelDetail['joins']>[number],
  ) => void
  onRemoveMetadataJoin: (index: number) => void
  filters: FilterRow[]
  filterFieldOpts: ReturnType<typeof filterFieldOptions>
  updateFilter: (i: number, field: keyof FilterRow, v: string) => void
  removeFilter: (i: number) => void
  addFilter: () => void
  setFilters: (items: FilterRow[]) => void
  isSummarized: boolean
  selectItems: SelectItem[]
  dimensions: SemanticModelDetail['dimensions']
  metrics: SemanticModelDetail['metrics']
  updateSelectItem: (i: number, field: keyof SelectItem, v: string) => void
  removeSelectItem: (i: number) => void
  addSelectItem: () => void
  groupBy: string[]
  addMetricSelectItem: () => void
  updateGroupByRow: (index: number, value: string) => void
  removeGroupByRow: (index: number) => void
  addGroupByRow: () => void
  setIsSummarized: (value: boolean) => void
  setGroupBy: (items: string[]) => void
  orderBy: string
  orderDir: string
  orderByOpts: { value: string; label: string; hint?: string }[]
  setOrderBy: (value: string) => void
  setOrderDir: (value: string) => void
  limit: number
  setLimit: (value: number) => void
  mode: 'simple' | 'advanced'
  having: HavingRow[]
  metricOptsHaving: ReturnType<typeof metricFieldOptions>
  updateHaving: (i: number, field: keyof HavingRow, v: string) => void
  removeHaving: (i: number) => void
  addHaving: () => void
  setHaving: (items: HavingRow[]) => void
  windowFunctions: WindowFuncRow[]
  updateWindowFunc: (i: number, field: keyof WindowFuncRow, v: string) => void
  removeWindowFunc: (i: number) => void
  addWindowFunc: () => void
  clearWindowFunctions: () => void
  ctes: CTERow[]
  updateCTE: (i: number, field: keyof CTERow, v: string) => void
  removeCTE: (i: number) => void
  addCTE: () => void
  clearCtes: () => void
  toggleSummarize: () => void
  loading: boolean
  runQuery: () => void | Promise<void>
  sqlVisible: boolean
  onToggleSql: () => void | Promise<void>
  fieldLabelMode: 'technical' | 'human'
  t: MetadataTFunction
}) {
  return (
    <>
      <div className={qbNotebookClass}>
        <NotebookStep label="Data" themeClass="data">
          {sourceMode === 'metadata' ? (
            <span className={`${qbTagBase} ${qbTagBlueClass}`}>
              <Select
                value={baseTableKey}
                onChange={onBaseTableChange}
                options={tableOptions}
                size="sm"
                searchable
              />
            </span>
          ) : (
            <span className={`${qbTagBase} ${qbTagBlueClass}`}>{modelDetail.base_table}</span>
          )}
        </NotebookStep>
        <QueryBuilderNotebookJoins
          joins={modelDetail.joins ?? []}
          editable={metadataJoinsEditable}
          tableOptions={tableOptions}
          includedTableOptions={includedTableOptions}
          columnOptionsByTable={columnOptionsByTable}
          onAddJoin={onAddMetadataJoin}
          onUpdateJoin={onUpdateMetadataJoin}
          onRemoveJoin={onRemoveMetadataJoin}
          t={t}
        />
        <FilterStep
          filters={filters}
          filterFieldOpts={filterFieldOpts}
          updateFilter={updateFilter}
          removeFilter={removeFilter}
          addFilter={addFilter}
          onClear={() => setFilters([])}
        />
        {!isSummarized && (
          <FieldsStep
            selectItems={selectItems}
            dimensions={dimensions ?? []}
            metrics={metrics ?? []}
            updateSelectItem={updateSelectItem}
            removeSelectItem={removeSelectItem}
            addSelectItem={addSelectItem}
            dimFieldOptions={(dims) => dimFieldOptions(dims, fieldLabelMode)}
            metricFieldOptions={(mets) => metricFieldOptions(mets, fieldLabelMode)}
          />
        )}
        {isSummarized && (
          <SummarizeStep
            selectItems={selectItems}
            groupBy={groupBy}
            dimensions={dimensions ?? []}
            metrics={metrics ?? []}
            updateSelectItem={updateSelectItem}
            removeSelectItem={removeSelectItem}
            addMetricSelectItem={addMetricSelectItem}
            updateGroupByRow={updateGroupByRow}
            removeGroupByRow={removeGroupByRow}
            addGroupByRow={addGroupByRow}
            onClear={() => {
              setIsSummarized(false)
              setGroupBy([])
            }}
            metricFieldOptions={(mets) => metricFieldOptions(mets, fieldLabelMode)}
            dimOptionsForGroupRow={(dims, gb, index) =>
              dimOptionsForGroupRow(dims, gb, index, selectItems, fieldLabelMode)
            }
          />
        )}
        <SortStep
          orderBy={orderBy}
          orderDir={orderDir}
          orderByOpts={orderByOpts}
          setOrderBy={setOrderBy}
          setOrderDir={setOrderDir}
          onClear={() => setOrderBy('')}
        />
        <NotebookStep label="Row limit" themeClass="limit">
          <input
            type="number"
            min={1}
            inputMode="numeric"
            value={limit}
            onChange={(e) => setLimit(Number(e.target.value))}
            style={{ width: '6rem' }}
          />
        </NotebookStep>
        {mode === 'advanced' && (
          <HavingStep
            having={having}
            metricOptsHaving={metricOptsHaving}
            updateHaving={updateHaving}
            removeHaving={removeHaving}
            addHaving={addHaving}
            onClear={() => setHaving([])}
          />
        )}
        {mode === 'advanced' && (
          <WindowFuncStep
            windowFunctions={windowFunctions}
            updateWindowFunc={updateWindowFunc}
            removeWindowFunc={removeWindowFunc}
            addWindowFunc={addWindowFunc}
            onClear={clearWindowFunctions}
          />
        )}
        {mode === 'advanced' && (
          <CteStep
            ctes={ctes}
            updateCTE={updateCTE}
            removeCTE={removeCTE}
            addCTE={addCTE}
            onClear={clearCtes}
          />
        )}
      </div>
      <QueryBuilderNotebookToolbar
        filtersActive={filters.length > 0}
        isSummarized={isSummarized}
        orderByActive={Boolean(orderBy)}
        limit={limit}
        mode={mode}
        havingActive={having.length > 0}
        windowActive={windowFunctions.length > 0}
        cteActive={ctes.length > 0}
        onAddFilter={addFilter}
        onToggleSummarize={toggleSummarize}
        onAddHaving={addHaving}
        onAddWindowFunc={addWindowFunc}
        onAddCte={addCTE}
        onPickFirstSort={() => {
          if (!orderBy) {
            const firstOpt = orderByOpts.find((o) => o.value)
            if (firstOpt) {
              setOrderBy(firstOpt.value)
            }
          }
        }}
      />
      <QueryBuilderVisualizeFooter
        show
        loading={loading}
        onRun={() => {
          void runQuery()
        }}
        sqlVisible={sqlVisible}
        onToggleSql={() => {
          void onToggleSql()
        }}
        t={t}
      />
    </>
  )
}
