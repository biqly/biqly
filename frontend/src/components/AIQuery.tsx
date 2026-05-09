import { useEffect, useState } from 'react'
import { useApi } from '../hooks/useApi'

interface Datasource {
  id: string
  name: string
  type: string
}

export default function AIQuery() {
  const { get, postData, loading, error } = useApi()
  const [datasources, setDatasources] = useState<Datasource[]>([])
  const [datasourceId, setDatasourceId] = useState('')
  const [modelId, setModelId] = useState('')
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

  const runAIQuery = async () => {
    const res = await postData('/api/ai/query/preview', {
      datasource_id: datasourceId,
      model_id: modelId,
      question,
    })
    if (res) setResult(res)
  }

  const runAndExecute = async () => {
    const res = await postData('/api/ai/query/run', {
      datasource_id: datasourceId,
      model_id: modelId,
      question,
    })
    if (res) setResult(res)
  }

  return (
    <div>
      <div className="card">
        <h2>🤖 AI Query</h2>
        <p style={{ color: 'var(--text-secondary)', marginBottom: '1rem' }}>
          Ask a question in natural language. The AI generates a LogicalQuery, the backend compiles it to SQL.
        </p>
        <div style={{ display: 'flex', gap: '1rem' }}>
          <div className="form-group" style={{ flex: 1 }}>
            <label>Datasource</label>
            <select value={datasourceId} onChange={(e) => setDatasourceId(e.target.value)}>
              <option value="">— select —</option>
              {datasources.map((d) => (
                <option key={d.id} value={d.id}>{d.name}</option>
              ))}
            </select>
          </div>
          <div className="form-group" style={{ flex: 1 }}>
            <label>Semantic Model</label>
            <input value={modelId} onChange={(e) => setModelId(e.target.value)} placeholder="e.g. orders" />
          </div>
        </div>
        <div className="form-group">
          <label>Your Question</label>
          <textarea
            value={question}
            onChange={(e) => setQuestion(e.target.value)}
            placeholder="Show total revenue by country for January 2026"
            rows={3}
          />
        </div>
        <div style={{ display: 'flex', gap: '0.5rem' }}>
          <button className="btn" onClick={runAIQuery} disabled={loading || !question || !datasourceId}>
            {loading ? 'Thinking...' : 'Preview SQL'}
          </button>
          <button className="btn" onClick={runAndExecute} disabled={loading || !question || !datasourceId}>
            {loading ? 'Running...' : 'Preview & Execute'}
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
