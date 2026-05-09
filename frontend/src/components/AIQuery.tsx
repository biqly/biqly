import { useEffect, useState } from 'react'
import { useApi } from '../hooks/useApi'

interface Datasource {
  id: string
  name: string
  type: string
}

interface TableOption {
  schema_name: string
  table_name: string
  description?: string | null
  label?: string
}

export default function AIQuery() {
  const { get, postData, loading, error } = useApi()
  const [datasources, setDatasources] = useState<Datasource[]>([])
  const [tables, setTables] = useState<TableOption[]>([])
  const [datasourceId, setDatasourceId] = useState('')
  const [selectedTables, setSelectedTables] = useState<string[]>([])
  const [tableSearch, setTableSearch] = useState('')
  const [question, setQuestion] = useState('')
  const [result, setResult] = useState<any>(null)

  useEffect(() => {
    get<Datasource[]>('/api/datasources').then((data) => {
      if (data) {
        setDatasources(data)
        if (data[0]) setDatasourceId(data[0].id)
      }
    })
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    setSelectedTables([])
    setTableSearch('')
    setTables([])
    if (!datasourceId) return

    get<TableOption[]>(`/api/datasources/${datasourceId}/tables`).then((data) => {
      setTables(data || [])
    })
  }, [datasourceId]) // eslint-disable-line react-hooks/exhaustive-deps

  const tableLabel = (table: TableOption) => table.label || `${table.schema_name}.${table.table_name}`

  const filteredTables = tables.filter((table) => {
    const search = tableSearch.trim().toLowerCase()
    if (!search) return true
    return (
      tableLabel(table).toLowerCase().includes(search) ||
      (table.description || '').toLowerCase().includes(search)
    )
  })

  const requestBody = () => ({
    datasource_id: datasourceId,
    question,
    tables: selectedTables,
  })

  const runAIQuery = async () => {
    const res = await postData('/api/ai/query/preview', requestBody())
    if (res) setResult(res)
  }

  const runAndExecute = async () => {
    const res = await postData('/api/ai/query/run', requestBody())
    if (res) setResult(res)
  }

  return (
    <div>
      <div className="card">
        <h2>Natural-language Query</h2>
        <p style={{ color: 'var(--text-secondary)', marginBottom: '1rem' }}>
          Ask a question in natural language. The AI generates a LogicalQuery, the backend compiles it to SQL.
        </p>
        <div style={{ display: 'flex', gap: '1rem', alignItems: 'flex-start' }}>
          <div className="form-group" style={{ flex: 1 }}>
            <label htmlFor="ai-datasource">Datasource</label>
            <select id="ai-datasource" name="datasource" value={datasourceId} onChange={(e) => setDatasourceId(e.target.value)}>
              <option value="">— select —</option>
              {datasources.map((d) => (
                <option key={d.id} value={d.id}>{d.name}</option>
              ))}
            </select>
          </div>
          <div className="form-group" style={{ flex: 1 }}>
            <label htmlFor="ai-table-search">Tables / Semantic Scope</label>
            <input
              id="ai-table-search"
              name="table_search"
              value={tableSearch}
              onChange={(e) => setTableSearch(e.target.value)}
              placeholder="Search tables…"
              disabled={!datasourceId || tables.length === 0}
              autoComplete="off"
            />
            <select
              aria-label="Selected tables"
              name="tables"
              multiple
              value={selectedTables}
              onChange={(e) => setSelectedTables(Array.from(e.target.selectedOptions, (option) => option.value))}
              disabled={!datasourceId || tables.length === 0}
              size={Math.min(6, Math.max(3, filteredTables.length || 3))}
              style={{ marginTop: '0.5rem' }}
            >
              {filteredTables.map((table) => {
                const label = tableLabel(table)
                return (
                  <option key={label} value={label}>
                    {label}
                  </option>
                )
              })}
            </select>
            <small style={{ color: 'var(--text-secondary)' }}>
              Leave empty to auto-detect relevant tables.
              {datasourceId && tables.length === 0 ? ' Sync metadata to load tables.' : ''}
            </small>
          </div>
        </div>
        <div className="form-group">
          <label htmlFor="ai-question">Your Question</label>
          <textarea
            id="ai-question"
            name="question"
            value={question}
            onChange={(e) => setQuestion(e.target.value)}
            placeholder="Show total revenue by country for January 2026…"
            rows={3}
            autoComplete="off"
          />
        </div>
        <div style={{ display: 'flex', gap: '0.5rem' }}>
          <button className="btn" onClick={runAIQuery} disabled={loading || !question || !datasourceId}>
            {loading ? 'Thinking…' : 'Preview SQL'}
          </button>
          <button className="btn" onClick={runAndExecute} disabled={loading || !question || !datasourceId}>
            {loading ? 'Running…' : 'Preview & Execute'}
          </button>
        </div>
        {error && <div className="error" style={{ marginTop: '1rem' }}>{error}</div>}
      </div>

      {result && (
        <div className="card">
          <h2>AI Response</h2>
          {result.confidence !== undefined && (
            <p style={{ marginBottom: '1rem' }}>
              Confidence:{' '}
              <span className={result.confidence > 0.7 ? 'success' : 'error'}>
                {(result.confidence * 100).toFixed(0)}%
              </span>
            </p>
          )}

          {result.table_routing && (
            <div style={{ marginBottom: '1rem' }}>
              <h3>Table Routing</h3>
              <p style={{ color: 'var(--text-secondary)' }}>
                Selected:{' '}
                {result.table_routing.selected_tables?.length
                  ? result.table_routing.selected_tables.join(', ')
                  : 'needs clarification'}
                {' '}· Confidence: {((result.table_routing.confidence || 0) * 100).toFixed(0)}%
              </p>
              {result.table_routing.candidates?.length > 0 && (
                <p style={{ color: 'var(--text-secondary)' }}>
                  Candidates: {result.table_routing.candidates.map((c: any) => c.table).join(', ')}
                </p>
              )}
            </div>
          )}

          {result.logical_query && (
            <>
              <h3>Generated LogicalQuery</h3>
              <div className="sql-preview">{JSON.stringify(result.logical_query, null, 2)}</div>
            </>
          )}

          {result.sql && (
            <>
              <h3 style={{ marginTop: '1rem' }}>Compiled SQL</h3>
              <div className="sql-preview">{result.sql}</div>
            </>
          )}

          {result.warnings && result.warnings.length > 0 && (
            <div className="error" style={{ marginTop: '1rem' }}>
              <strong>Warnings:</strong>
              <ul>
                {result.warnings.map((w: string, i: number) => (
                  <li key={i}>{w}</li>
                ))}
              </ul>
            </div>
          )}

          {result.result && result.result.columns && (
            <>
              <h3 style={{ marginTop: '1rem' }}>Results ({result.result.stats?.row_count ?? 0} rows)</h3>
              <table className="results-table">
                <thead>
                  <tr>
                    {result.result.columns.map((c: any) => (
                      <th key={c.name}>{c.name}</th>
                    ))}
                  </tr>
                </thead>
                <tbody>
                  {result.result.rows.map((row: any[], i: number) => (
                    <tr key={i}>
                      {row.map((cell, j) => <td key={j}>{String(cell)}</td>)}
                    </tr>
                  ))}
                </tbody>
              </table>
            </>
          )}
        </div>
      )}
    </div>
  )
}
