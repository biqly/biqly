import { useEffect, useState } from 'react'
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, LineChart, Line, PieChart, Pie, Cell } from 'recharts'
import { useApi } from '../hooks/useApi'
import { useQueryParam } from '../hooks/useQueryParam'
import { formatResultCell } from '../utils/resultCellFormat'
import { Select } from './ui/Select'
import type { FilterClause, GroupByField, OrderByField, CTE, LogicalQuery } from '../types/ai'

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
  const [dsParam, setDsParam] = useQueryParam('ds')
  const [datasourceId, setDatasourceId] = useState(dsParam)
  const [modelId, setModelId] = useState('')

  useEffect(() => {
    get<Datasource[]>('/api/datasources').then((data) => {
      if (!data) return
      setDatasources(data)
      setDatasourceId((prev) => {
        if (prev && data.some((d) => d.id === prev)) return prev
        return data[0]?.id ?? ''
      })
    })
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    setDsParam(datasourceId)
  }, [datasourceId, setDsParam])
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
      ...(mode === 'advanced' ? {
        select: [
          ...selectItems.filter((s) => s.name),
          ...windowFunctions.filter((w) => w.field).map((w) => ({
            type: 'window' as const,
            name: w.field,
            window: {
              aggregation: (w.func || 'row_number').toLowerCase(),
              expression: w.field,
              partition_by: w.partition_by ? w.partition_by.split(',').map((s) => s.trim()).filter(Boolean) : undefined,
              order_by: w.order_by ? [{ field: w.order_by, direction: 'asc' as const }] : undefined,
            },
          })),
        ],
      } : {
        select: selectItems.filter((s) => s.name),
      }),
      ctes: mode === 'advanced'
        ? ctes.filter((c) => c.name && c.query).map((c): CTE => ({
            name: c.name,
            query: { datasource_id: datasourceId, model_id: modelId, select: [], query_text: c.query } as unknown as LogicalQuery,
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
    <div className="page-stack">
      <div className="card">
        <div className="card-header-row card-header-row--spaced">
          <h2>Sorgu kurulumu</h2>
          <div className="toggle-group">
            <button className={`toggle-btn ${mode === 'simple' ? 'active' : ''}`} onClick={() => setMode('simple')}>Basit</button>
            <button className={`toggle-btn ${mode === 'advanced' ? 'active' : ''}`} onClick={() => setMode('advanced')}>Gelişmiş</button>
          </div>
        </div>
        <div style={{ display: 'flex', gap: '1rem' }}>
          <div className="form-group" style={{ flex: 1 }}>
            <label htmlFor="query-datasource">Veri kaynağı</label>
            <Select
              id="query-datasource"
              name="datasource"
              value={datasourceId}
              onChange={setDatasourceId}
              placeholder="— seçin —"
              header="Veri kaynakları"
              options={datasources.map((d) => ({ value: d.id, label: d.name, hint: d.type }))}
            />
          </div>
          <div className="form-group" style={{ flex: 1 }}>
            <label htmlFor="query-model">Anlamsal model</label>
            <input id="query-model" name="model_id" value={modelId} onChange={(e) => setModelId(e.target.value)} placeholder="ör. orders…" autoComplete="off" />
          </div>
        </div>

        <div className="form-group">
          <label>Seçim alanları</label>
          {selectItems.map((item, i) => (
            <div key={i} className="query-builder-row">
              <Select
                value={item.type}
                onChange={(v) => updateSelectItem(i, 'type', v)}
                ariaLabel={`Alan ${i + 1} türü`}
                options={[
                  { value: 'dimension', label: 'Boyut' },
                  { value: 'metric', label: 'Metrik' },
                ]}
              />
              <input value={item.name} onChange={(e) => updateSelectItem(i, 'name', e.target.value)} placeholder="alan adı…" aria-label={`Alan ${i + 1} adı`} autoComplete="off" />
              <button className="remove-btn" onClick={() => removeSelectItem(i)} aria-label={`Alan ${i + 1} kaldır`}>×</button>
            </div>
          ))}
          <button className="add-btn" onClick={addSelectItem}>+ Alan ekle</button>
        </div>

        <div className="form-group">
          <label>Filtreler</label>
          {filters.map((f, i) => (
            <div key={i} className="query-builder-row">
              <input value={f.field} onChange={(e) => updateFilter(i, 'field', e.target.value)} placeholder="alan…" aria-label={`Filtre ${i + 1} alan`} autoComplete="off" />
              <Select
                value={f.operator}
                onChange={(v) => updateFilter(i, 'operator', v)}
                ariaLabel={`Filtre ${i + 1} operatör`}
                options={[
                  { value: 'eq', label: '=' },
                  { value: 'neq', label: '!=' },
                  { value: 'gt', label: '>' },
                  { value: 'gte', label: '>=' },
                  { value: 'lt', label: '<' },
                  { value: 'lte', label: '<=' },
                  { value: 'contains', label: 'içerir' },
                  { value: 'in', label: 'içinde' },
                  { value: 'between', label: 'arasında' },
                ]}
              />
              <input value={f.value} onChange={(e) => updateFilter(i, 'value', e.target.value)} placeholder="değer…" aria-label={`Filtre ${i + 1} değer`} autoComplete="off" />
              <button className="remove-btn" onClick={() => removeFilter(i)} aria-label={`Filtre ${i + 1} kaldır`}>×</button>
            </div>
          ))}
          <button className="add-btn" onClick={addFilter}>+ Filtre ekle</button>
        </div>

        <div className="form-group">
          <label htmlFor="query-group-by">Grupla</label>
          <input id="query-group-by" name="group_by" value={groupBy.join(', ')} onChange={(e) => setGroupBy(e.target.value.split(',').map((s) => s.trim()))} placeholder="virgülle ayrılmış alanlar…" autoComplete="off" />
        </div>

        <div style={{ display: 'flex', gap: '1rem' }}>
          <div className="form-group" style={{ flex: 1 }}>
            <label htmlFor="query-order-by">Sırala</label>
            <input id="query-order-by" name="order_by" value={orderBy} onChange={(e) => setOrderBy(e.target.value)} placeholder="alan…" autoComplete="off" />
          </div>
          <div className="form-group" style={{ flex: 1 }}>
            <label htmlFor="query-order-direction">Yön</label>
            <Select
              id="query-order-direction"
              name="order_direction"
              value={orderDir}
              onChange={setOrderDir}
              options={[
                { value: 'asc', label: 'ASC' },
                { value: 'desc', label: 'DESC' },
              ]}
            />
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
              <summary>HAVING (özet sonrası filtre)</summary>
              <div className="form-group" style={{ marginTop: '0.5rem' }}>
                {having.map((h, i) => (
                  <div key={i} className="query-builder-row">
                    <input value={h.field} onChange={(e) => updateHaving(i, 'field', e.target.value)} placeholder="özetlenmiş alan…" aria-label={`Having ${i + 1} alan`} autoComplete="off" />
                    <Select
                      value={h.operator}
                      onChange={(v) => updateHaving(i, 'operator', v)}
                      ariaLabel={`Having ${i + 1} operator`}
                      options={[
                        { value: 'gt', label: '>' },
                        { value: 'gte', label: '>=' },
                        { value: 'lt', label: '<' },
                        { value: 'lte', label: '<=' },
                        { value: 'eq', label: '=' },
                        { value: 'neq', label: '!=' },
                      ]}
                    />
                    <input value={h.value} onChange={(e) => updateHaving(i, 'value', e.target.value)} placeholder="değer…" aria-label={`Having ${i + 1} değer`} autoComplete="off" />
                    <button className="remove-btn" onClick={() => removeHaving(i)} aria-label={`Having ${i + 1} kaldır`}>×</button>
                  </div>
                ))}
                <button className="add-btn" onClick={addHaving}>+ HAVING koşulu ekle</button>
              </div>
            </details>

            <details open={false}>
              <summary>Pencere fonksiyonları</summary>
              <div className="form-group" style={{ marginTop: '0.5rem' }}>
                {windowFunctions.map((w, i) => (
                  <div key={i} className="query-builder-row query-builder-row--wide">
                    <Select
                      value={w.func}
                      onChange={(v) => updateWindowFunc(i, 'func', v)}
                      ariaLabel={`Pencere ${i + 1} tür`}
                      options={WINDOW_FUNC_OPTIONS.map((opt) => ({ value: opt, label: opt }))}
                    />
                    <input value={w.field} onChange={(e) => updateWindowFunc(i, 'field', e.target.value)} placeholder="alan…" aria-label={`Pencere ${i + 1} alan`} autoComplete="off" />
                    <input value={w.partition_by} onChange={(e) => updateWindowFunc(i, 'partition_by', e.target.value)} placeholder="PARTITION BY (virgülle)" aria-label={`Pencere ${i + 1} bölüm`} autoComplete="off" />
                    <input value={w.order_by} onChange={(e) => updateWindowFunc(i, 'order_by', e.target.value)} placeholder="ORDER BY alanı" aria-label={`Pencere ${i + 1} sıra`} autoComplete="off" />
                    <button className="remove-btn" onClick={() => removeWindowFunc(i)} aria-label={`Pencere ${i + 1} kaldır`}>×</button>
                  </div>
                ))}
                <button className="add-btn" onClick={addWindowFunc}>+ Pencere fonksiyonu ekle</button>
              </div>
            </details>

            <details open={false}>
              <summary>Ortak tablo ifadeleri (CTE / WITH)</summary>
              <div className="form-group" style={{ marginTop: '0.5rem' }}>
                {ctes.map((c, i) => (
                  <div key={i} style={{ marginBottom: '0.75rem', padding: '0.5rem', border: '1px dashed var(--border)', borderRadius: '0.5rem' }}>
                    <div className="query-builder-row" style={{ marginBottom: '0.4rem' }}>
                      <input value={c.name} onChange={(e) => updateCTE(i, 'name', e.target.value)} placeholder="CTE adı…" aria-label={`CTE ${i + 1} ad`} autoComplete="off" style={{ gridColumn: '1 / -2' }} />
                      <button className="remove-btn" onClick={() => removeCTE(i)} aria-label={`CTE ${i + 1} kaldır`}>×</button>
                    </div>
                    <textarea value={c.query} onChange={(e) => updateCTE(i, 'query', e.target.value)} placeholder="Sorgu tanımı (ör. SELECT ... FROM ...)" rows={3} style={{ width: '100%', boxSizing: 'border-box' }} />
                  </div>
                ))}
                <button className="add-btn" onClick={addCTE}>+ CTE ekle</button>
              </div>
            </details>
          </>
        )}

        <button className="btn" onClick={runQuery} disabled={loading}>
          {loading ? 'Çalışıyor…' : 'Sorguyu çalıştır'}
        </button>

        {error && <div className="error">{error}</div>}
      </div>

      {sql && (
        <div className="card">
          <h2>Üretilen SQL</h2>
          <div className="sql-preview">{sql}</div>
        </div>
      )}

      {result && (
        <div className="card">
          {chartData.length > 0 ? (
            <div className="card-header-row card-header-row--spaced">
              <h2>Sonuçlar ({result.stats?.row_count || 0} satır, {result.stats?.duration_ms || 0} ms)</h2>
              <div className="toggle-group" role="group" aria-label="Grafik türü">
                <button type="button" className={`toggle-btn ${chartType === 'bar' ? 'active' : ''}`} onClick={() => setChartType('bar')}>Çubuk</button>
                <button type="button" className={`toggle-btn ${chartType === 'line' ? 'active' : ''}`} onClick={() => setChartType('line')}>Çizgi</button>
                <button type="button" className={`toggle-btn ${chartType === 'pie' ? 'active' : ''}`} onClick={() => setChartType('pie')}>Pasta</button>
              </div>
            </div>
          ) : (
            <h2>Sonuçlar ({result.stats?.row_count || 0} satır, {result.stats?.duration_ms || 0} ms)</h2>
          )}

          {chartData.length > 0 && (
            <>
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
