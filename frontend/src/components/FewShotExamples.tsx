import { useEffect, useState } from 'react'
import { useApi } from '../hooks/useApi'

interface FewShotExample {
  id: string
  question: string
  logical_query: Record<string, unknown>
  tags: string[]
  dialect: string
  created_at?: string
}

const DIALECTS = ['postgresql', 'mysql', 'bigquery', 'snowflake', 'duckdb']

export default function FewShotExamples() {
  const { get, postData, putData, deleteData, loading, error } = useApi()
  const [examples, setExamples] = useState<FewShotExample[]>([])
  const [showForm, setShowForm] = useState(false)
  const [editId, setEditId] = useState<string | null>(null)
  const [formQuestion, setFormQuestion] = useState('')
  const [formLq, setFormLq] = useState('')
  const [formTags, setFormTags] = useState('')
  const [formDialect, setFormDialect] = useState('postgresql')
  const [formError, setFormError] = useState<string | null>(null)
  const [apiReady, setApiReady] = useState(true)

  useEffect(() => {
    get<FewShotExample[]>('/api/ai/examples').then((data) => {
      if (data) {
        setExamples(data)
        setApiReady(true)
      } else {
        // Endpoint not ready yet — load from localStorage
        try {
          const local = JSON.parse(localStorage.getItem('biqly_fewshot') || '[]')
          setExamples(local)
        } catch { /* empty */ }
        setApiReady(false)
      }
    })
  }, []) // eslint-disable-line react-hooks/exhaustive-deps

  const persist = (updated: FewShotExample[]) => {
    setExamples(updated)
    if (!apiReady) {
      localStorage.setItem('biqly_fewshot', JSON.stringify(updated))
    }
  }

  const resetForm = () => {
    setFormQuestion(''); setFormLq(''); setFormTags(''); setFormDialect('postgresql'); setEditId(null); setFormError(null); setShowForm(false)
  }

  const openAdd = () => { setFormQuestion(''); setFormLq(''); setFormTags(''); setFormDialect('postgresql'); setEditId(null); setFormError(null); setShowForm(true) }

  const openEdit = (ex: FewShotExample) => {
    setEditId(ex.id); setFormQuestion(ex.question); setFormLq(JSON.stringify(ex.logical_query, null, 2)); setFormTags(ex.tags.join(', ')); setFormDialect(ex.dialect); setShowForm(true)
  }

  const handleSave = async () => {
    setFormError(null)
    let lq: Record<string, unknown>
    try { lq = JSON.parse(formLq) } catch { setFormError('Invalid JSON in LogicalQuery'); return }
    if (!formQuestion.trim()) { setFormError('Question is required'); return }

    if (editId) {
      // Update
      const updated = examples.map((e) => e.id === editId ? { ...e, question: formQuestion, logical_query: lq, tags: formTags.split(',').map((t) => t.trim()).filter(Boolean), dialect: formDialect } : e)
      if (apiReady) {
        await putData(`/api/ai/examples/${editId}`, updated.find((e) => e.id === editId))
      }
      persist(updated)
    } else {
      const newEx: FewShotExample = {
        id: `ex_${Date.now()}`,
        question: formQuestion,
        logical_query: lq,
        tags: formTags.split(',').map((t) => t.trim()).filter(Boolean),
        dialect: formDialect,
      }
      if (apiReady) {
        const res = await postData<FewShotExample>('/api/ai/examples', newEx)
        if (res) { persist([...examples, res]); resetForm(); return }
        setApiReady(false)
      }
      persist([...examples, newEx])
    }
    resetForm()
  }

  const handleDelete = async (id: string) => {
    if (apiReady) {
      await deleteData(`/api/ai/examples/${id}`)
    }
    persist(examples.filter((e) => e.id !== id))
  }

  return (
    <div>
      {!apiReady && (
        <div className="error" style={{ marginBottom: '1rem' }}>
          Backend endpoint not ready yet. Examples are stored locally in your browser.
        </div>
      )}

      <div className="card">
        <div className="card-header-row">
          <h2>Few-Shot Examples</h2>
          <button className="btn btn-sm" onClick={openAdd}>+ Add Example</button>
        </div>
        <p style={{ color: 'var(--text-secondary)', fontSize: '0.85rem', marginBottom: '1rem' }}>
          Manage examples used as few-shot prompts for the AI text-to-SQL engine. Each example teaches the AI a pattern.
        </p>

        {examples.length === 0 && (
          <p style={{ color: 'var(--text-muted)' }}>No examples yet. Add one to improve AI query generation.</p>
        )}

        {examples.length > 0 && (
          <table className="results-table">
            <thead>
              <tr>
                <th>Question</th>
                <th>Dialect</th>
                <th>Tags</th>
                <th className="actions">Actions</th>
              </tr>
            </thead>
            <tbody>
              {examples.map((ex) => (
                <tr key={ex.id}>
                  <td style={{ maxWidth: 350, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={ex.question}>
                    {ex.question}
                  </td>
                  <td><code style={{ fontSize: '0.78rem', color: 'var(--accent)' }}>{ex.dialect}</code></td>
                  <td>
                    {ex.tags.map((tag) => (
                      <span key={tag} style={{ display: 'inline-block', padding: '0.15rem 0.5rem', background: 'rgba(96,165,250,0.1)', borderRadius: '0.3rem', fontSize: '0.72rem', marginRight: '0.3rem', color: 'var(--accent)' }}>
                        {tag}
                      </span>
                    ))}
                  </td>
                  <td className="actions">
                    <div className="row-actions">
                      <button className="btn btn-sm btn-ghost" onClick={() => openEdit(ex)}>Edit</button>
                      <button className="btn btn-sm btn-danger" onClick={() => handleDelete(ex.id)}>Delete</button>
                    </div>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>

      {/* Form Modal */}
      {showForm && (
        <div className="modal-backdrop" onClick={resetForm}>
          <div className="modal-card" onClick={(e) => e.stopPropagation()}>
            <div className="modal-header">
              <h2>{editId ? 'Edit Example' : 'Add Few-Shot Example'}</h2>
              <button className="modal-close" onClick={resetForm}>×</button>
            </div>
            <div className="modal-body">
              <div className="form-group">
                <label htmlFor="fs-question">Natural Language Question</label>
                <textarea id="fs-question" value={formQuestion} onChange={(e) => setFormQuestion(e.target.value)} placeholder="Show total revenue by country for last month…" rows={3} />
              </div>
              <div className="form-group">
                <label htmlFor="fs-lq">LogicalQuery (JSON)</label>
                <textarea id="fs-lq" value={formLq} onChange={(e) => setFormLq(e.target.value)} placeholder='{"select": [{"type": "metric", "name": "revenue"}]}' rows={6} style={{ fontFamily: 'monospace', fontSize: '0.8rem' }} />
              </div>
              <div className="modal-form-row">
                <div className="form-group">
                  <label htmlFor="fs-tags">Tags (comma-separated)</label>
                  <input id="fs-tags" value={formTags} onChange={(e) => setFormTags(e.target.value)} placeholder="aggregation, revenue, date-filter" />
                </div>
                <div className="form-group">
                  <label htmlFor="fs-dialect">SQL Dialect</label>
                  <select id="fs-dialect" value={formDialect} onChange={(e) => setFormDialect(e.target.value)}>
                    {DIALECTS.map((d) => <option key={d} value={d}>{d}</option>)}
                  </select>
                </div>
              </div>
              {formError && <div className="error">{formError}</div>}
            </div>
            <div className="modal-actions">
              <button className="btn btn-ghost" onClick={resetForm}>Cancel</button>
              <button className="btn btn-primary" onClick={handleSave} disabled={loading}>{loading ? 'Saving…' : 'Save'}</button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
