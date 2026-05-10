import { useEffect, useState } from 'react'
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, LineChart, Line, PieChart, Pie, Cell } from 'recharts'
import { useApi } from '../hooks/useApi'
import { formatResultCell } from '../utils/resultCellFormat'
import type { FilterClause, GroupByField, OrderByField, WindowFunction, CTE, LogicalQuery } from '../types/ai'

interface Datasource {
  id: string
  name: string
  type: string
}

interface FilterRow {
  field: string
  operator: string
  value: string
}

interface SelectItem {
  type: 'dimension' | 'metric'
  name: string
}

interface HavingRow {
  field: string
  operator: string
  value: string
}

interface WindowFuncRow {
  func: string
  field: string
  partition_by: string
  order_by: string
}

interface CTERow {
  name: string
  query: string
}

const COLORS = ['#3b82f6', '#22c55e', '#f59e0b', '#ef4444', '#8b5cf6', '#ec4899', '#14b8a6', '#f97316']

const WINDOW_FUNC_OPTIONS = ['ROW_NUMBER', 'RANK', 'DENSE_RANK', 'LAG', 'LEAD', 'SUM', 'AVG', 'COUNT']

export default function QueryBuilder() {
  const { get, postData, loading, error } = useApi()
  const [datasources, setDatasources] = useState<Datasource[]>([])
  const [datasourceId, setDatasourceId] = useState('')
  const [modelId, setModelId] = useState('')

  useEffect(() => {
    get<Datasource[]>('/api/datasources').then((data) => {
      if (data) {
        setDatasources(data)
        if (data[0]) setDatasourceId(data[0].id)
      }
    })
  }, []) // eslint-disable-line react-hooks/exhaustive-deps
  const [selectItems, setSelectItems] = useState<SelectItem[]>([])
  const [filters, setFilters] = useState<FilterRow[]>([])
  const [groupBy, setGroupBy] = useState<string[]>([])
  const [orderBy, setOrderBy] = useState<string>('')
  const [orderDir, setOrderDir] = useState('asc')
  const [limit, setLimit] = useState(100)
  const [offset, setOffset] = useState(0)
  const [mode, setMode] = useState<'simple' | 'advanced'>('simple')
  const [having, setHaving] = useState<HavingRow[]>([])
  const [windowFunctions, setWindowFunctions] = useState<WindowFuncRow[]>([])
  const [ctes, setCTEs] = useState<CTERow[]>([])
  const [result, setResult] = useState<any>(null)
  const [sql, setSql] = useState('')
  const [chartType, setChartType] = useState<'bar' | 'line' | 'pie'>('bar')

  const addSelectItem = () => setSelectItems([...selectItems, { type: 'dimension', name: '' }])
  const updateSelectItem = (i: number, field: keyof SelectItem, value: string) => {
    const items = [...selectItems]
    const existing = items[i]
    items[i] = { type: existing?.type ?? 'dimension', name: existing?.name ?? '', [field]: value }
    setSelectItems(items)
  }
  const removeSelectItem = (i: number) => setSelectItems(selectItems.filter((_, idx) => idx !== i))

  const addFilter = () => setFilters([...filters, { field: '', operator: 'eq', value: '' }])
  const updateFilter = (i: number, field: keyof FilterRow, value: string) => {
    const items = [...filters]
    const existing = items[i]
    items[i] = { field: existing?.field ?? '', operator: existing?.operator ?? 'eq', value: existing?.value ?? '', [field]: value }
    setFilters(items)
  }
  const removeFilter = (i: number) => setFilters(filters.filter((_, idx) => idx !== i))

  // HAVING helpers (advanced mode)
  const addHaving = () => setHaving([...having, { field: '', operator: 'gt', value: '' }])
  const updateHaving = (i: number, field: keyof HavingRow, value: string) => {
    const items = [...having]
    const existing = items[i]
    items[i] = { field: existing?.field ?? '', operator: existing?.operator ?? 'gt', value: existing?.value ?? '', [field]: value }
    setHaving(items)
  }
  const removeHaving = (i: number) => setHaving(having.filter((_, idx) => idx !== i))

  // Window function helpers
  const addWindowFunc = () => setWindowFunctions([...windowFunctions, { func: 'ROW_NUMBER', field: '', partition_by: '', order_by: '' }])
  const updateWindowFunc = (i: number, field: keyof WindowFuncRow, value: string) => {
    const items = [...windowFunctions]
    const existing = items[i]
    items[i] = { func: existing?.func ?? 'ROW_NUMBER', field: existing?.field ?? '', partition_by: existing?.partition_by ?? '', order_by: existing?.order_by ?? '', [field]: value }
    setWindowFunctions(items)
  }
  const removeWindowFunc = (i: number) => setWindowFunctions(windowFunctions.filter((_, idx) => idx !== i))

  // CTE helpers
  const addCTE = () => setCTEs([...ctes, { name: '', query: '' }])
  const updateCTE = (i: number, field: keyof CTERow, value: string) => {
    const items = [...ctes]
    const existing = items[i]
    items[i] = { name: existing?.name ?? '', query: existing?.query ?? '', [field]: value }
    setCTEs(items)
  }
  const removeCTE = (i: number) => setCTEs(ctes.filter((_, idx) => idx !== i))

  const runQuery = async () => {
    const payload = {
      datasource_id: datasourceId,
      model_id: modelId,
      select: selectItems.filter((s) => s.name),
      filters: filters.filter((f) => f.field && f.value).map((f) => ({
        field: f.field,
        operator: f.operator,
        value: f.value,
      })),
      group_by: groupBy.filter(Boolean).map((g) => ({ field: g })),
      having: mode === 'advanced'
        ? having.filter((h) => h.field && h.value).map((h) => ({
            field: h.field,
            operator: h.operator,
            value: h.value,
          }))
        : undefined,
      order_by: orderBy ? [{ field: orderBy, direction: orderDir }] : [],
      limit: parseInt(String(limit)) || 100,
      offset: mode === 'advanced' ? parseInt(String(offset)) || 0 : undefined,
      window_functions: mode === 'advanced'
        ? windowFunctions.filter((w) => w.field).map((w): WindowFunction => ({
            function: w.func as WindowFunction['function'],
            field: w.field,
            partition_by: w.partition_by ? w.partition_by.split(',').map((s) => s.trim()).filter(Boolean) : undefined,
            order_by: w.order_by ? [{ field: w.order_by, direction: 'asc' as const }] : undefined,
          }))
        : undefined,
      ctes: mode === 'advanced'
        ? ctes.filter((c) => c.name && c.query).map((c): CTE => ({
            name: c.name,
            query: { data_source: datasourceId, tables: [], select: [], query_text: c.query } as unknown as LogicalQuery,
          }))
        : undefined,
    }

    // First get SQL preview
    const explainRes = await postData('/api/query/explain', payload)
    if (explainRes?.compiled_sql) {
      setSql(explainRes.compiled_sql)
    }

    // Then execute
    const res = await postData('/api/query/run', payload)
    if (res) {
      setResult(res)
    }
  }

  const chartData = result?.rows?.map((row: any[]) => {
    const obj: Record<string, any> = { name: String(row[0]) }
    if (row[1] !== undefined) obj.value = Number(row[1]) || 0
    return obj
  }) || []

  return (
    <div>
      <div className="card">
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
          <h2 style={{ margin: 0 }}>Query Setup</h2>
          <div className="toggle-group">
            <button className={`toggle-btn ${mode === 'simple' ? 'active' : ''}`} onClick={() => setMode('simple')}>Simple</button>
            <button className={`toggle-btn ${mode === 'advanced' ? 'active' : ''}`} onClick={() => setMode('advanced')}>Advanced</button>
          </div>
        </div>
        <div style={{ display: 'flex', gap: '1rem' }}>
          <div className="form-group" style={{ flex: 1 }}>
            <label htmlFor="query-datasource">Datasource</label>
            <select id="query-datasource" name="datasource" value={datasourceId} onChange={(e) => setDatasourceId(e.target.value)}>
              <option value="">— select —</option>
              {datasources.map((d) => (
                <option key={d.id} value={d.id}>{d.name}</option>
              ))}
            </select>
          </div>
          <div className="form-group" style={{ flex: 1 }}>
            <label htmlFor="query-model">Semantic Model</label>
            <input id="query-model" name="model_id" value={modelId} onChange={(e) => setModelId(e.target.value)} placeholder="e.g. orders…" autoComplete="off" />
          </div>
        </div>

        <div className="form-group">
          <label>Select Fields</label>
          {selectItems.map((item, i) => (
            <div key={i} className="query-builder-row">
              <select value={item.type} onChange={(e) => updateSelectItem(i, 'type', e.target.value)} aria-label={`Field ${i + 1} type`}>
                <option value="dimension">Dimension</option>
                <option value="metric">Metric</option>
              </select>
              <input value={item.name} onChange={(e) => updateSelectItem(i, 'name', e.target.value)} placeholder="field name…" aria-label={`Field ${i + 1} name`} autoComplete="off" />
              <button className="remove-btn" onClick={() => removeSelectItem(i)} aria-label={`Remove field ${i + 1}`}>×</button>
            </div>
          ))}
          <button className="add-btn" onClick={addSelectItem}>+ Add Field</button>
        </div>

        <div className="form-group">
          <label>Filters</label>
          {filters.map((f, i) => (
            <div key={i} className="query-builder-row">
              <input value={f.field} onChange={(e) => updateFilter(i, 'field', e.target.value)} placeholder="field…" aria-label={`Filter ${i + 1} field`} autoComplete="off" />
              <select value={f.operator} onChange={(e) => updateFilter(i, 'operator', e.target.value)} aria-label={`Filter ${i + 1} operator`}>
                <option value="eq">=</option>
                <option value="neq">!=</option>
                <option value="gt">&gt;</option>
                <option value="gte">&gt;=</option>
                <option value="lt">&lt;</option>
                <option value="lte">&lt;=</option>
                <option value="contains">contains</option>
                <option value="in">in</option>
                <option value="between">between</option>
              </select>
              <input value={f.value} onChange={(e) => updateFilter(i, 'value', e.target.value)} placeholder="value…" aria-label={`Filter ${i + 1} value`} autoComplete="off" />
              <button className="remove-btn" onClick={() => removeFilter(i)} aria-label={`Remove filter ${i + 1}`}>×</button>
            </div>
          ))}
          <button className="add-btn" onClick={addFilter}>+ Add Filter</button>
        </div>

        <div className="form-group">
          <label htmlFor="query-group-by">Group By</label>
          <input id="query-group-by" name="group_by" value={groupBy.join(', ')} onChange={(e) => setGroupBy(e.target.value.split(',').map((s) => s.trim()))} placeholder="comma-separated fields…" autoComplete="off" />
        </div>

        <div style={{ display: 'flex', gap: '1rem' }}>
          <div className="form-group" style={{ flex: 1 }}>
            <label htmlFor="query-order-by">Order By</label>
            <input id="query-order-by" name="order_by" value={orderBy} onChange={(e) => setOrderBy(e.target.value)} placeholder="field…" autoComplete="off" />
          </div>
          <div className="form-group" style={{ flex: 1 }}>
            <label htmlFor="query-order-direction">Direction</label>
            <select id="query-order-direction" name="order_direction" value={orderDir} onChange={(e) => setOrderDir(e.target.value)}>
              <option value="asc">ASC</option>
              <option value="desc">DESC</option>
            </select>
          </div>
          <div className="form-group" style={{ flex: 1 }}>
            <label htmlFor="query-limit">Limit</label>
            <input id="query-limit" name="limit" type="number" min={1} inputMode="numeric" value={limit} onChange={(e) => setLimit(Number(e.target.value))} />
          </div>
          {mode === 'advanced' && (
            <div className="form-group" style={{ flex: 1 }}>
              <label htmlFor="query-offset">Offset</label>
              <input id="query-offset" name="offset" type="number" min={0} inputMode="numeric" value={offset} onChange={(e) => setOffset(Number(e.target.value))} />
            </div>
          )}
        </div>

        {/* ─── Advanced Mode Sections ─────────────────────────── */}
        {mode === 'advanced' && (
          <>
            <details open={false}>
              <summary>HAVING Clause</summary>
              <div className="form-group" style={{ marginTop: '0.5rem' }}>
                {having.map((h, i) => (
                  <div key={i} className="query-builder-row">
                    <input value={h.field} onChange={(e) => updateHaving(i, 'field', e.target.value)} placeholder="aggregated field…" aria-label={`Having ${i + 1} field`} autoComplete="off" />
                    <select value={h.operator} onChange={(e) => updateHaving(i, 'operator', e.target.value)} aria-label={`Having ${i + 1} operator`}>
                      <option value="gt">&gt;</option>
                      <option value="gte">&gt;=</option>
                      <option value="lt">&lt;</option>
                      <option value="lte">&lt;=</option>
                      <option value="eq">=</option>
                      <option value="neq">!=</option>
                    </select>
                    <input value={h.value} onChange={(e) => updateHaving(i, 'value', e.target.value)} placeholder="value…" aria-label={`Having ${i + 1} value`} autoComplete="off" />
                    <button className="remove-btn" onClick={() => removeHaving(i)} aria-label={`Remove having ${i + 1}`}>×</button>
                  </div>
                ))}
                <button className="add-btn" onClick={addHaving}>+ Add HAVING Condition</button>
              </div>
            </details>

            <details open={false}>
              <summary>Window Functions</summary>
              <div className="form-group" style={{ marginTop: '0.5rem' }}>
                {windowFunctions.map((w, i) => (
                  <div key={i} className="query-builder-row query-builder-row--wide">
                    <select value={w.func} onChange={(e) => updateWindowFunc(i, 'func', e.target.value)} aria-label={`Window func ${i + 1} type`}>
                      {WINDOW_FUNC_OPTIONS.map((opt) => <option key={opt} value={opt}>{opt}</option>)}
                    </select>
                    <input value={w.field} onChange={(e) => updateWindowFunc(i, 'field', e.target.value)} placeholder="field…" aria-label={`Window func ${i + 1} field`} autoComplete="off" />
                    <input value={w.partition_by} onChange={(e) => updateWindowFunc(i, 'partition_by', e.target.value)} placeholder="PARTITION BY (comma-sep)" aria-label={`Window func ${i + 1} partition`} autoComplete="off" />
                    <input value={w.order_by} onChange={(e) => updateWindowFunc(i, 'order_by', e.target.value)} placeholder="ORDER BY field" aria-label={`Window func ${i + 1} order`} autoComplete="off" />
                    <button className="remove-btn" onClick={() => removeWindowFunc(i)} aria-label={`Remove window func ${i + 1}`}>×</button>
                  </div>
                ))}
                <button className="add-btn" onClick={addWindowFunc}>+ Add Window Function</button>
              </div>
            </details>

            <details open={false}>
              <summary>Common Table Expressions (CTEs)</summary>
              <div className="form-group" style={{ marginTop: '0.5rem' }}>
                {ctes.map((c, i) => (
                  <div key={i} style={{ marginBottom: '0.75rem', padding: '0.5rem', border: '1px dashed var(--border)', borderRadius: '0.5rem' }}>
                    <div className="query-builder-row" style={{ marginBottom: '0.4rem' }}>
                      <input value={c.name} onChange={(e) => updateCTE(i, 'name', e.target.value)} placeholder="CTE name…" aria-label={`CTE ${i + 1} name`} autoComplete="off" style={{ gridColumn: '1 / -2' }} />
                      <button className="remove-btn" onClick={() => removeCTE(i)} aria-label={`Remove CTE ${i + 1}`}>×</button>
                    </div>
                    <textarea value={c.query} onChange={(e) => updateCTE(i, 'query', e.target.value)} placeholder="Query definition (e.g. SELECT ... FROM ...)" rows={3} style={{ width: '100%', boxSizing: 'border-box' }} />
                  </div>
                ))}
                <button className="add-btn" onClick={addCTE}>+ Add CTE</button>
              </div>
            </details>
          </>
        )}

        <button className="btn" onClick={runQuery} disabled={loading}>
          {loading ? 'Running…' : 'Run Query'}
        </button>

        {error && <div className="error">{error}</div>}
      </div>

      {sql && (
        <div className="card">
          <h2>Generated SQL</h2>
          <div className="sql-preview">{sql}</div>
        </div>
      )}

      {result && (
        <div className="card">
          <h2>Results ({result.stats?.row_count || 0} rows, {result.stats?.duration_ms || 0}ms)</h2>

          {chartData.length > 0 && (
            <>
              <div style={{ marginBottom: '1rem', display: 'flex', gap: '0.5rem' }}>
                <button className={chartType === 'bar' ? 'btn' : ''} onClick={() => setChartType('bar')}>Bar</button>
                <button className={chartType === 'line' ? 'btn' : ''} onClick={() => setChartType('line')}>Line</button>
                <button className={chartType === 'pie' ? 'btn' : ''} onClick={() => setChartType('pie')}>Pie</button>
              </div>
              <div className="chart-container" style={{ height: 300 }}>
                <ResponsiveContainer width="100%" height="100%">
                  {chartType === 'bar' ? (
                    <BarChart data={chartData}>
                      <CartesianGrid strokeDasharray="3 3" stroke="#475569" />
                      <XAxis dataKey="name" stroke="#94a3b8" />
                      <YAxis stroke="#94a3b8" />
                      <Tooltip contentStyle={{ background: '#1e293b', border: '1px solid #475569' }} />
                      <Bar dataKey="value" fill="#3b82f6" />
                    </BarChart>
                  ) : chartType === 'line' ? (
                    <LineChart data={chartData}>
                      <CartesianGrid strokeDasharray="3 3" stroke="#475569" />
                      <XAxis dataKey="name" stroke="#94a3b8" />
                      <YAxis stroke="#94a3b8" />
                      <Tooltip contentStyle={{ background: '#1e293b', border: '1px solid #475569' }} />
                      <Line type="monotone" dataKey="value" stroke="#3b82f6" strokeWidth={2} />
                    </LineChart>
                  ) : (
                    <PieChart>
                      <Pie data={chartData} dataKey="value" nameKey="name" cx="50%" cy="50%" outerRadius={100} label>
                        {chartData.map((_: any, i: number) => (
                          <Cell key={i} fill={COLORS[i % COLORS.length]} />
                        ))}
                      </Pie>
                      <Tooltip contentStyle={{ background: '#1e293b', border: '1px solid #475569' }} />
                    </PieChart>
                  )}
                </ResponsiveContainer>
              </div>
            </>
          )}

          {result.columns && result.rows && (
            <table className="results-table">
              <thead>
                <tr>
                  {result.columns.map((col: any) => (
                    <th key={col.name}>{col.name}</th>
                  ))}
                </tr>
              </thead>
              <tbody>
                {result.rows.map((row: any[], i: number) => (
                  <tr key={i}>
                    {row.map((cell: any, j: number) => (
                      <td key={j}>{formatResultCell(cell, result.columns[j]?.name ?? '', {})}</td>
                    ))}
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      )}
    </div>
  )
}
