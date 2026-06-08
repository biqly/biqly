import { MultiSelect } from '../ui/MultiSelect'
import { routingTableLabel } from './routingPanelUtils'
import type { RoutingPanelProps } from './types'

export function RoutingPanelManualScope({
  t,
  datasourceId,
  tables,
  includeBaseTables,
  setIncludeBaseTables,
  includeViews,
  setIncludeViews,
  tableSearch,
  setTableSearch,
  selectedTables,
  setSelectedTables,
  filteredTables,
}: {
  t: RoutingPanelProps['t']
  datasourceId: string
  tables: RoutingPanelProps['tables']
  includeBaseTables: boolean
  setIncludeBaseTables: (value: boolean) => void
  includeViews: boolean
  setIncludeViews: (value: boolean) => void
  tableSearch: string
  setTableSearch: (value: string) => void
  selectedTables: string[]
  setSelectedTables: (value: string[]) => void
  filteredTables: RoutingPanelProps['tables']
}) {
  return (
    <div className="form-group">
      <span className="ai-scope-label">{t('ai_query.scope_label')}</span>
      <div className="ai-scope-type-filters" role="group">
        <label className="ai-scope-type-option">
          <input
            type="checkbox"
            checked={includeBaseTables}
            onChange={(e) => setIncludeBaseTables(e.target.checked)}
            disabled={!datasourceId || tables.length === 0}
          />
          <span>{t('ai_query.scope_base_tables')}</span>
        </label>
        <label className="ai-scope-type-option">
          <input
            type="checkbox"
            checked={includeViews}
            onChange={(e) => setIncludeViews(e.target.checked)}
            disabled={!datasourceId || tables.length === 0}
          />
          <span>{t('ai_query.scope_views')}</span>
        </label>
      </div>
      <input
        value={tableSearch}
        onChange={(e) => setTableSearch(e.target.value)}
        placeholder={t('ai_query.table_search_placeholder')}
        disabled={!datasourceId || tables.length === 0}
        autoComplete="off"
      />
      <MultiSelect
        display="inline"
        className="ai-scope-multiselect"
        ariaLabel={t('ai_query.selected_tables_aria')}
        value={selectedTables}
        onChange={setSelectedTables}
        disabled={!datasourceId || tables.length === 0 || (!includeBaseTables && !includeViews)}
        maxHeight={Math.min(288, Math.max(120, (filteredTables.length || 3) * 36))}
        options={filteredTables.map((table) => {
          const label = routingTableLabel(table)
          return { value: label, label }
        })}
      />
    </div>
  )
}
