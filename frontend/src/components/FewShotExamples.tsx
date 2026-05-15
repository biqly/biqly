import { useEffect, useState } from 'react'
import { useApi } from '../hooks/useApi'
import { useT } from '../i18n'
import { EmptyState } from './ui/EmptyState'
import { ErrorAlert } from './ui/ErrorAlert'
import { Select } from './ui/Select'

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
  const t = useT()
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
  }, [])

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
    try { lq = JSON.parse(formLq) } catch { setFormError(t('few_shot.err_invalid_lq')); return }
    if (!formQuestion.trim()) { setFormError(t('few_shot.err_question_required')); return }

    if (editId) {
      // Update
      const updated = examples.map((e) => e.id === editId ? { ...e, question: formQuestion, logical_query: lq, tags: formTags.split(',').map((tok) => tok.trim()).filter(Boolean), dialect: formDialect } : e)
      if (apiReady) {
        await putData(`/api/ai/examples/${editId}`, updated.find((e) => e.id === editId))
      }
      persist(updated)
    } else {
      const newEx: FewShotExample = {
        id: `ex_${Date.now()}`,
        question: formQuestion,
        logical_query: lq,
        tags: formTags.split(',').map((tok) => tok.trim()).filter(Boolean),
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
    <div className="page-stack">
      {!apiReady && (
        <ErrorAlert error={t('few_shot.api_offline_alert')} />
      )}

      <div className="card">
        <div className="card-header-row">
          <h2>{t('few_shot.title')}</h2>
          <button type="button" className="btn btn-sm" onClick={openAdd}>{t('few_shot.new')}</button>
        </div>
        <p className="card-subtitle">{t('few_shot.manage_hint')}</p>

        {examples.length === 0 && <EmptyState description={t('few_shot.empty')} />}

        {examples.length > 0 && (
          <table className="results-table">
            <thead>
              <tr>
                <th>{t('few_shot.col_question')}</th>
                <th>{t('few_shot.col_dialect')}</th>
                <th>{t('few_shot.col_tags')}</th>
                <th className="actions">{t('few_shot.col_actions')}</th>
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
                      <button type="button" className="btn btn-sm btn-ghost" onClick={() => openEdit(ex)}>{t('common.edit')}</button>
                      <button type="button" className="btn btn-sm btn-danger" onClick={() => handleDelete(ex.id)}>{t('common.delete')}</button>
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
              <h2>{editId ? t('few_shot.form_edit_title') : t('few_shot.form_add_title')}</h2>
              <button className="modal-close" onClick={resetForm}>×</button>
            </div>
            <div className="modal-body">
              <div className="form-group">
                <label htmlFor="fs-question">{t('few_shot.label_nl_question')}</label>
                <textarea id="fs-question" value={formQuestion} onChange={(e) => setFormQuestion(e.target.value)} placeholder={t('few_shot.placeholder_nl_question')} rows={3} />
              </div>
              <div className="form-group">
                <label htmlFor="fs-lq">{t('few_shot.label_lq_json')}</label>
                <textarea id="fs-lq" value={formLq} onChange={(e) => setFormLq(e.target.value)} placeholder='{"select": [{"type": "metric", "name": "revenue"}]}' rows={6} style={{ fontFamily: 'monospace', fontSize: '0.8rem' }} />
              </div>
              <div className="modal-form-row">
                <div className="form-group">
                  <label htmlFor="fs-tags">{t('few_shot.label_tags')}</label>
                  <input id="fs-tags" value={formTags} onChange={(e) => setFormTags(e.target.value)} placeholder={t('few_shot.placeholder_tags')} />
                </div>
                <div className="form-group">
                  <label htmlFor="fs-dialect">{t('few_shot.label_dialect')}</label>
                  <Select
                    id="fs-dialect"
                    value={formDialect}
                    onChange={setFormDialect}
                    options={DIALECTS.map((d) => ({ value: d, label: d }))}
                  />
                </div>
              </div>
              <ErrorAlert error={formError} />
            </div>
            <div className="modal-actions">
              <button type="button" className="btn btn-ghost" onClick={resetForm}>{t('common.cancel')}</button>
              <button type="button" className="btn btn-primary" onClick={handleSave} disabled={loading}>{loading ? t('common.saving') : t('common.save')}</button>
            </div>
          </div>
        </div>
      )}
    </div>
  )
}
