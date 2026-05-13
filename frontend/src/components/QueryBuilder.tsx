import { useEffect, useMemo, useState } from 'react'
import { BarChart, Bar, XAxis, YAxis, CartesianGrid, Tooltip, ResponsiveContainer, LineChart, Line, PieChart, Pie, Cell } from 'recharts'
import { useApi } from '../hooks/useApi'
import { useArrayState } from '../hooks/useArrayState'
import { useQueryParam } from '../hooks/useQueryParam'
import type { Datasource } from '../types/metadata'
import { formatResultCell } from '../utils/resultCellFormat'
import { chartAxisStroke, chartGridStroke, chartTooltipStyle } from '../utils/chartConfig'
import { chartColor } from '../utils/constants'
import { Select } from './ui/Select'
import type { CTE, LogicalQuery } from '../types/ai'

interface SemanticModelSummary {
  id: string
  datasource_id: string
  name: string
  label?: string | null
  status: string
  base_schema: string
  base_table: string
}

interface SemanticDimension {
  id: string
  name: string
  label?: string | null
  type: string
}

interface SemanticMetric {
  id: string
  name: string
  label?: string | null
  aggregation: string
}

interface SemanticModelDetail {
  id: string
  name: string
  status: string
  dimensions?: SemanticDimension[]
  metrics?: SemanticMetric[]
}

function modelListLabel(m: SemanticModelSummary): string {
  if (m.label && m.label.trim()) return `${m.name} (${m.label})`
  return m.name
}

function modelListHint(m: SemanticModelSummary): string {
  const bits = [m.status, `${m.base_schema}.${m.base_table}`]
  return bits.join(' · ')
}

function dimFieldOptions(dims: SemanticDimension[]) {
  return dims.map((d) => ({
    value: d.name,
    label: d.label && d.label.trim() ? `${d.name} (${d.label})` : d.name,
    hint: d.type,
  }))
}

function metricFieldOptions(metrics: SemanticMetric[]) {
  return metrics.map((m) => ({
    value: m.name,
    label: m.label && m.label.trim() ? `${m.name} (${m.label})` : m.name,
    hint: m.aggregation,
  }))
}

function orderByFieldOptions(dims: SemanticDimension[], metrics: SemanticMetric[]) {
  const out: { value: string; label: string; hint: string }[] = []
  for (const d of dims) {
    out.push({
      value: d.name,
      label: d.label && d.label.trim() ? `${d.name} (${d.label})` : d.name,
      hint: `boyut · ${d.type}`,
    })
  }
  for (const m of metrics) {
    out.push({
      value: m.name,
      label: m.label && m.label.trim() ? `${m.name} (${m.label})` : m.name,
      hint: `metrik · ${m.aggregation}`,
    })
  }
  return out
}

function filterFieldOptions(dims: SemanticDimension[], metrics: SemanticMetric[]) {
  return orderByFieldOptions(dims, metrics)
}

function dimOptionsForGroupRow(
  dimensions: SemanticDimension[],
  groupBy: string[],
  rowIndex: number,
): { value: string; label: string; hint: string }[] {
  const chosenElsewhere = new Set(
    groupBy.filter((g, j) => j !== rowIndex && g !== '').map((g) => g),
  )
  return dimensions
    .filter((d) => !chosenElsewhere.has(d.name) || d.name === groupBy[rowIndex])
    .map((d) => ({
      value: d.name,
      label: d.label && d.label.trim() ? `${d.name} (${d.label})` : d.name,
      hint: d.type,
    }))
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

interface QueryBuilderResult {
  columns?: { name: string; type?: string }[]
  rows?: unknown[][]
  stats?: {
    row_count?: number
    duration_ms?: number
  }
}

interface QueryExplainResponse {
  compiled_sql?: string
}

const WINDOW_FUNC_OPTIONS = ['ROW_NUMBER', 'RANK', 'DENSE_RANK', 'LAG', 'LEAD', 'SUM', 'AVG', 'COUNT']

export default function QueryBuilder() {
  const { get, postData, loading, error } = useApi()
  const [datasources, setDatasources] = useState<Datasource[]>([])
  const [dsParam, setDsParam] = useQueryParam('ds')
  const [datasourceId, setDatasourceId] = useState(dsParam)
  const [modelId, setModelId] = useState('')
  const [models, setModels] = useState<SemanticModelSummary[]>([])
  const [modelDetail, setModelDetail] = useState<SemanticModelDetail | null>(null)

  useEffect(() => {
    get<Datasource[]>('/api/datasources').then((data) => {
      if (!data) return
      setDatasources(data)
      setDatasourceId((prev) => {
        if (prev && data.some((d) => d.id === prev)) return prev
        return data[0]?.id ?? ''
      })
    })
  }, [])

  useEffect(() => {
    setDsParam(datasourceId)
  }, [datasourceId, setDsParam])

  useEffect(() => {
    if (!datasourceId) {
      setModels([])
      return
    }
    let cancelled = false
    void get<SemanticModelSummary[]>(
      `/api/semantic/models?datasource_id=${encodeURIComponent(datasourceId)}`,
    ).then((data) => {
      if (!data || cancelled) return
      setModels(data)
      setModelId((prev) => {
        if (prev && data.some((m) => m.id === prev)) return prev
        const published = data.filter((m) => m.status === 'published')
        if (published.length > 0) return published[0]!.id
        return data[0]?.id ?? ''
      })
    })
    return () => {
      cancelled = true
    }
  }, [datasourceId])

  useEffect(() => {
    if (!modelId) {
      setModelDetail(null)
      return
    }
    let cancelled = false
    void get<SemanticModelDetail>(`/api/semantic/models/${encodeURIComponent(modelId)}`).then((d) => {
      if (!cancelled) setModelDetail(d ?? null)
    })
    return () => {
      cancelled = true
    }
  }, [modelId])

  const selectItemsState = useArrayState<SelectItem>([])
  const filterState = useArrayState<FilterRow>([])
  const groupByState = useArrayState<string>([])
  const { items: selectItems, setItems: setSelectItems } = selectItemsState
  const { items: filters } = filterState
  const { items: groupBy } = groupByState
  const [orderBy, setOrderBy] = useState<string>('')
  const [orderDir, setOrderDir] = useState('asc')
  const [limit, setLimit] = useState(100)
  const [offset, setOffset] = useState(0)
  const [mode, setMode] = useState<'simple' | 'advanced'>('simple')
  const havingState = useArrayState<HavingRow>([])
  const windowFunctionState = useArrayState<WindowFuncRow>([])
  const cteState = useArrayState<CTERow>([])
  const { items: having } = havingState
  const { items: windowFunctions } = windowFunctionState
  const { items: ctes } = cteState
  const [result, setResult] = useState<QueryBuilderResult | null>(null)
  const [sql, setSql] = useState('')
  const [chartType, setChartType] = useState<'bar' | 'line' | 'pie'>('bar')

  const dimensions = useMemo(() => modelDetail?.dimensions ?? [], [modelDetail])
  const metrics = useMemo(() => modelDetail?.metrics ?? [], [modelDetail])
  const filterFieldOpts = useMemo(() => filterFieldOptions(dimensions, metrics), [dimensions, metrics])
  const orderByOpts = useMemo(() => {
    const fields = orderByFieldOptions(dimensions, metrics)
    if (fields.length === 0) return []
    return [{ value: '', label: '(sıralama yok)', hint: '' }, ...fields]
  }, [dimensions, metrics])
  const metricOptsHaving = useMemo(() => metricFieldOptions(metrics), [metrics])

  const addSelectItem = () => selectItemsState.add({ type: 'dimension', name: '' })
  const updateSelectItem = (i: number, field: keyof SelectItem, value: string) => {
    const existing = selectItems[i]
    if (!existing) return
    if (field === 'type' && value !== existing.type) {
      selectItemsState.update(i, { type: value as 'dimension' | 'metric', name: '' })
    } else {
      selectItemsState.update(i, { ...existing, [field]: value })
    }
  }
  const removeSelectItem = (i: number) => selectItemsState.remove(i)

  const addFilter = () => filterState.add({ field: '', operator: 'eq', value: '' })
  const updateFilter = (i: number, field: keyof FilterRow, value: string) => {
    const existing = filters[i]
    filterState.update(i, { field: existing?.field ?? '', operator: existing?.operator ?? 'eq', value: existing?.value ?? '', [field]: value })
  }
  const removeFilter = (i: number) => filterState.remove(i)

  const addGroupByRow = () => groupByState.add('')
  const updateGroupByRow = (i: number, value: string) => groupByState.update(i, value)
  const removeGroupByRow = (i: number) => groupByState.remove(i)

  // HAVING helpers (advanced mode)
  const addHaving = () => havingState.add({ field: '', operator: 'gt', value: '' })
  const updateHaving = (i: number, field: keyof HavingRow, value: string) => {
    const existing = having[i]
    havingState.update(i, { field: existing?.field ?? '', operator: existing?.operator ?? 'gt', value: existing?.value ?? '', [field]: value })
  }
  const removeHaving = (i: number) => havingState.remove(i)

  // Window function helpers
  const addWindowFunc = () => windowFunctionState.add({ func: 'ROW_NUMBER', field: '', partition_by: '', order_by: '' })
  const updateWindowFunc = (i: number, field: keyof WindowFuncRow, value: string) => {
    const existing = windowFunctions[i]
    windowFunctionState.update(i, { func: existing?.func ?? 'ROW_NUMBER', field: existing?.field ?? '', partition_by: existing?.partition_by ?? '', order_by: existing?.order_by ?? '', [field]: value })
  }
  const removeWindowFunc = (i: number) => windowFunctionState.remove(i)

  // CTE helpers
  const addCTE = () => cteState.add({ name: '', query: '' })
  const updateCTE = (i: number, field: keyof CTERow, value: string) => {
    const existing = ctes[i]
    cteState.update(i, { name: existing?.name ?? '', query: existing?.query ?? '', [field]: value })
  }
  const removeCTE = (i: number) => cteState.remove(i)

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
    const explainRes = await postData<QueryExplainResponse>('/api/query/explain', payload)
    if (explainRes?.compiled_sql) {
      setSql(explainRes.compiled_sql)
    }

    // Then execute
    const res = await postData<QueryBuilderResult>('/api/query/run', payload)
    if (res) {
      setResult(res)
    }
  }

  const chartData = result?.rows?.map((row) => {
    const obj: { name: string; value?: number } = { name: String(row[0]) }
    if (row[1] !== undefined) obj.value = Number(row[1]) || 0
    return obj
  }) || []

  return (
    <div className="page-stack">
      <div className="card card--query-builder">
        <div className="card-header-row card-header-row--spaced">
          <h2>Sorgu kurulumu</h2>
          <div className="toggle-group">
            <button type="button" className={`toggle-btn ${mode === 'simple' ? 'active' : ''}`} onClick={() => setMode('simple')}>Basit</button>
            <button type="button" className={`toggle-btn ${mode === 'advanced' ? 'active' : ''}`} onClick={() => setMode('advanced')}>Gelişmiş</button>
          </div>
        </div>
        <div className="query-builder-inline-2">
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
            <Select
              id="query-model"
              name="model_id"
              value={modelId}
              onChange={setModelId}
              placeholder={models.length ? '— model seçin —' : 'Bu kaynakta model yok'}
              header="Anlamsal modeller"
              disabled={models.length === 0}
              options={models.map((m) => ({
                value: m.id,
                label: modelListLabel(m),
                hint: modelListHint(m),
              }))}
            />
            {modelId && models.find((m) => m.id === modelId)?.status !== 'published' ? (
              <p className="hint-text" style={{ marginTop: '0.35rem', fontSize: '0.82rem', color: 'var(--text-muted)' }}>
                Taslak model: sorgu yalnızca yayınlanmış anlamsal bağlamda derlenir. Gerekirse modeli yayınlayın.
              </p>
            ) : null}
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
              <Select
                value={item.name}
                onChange={(v) => updateSelectItem(i, 'name', v)}
                ariaLabel={`Alan ${i + 1} adı`}
                placeholder="— alan seçin —"
                header={item.type === 'dimension' ? 'Boyutlar' : 'Metrikler'}
                disabled={
                  !modelId
                  || (item.type === 'dimension' ? dimensions.length === 0 : metrics.length === 0)
                }
                options={item.type === 'dimension' ? dimFieldOptions(dimensions) : metricFieldOptions(metrics)}
              />
              <button type="button" className="remove-btn remove-btn--compact" onClick={() => removeSelectItem(i)} aria-label={`Alan ${i + 1} kaldır`}>×</button>
            </div>
          ))}
          <button type="button" className="add-btn" onClick={addSelectItem}>+ Alan ekle</button>
        </div>

        <div className="form-group">
          <label>Filtreler</label>
          {filters.map((f, i) => (
            <div key={i} className="query-builder-row query-builder-row--filter">
              <Select
                value={f.field}
                onChange={(v) => updateFilter(i, 'field', v)}
                ariaLabel={`Filtre ${i + 1} alan`}
                placeholder="— alan seçin —"
                header="Boyut veya metrik"
                disabled={!modelId || filterFieldOpts.length === 0}
                options={filterFieldOpts}
              />
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
              <button type="button" className="remove-btn remove-btn--compact" onClick={() => removeFilter(i)} aria-label={`Filtre ${i + 1} kaldır`}>×</button>
            </div>
          ))}
          <button type="button" className="add-btn" onClick={addFilter}>+ Filtre ekle</button>
        </div>

        <div className="form-group">
          <label>Grupla</label>
          <p style={{ margin: '0 0 0.4rem', fontSize: '0.82rem', color: 'var(--text-muted)' }}>
            Yalnızca boyut alanları. Aynı alanı iki kez eklemeyin.
          </p>
          {groupBy.map((g, i) => (
            <div key={i} className="query-builder-row query-builder-row--group">
              <Select
                value={g}
                onChange={(v) => updateGroupByRow(i, v)}
                ariaLabel={`Gruplama ${i + 1}`}
                placeholder="— boyut seçin —"
                header="Boyutlar"
                disabled={!modelId || dimensions.length === 0}
                options={dimOptionsForGroupRow(dimensions, groupBy, i)}
              />
              <button type="button" className="remove-btn remove-btn--compact" onClick={() => removeGroupByRow(i)} aria-label={`Gruplama ${i + 1} kaldır`}>×</button>
            </div>
          ))}
          <button type="button" className="add-btn" onClick={addGroupByRow}>+ Gruplama alanı</button>
        </div>

        <div className="query-builder-inline-2">
          <div className="form-group" style={{ flex: 1 }}>
            <label htmlFor="query-order-by">Sırala</label>
            <Select
              id="query-order-by"
              name="order_by"
              value={orderBy}
              onChange={setOrderBy}
              placeholder="— alan seçin —"
              header="Boyut veya metrik"
              disabled={!modelId || orderByOpts.length === 0}
              options={orderByOpts}
            />
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
          <div className="query-builder-advanced">
            <details className="query-builder-panel">
              <summary>HAVING (özet sonrası filtre)</summary>
              <div className="query-builder-panel__body">
                <div className="form-group query-builder-panel__fields">
                  {having.map((h, i) => (
                    <div key={i} className="query-builder-row query-builder-row--filter">
                      <Select
                        value={h.field}
                        onChange={(v) => updateHaving(i, 'field', v)}
                        ariaLabel={`Having ${i + 1} alan`}
                        placeholder="— metrik seçin —"
                        header="Metrikler (özet sonrası)"
                        disabled={!modelId || metricOptsHaving.length === 0}
                        options={metricOptsHaving}
                      />
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
                      <button type="button" className="remove-btn remove-btn--compact" onClick={() => removeHaving(i)} aria-label={`Having ${i + 1} kaldır`}>×</button>
                    </div>
                  ))}
                  <button type="button" className="add-btn" onClick={addHaving}>+ HAVING koşulu ekle</button>
                </div>
              </div>
            </details>

            <details className="query-builder-panel">
              <summary>Pencere fonksiyonları</summary>
              <div className="query-builder-panel__body">
                <div className="form-group query-builder-panel__fields">
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
                      <button type="button" className="remove-btn remove-btn--compact" onClick={() => removeWindowFunc(i)} aria-label={`Pencere ${i + 1} kaldır`}>×</button>
                    </div>
                  ))}
                  <button type="button" className="add-btn" onClick={addWindowFunc}>+ Pencere fonksiyonu ekle</button>
                </div>
              </div>
            </details>

            <details className="query-builder-panel">
              <summary>Ortak tablo ifadeleri (CTE / WITH)</summary>
              <div className="query-builder-panel__body">
                <div className="form-group query-builder-panel__fields">
                  {ctes.map((c, i) => (
                    <div key={i} className="query-builder-cte-card">
                      <div className="query-builder-row query-builder-row--cte-head">
                        <input value={c.name} onChange={(e) => updateCTE(i, 'name', e.target.value)} placeholder="CTE adı…" aria-label={`CTE ${i + 1} ad`} autoComplete="off" />
                        <button type="button" className="remove-btn remove-btn--compact" onClick={() => removeCTE(i)} aria-label={`CTE ${i + 1} kaldır`}>×</button>
                      </div>
                      <textarea
                        className="query-builder-cte-textarea"
                        value={c.query}
                        onChange={(e) => updateCTE(i, 'query', e.target.value)}
                        placeholder="Sorgu tanımı (ör. SELECT ... FROM ...)"
                        rows={3}
                      />
                    </div>
                  ))}
                  <button type="button" className="add-btn" onClick={addCTE}>+ CTE ekle</button>
                </div>
              </div>
            </details>
          </div>
        )}

        <div className="query-builder-footer">
          <button type="button" className="btn btn-sm" onClick={runQuery} disabled={loading}>
            {loading ? 'Çalışıyor…' : 'Sorguyu çalıştır'}
          </button>
        </div>

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
                      <CartesianGrid strokeDasharray="3 3" stroke={chartGridStroke} />
                      <XAxis dataKey="name" stroke={chartAxisStroke} />
                      <YAxis stroke={chartAxisStroke} />
                      <Tooltip contentStyle={chartTooltipStyle} />
                      <Bar dataKey="value" fill="#3b82f6" />
                    </BarChart>
                  ) : chartType === 'line' ? (
                    <LineChart data={chartData}>
                      <CartesianGrid strokeDasharray="3 3" stroke={chartGridStroke} />
                      <XAxis dataKey="name" stroke={chartAxisStroke} />
                      <YAxis stroke={chartAxisStroke} />
                      <Tooltip contentStyle={chartTooltipStyle} />
                      <Line type="monotone" dataKey="value" stroke="#3b82f6" strokeWidth={2} />
                    </LineChart>
                  ) : (
                    <PieChart>
                      <Pie data={chartData} dataKey="value" nameKey="name" cx="50%" cy="50%" outerRadius={100} label>
                        {chartData.map((_, i) => (
                          <Cell key={i} fill={chartColor(i)} />
                        ))}
                      </Pie>
                      <Tooltip contentStyle={chartTooltipStyle} />
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
          )}
        </div>
      )}
    </div>
  )
}
