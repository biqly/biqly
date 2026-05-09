import { useEffect, useState } from 'react'
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, LineChart, Line, PieChart, Pie, Cell } from 'recharts'
import { useApi } from '../hooks/useApi'

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

const COLORS = ['#3b82f6', '#22c55e', '#f59e0b', '#ef4444', '#8b5cf6', '#ec4899', '#14b8a6', '#f97316']

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
      order_by: orderBy ? [{ field: orderBy, direction: orderDir }] : [],
      limit: parseInt(String(limit)) || 100,
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
        <h2>Query Setup</h2>
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
        </div>

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
                      <td key={j}>{String(cell)}</td>
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
