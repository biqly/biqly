import { legacyCardClass } from '../../lib/cardClasses'
import { legacyTableClass } from '../../lib/tableClasses'
import type { rowsToChartData } from '../../utils/chartData'
import { formatResultCell } from '../../utils/resultCellFormat'
import type { MetadataTFunction } from '../metadata/utils'
import { ChartContainer, type ChartKind } from '../ui/ChartContainer'
import { ChartTypeSelector } from '../ui/ChartTypeSelector'
interface QueryBuilderResult {
  columns?: { name: string; type?: string }[]
  rows?: unknown[][]
  stats?: {
    row_count?: number
    duration_ms?: number
  }
}

export function QueryBuilderResults({
  result,
  chartData,
  chartType,
  setChartType,
  t,
}: {
  result: QueryBuilderResult
  chartData: ReturnType<typeof rowsToChartData>
  chartType: ChartKind
  setChartType: (value: ChartKind) => void
  t: MetadataTFunction
}) {
  const title = t('query_builder.results_title', {
    rows: result.stats?.row_count ?? 0,
    ms: result.stats?.duration_ms ?? 0,
  })

  return (
    <div className={legacyCardClass('card')}>
      {chartData.length > 0 ? (
        <div className={legacyCardClass('card-header-row card-header-row--spaced')}>
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
    </div>
  )
}
