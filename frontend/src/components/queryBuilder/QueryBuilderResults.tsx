import { cardClass, cardHeaderRowClass } from '../../lib/cardClasses'
import { legacyTableClass } from '../../lib/tableClasses'
import type { rowsToChartData } from '../../utils/chartData'
import { formatResultCell } from '../../utils/resultCellFormat'
import type { MetadataTFunction } from '../metadata/utils'
import { ChartContainer, type ChartKind } from '../ui/ChartContainer'
import { ChartTypeSelector } from '../ui/ChartTypeSelector'
import { Pagination } from '../ui/Pagination'
interface QueryBuilderResult {
  columns?: { name: string; type?: string }[]
  rows?: unknown[][]
  stats?: {
    row_count?: number
    duration_ms?: number
    total_count?: number
  }
}

export function QueryBuilderResults({
  result,
  chartData,
  chartType,
  setChartType,
  page,
  pageSize,
  onPageChange,
  loading,
  t,
}: {
  result: QueryBuilderResult
  chartData: ReturnType<typeof rowsToChartData>
  chartType: ChartKind
  setChartType: (value: ChartKind) => void
  page: number
  pageSize: number
  onPageChange: (page: number) => void
  loading: boolean
  t: MetadataTFunction
}) {
  const totalCount = result.stats?.total_count ?? 0
  const totalPages = totalCount > 0 ? Math.ceil(totalCount / pageSize) : 0
  const title = t('query_builder.results_title', {
    rows: result.stats?.row_count ?? 0,
    ms: result.stats?.duration_ms ?? 0,
  })

  return (
    <div className={cardClass()}>
      {chartData.length > 0 ? (
        <div className={cardHeaderRowClass}>
          <h2>{title}</h2>
          <ChartTypeSelector
            value={chartType}
            onChange={setChartType}
            variant="group"
            ariaLabel={t('ai_query.chart_type_aria')}
            labels={{
              bar: t('ai_query.chart_bar'),
              line: t('ai_query.chart_line'),
              pie: t('ai_query.chart_pie'),
              table: t('ai_query.chart_table'),
            }}
          />
        </div>
      ) : (
        <h2>{title}</h2>
      )}

      {chartData.length > 0 && <ChartContainer data={chartData} type={chartType} />}

      {result.columns && result.rows && (
        <div className={legacyTableClass('results-table-scroll')}>
          <table className={legacyTableClass('results-table')}>
            <thead>
              <tr>
                {result.columns.map((col) => (
                  <th key={col.name}>{col.name}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {result.rows.map((row, i) => (
                <tr key={i}>
                  {row.map((cell, j) => (
                    <td key={j}>{formatResultCell(cell, result.columns?.[j]?.name ?? '', {})}</td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {totalCount > 0 && totalPages > 1 && (
        <Pagination
          currentPage={page}
          totalPages={totalPages}
          onPageChange={onPageChange}
          totalItems={totalCount}
          itemsPerPage={pageSize}
        />
      )}

      {/* Loading indicator for page changes */}
      {loading && (
        <div className="text-foreground-faint flex items-center justify-center py-3 text-[0.8rem]">
          {t('common.loading')}
        </div>
      )}
    </div>
  )
}
