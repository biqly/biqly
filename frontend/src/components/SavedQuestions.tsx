import { useState } from 'react'

interface SavedQuestion {
  id: string
  name: string
  description: string
  datasource_id: string
  model_id: string
  logical_query: any
  created_at: string
  tags: string[]
}

interface FewShotEntry {
  questionId: string
  savedAt: string
}

const FEWSHOT_STORAGE_KEY = 'biqly_saved_fewshot'

function loadFewShotEntries(): FewShotEntry[] {
  try {
    return JSON.parse(localStorage.getItem(FEWSHOT_STORAGE_KEY) || '[]')
  } catch { return [] }
}

function saveFewShotEntries(entries: FewShotEntry[]) {
  localStorage.setItem(FEWSHOT_STORAGE_KEY, JSON.stringify(entries))
}

// Demo saved questions - connects to API when backend is available
const demoQuestions: SavedQuestion[] = [
  {
    id: '1',
    name: 'Ülkeye göre gelir',
    description: 'Ülkelere göre aylık gelir dağılımı',
    datasource_id: 'ds_1',
    model_id: 'orders',
    logical_query: {
      select: [{ type: 'dimension', name: 'country' }, { type: 'metric', name: 'revenue' }],
      group_by: [{ field: 'country' }],
      order_by: [{ field: 'revenue', direction: 'desc' }],
      limit: 100,
    },
    created_at: '2026-05-01',
    tags: ['revenue', 'country'],
  },
  {
    id: '2',
    name: 'Bu hafta aktif kullanıcılar',
    description: 'Güne göre aktif kullanıcı sayısı',
    datasource_id: 'ds_1',
    model_id: 'users',
    logical_query: {
      select: [{ type: 'dimension', name: 'date' }, { type: 'metric', name: 'user_count' }],
      group_by: [{ field: 'date' }],
      limit: 7,
    },
    created_at: '2026-05-05',
    tags: ['users', 'weekly'],
  },
  {
    id: '3',
    name: 'En çok satan 10 ürün',
    description: 'Satış adedine göre ilk 10 ürün',
    datasource_id: 'ds_1',
    model_id: 'products',
    logical_query: {
      select: [{ type: 'dimension', name: 'name' }, { type: 'metric', name: 'sales_count' }],
      group_by: [{ field: 'name' }],
      order_by: [{ field: 'sales_count', direction: 'desc' }],
      limit: 10,
    },
    created_at: '2026-05-08',
    tags: ['products', 'sales'],
  },
]

export default function SavedQuestions() {
  const [questions] = useState<SavedQuestion[]>(demoQuestions)
  const [search, setSearch] = useState('')
  const [selectedQuestion, setSelectedQuestion] = useState<SavedQuestion | null>(null)
  const [fewShotEntries, setFewShotEntries] = useState<FewShotEntry[]>(loadFewShotEntries)

  const filtered = questions.filter(
    (q) =>
      q.name.toLowerCase().includes(search.toLowerCase()) ||
      q.tags.some((t) => t.toLowerCase().includes(search.toLowerCase()))
  )

  const isFewShot = (id: string) => fewShotEntries.some((e) => e.questionId === id)

  const toggleFewShot = (id: string) => {
    const exists = isFewShot(id)
    const updated = exists
      ? fewShotEntries.filter((e) => e.questionId !== id)
      : [...fewShotEntries, { questionId: id, savedAt: new Date().toISOString() }]
    setFewShotEntries(updated)
    saveFewShotEntries(updated)
  }

  const fewShotCount = fewShotEntries.length

  return (
    <div>
      <div className="card">
        <h2>Soru kütüphanesi</h2>
        <p style={{ color: 'var(--text-secondary)', fontSize: '0.82rem', marginBottom: '0.75rem' }}>
          {fewShotCount > 0
            ? `${fewShotCount} soru AI few-shot örneği olarak işaretlendi.`
            : 'Metinden-SQL doğruluğunu artırmak için soruları AI few-shot örneği olarak işaretleyin.'}
        </p>
        <div className="form-group">
          <input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Soru veya etiket ara…"
            aria-label="Kayıtlı sorularda ara"
            name="saved_question_search"
            autoComplete="off"
          />
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 2fr', gap: '1.5rem' }}>
        <div className="card">
          {filtered.map((q) => {
            const checked = isFewShot(q.id)
            return (
              <div key={q.id} style={{ position: 'relative' }}>
                <button
                  className="saved-question-item"
                  onClick={() => setSelectedQuestion(q)}
                >
                  <h3>{q.name}</h3>
                  <p>{q.description}</p>
                  <div style={{ display: 'flex', gap: '0.25rem', marginTop: '0.5rem' }}>
                    {q.tags.map((tag) => (
                      <span
                        key={tag}
                        style={{
                          background: 'var(--bg-card)',
                          padding: '0.125rem 0.5rem',
                          borderRadius: '0.25rem',
                          fontSize: '0.75rem',
                          color: 'var(--text-secondary)',
                        }}
                      >
                        {tag}
                      </span>
                    ))}
                  </div>
                </button>
                <label
                  className="fewshot-checkbox"
                  title="AI few-shot örneği olarak kullan"
                  style={{
                    position: 'absolute',
                    top: '0.6rem',
                    right: '0.6rem',
                    display: 'flex',
                    alignItems: 'center',
                    gap: '0.3rem',
                    fontSize: '0.72rem',
                    color: checked ? 'var(--accent)' : 'var(--text-muted)',
                    cursor: 'pointer',
                    background: checked ? 'rgba(96,165,250,0.1)' : 'transparent',
                    padding: '0.15rem 0.4rem',
                    borderRadius: '0.3rem',
                  }}
                >
                  <input
                    type="checkbox"
                    checked={checked}
                    onChange={() => toggleFewShot(q.id)}
                    onClick={(e) => e.stopPropagation()}
                  />
                  <span>AI örneği</span>
                </label>
              </div>
            )
          })}
        </div>

        {selectedQuestion && (
          <div className="card">
            <h2>{selectedQuestion.name}</h2>
            <p style={{ color: 'var(--text-secondary)', marginBottom: '1rem' }}>
              {selectedQuestion.description}
            </p>
            <h3>LogicalQuery</h3>
            <div className="sql-preview">
              {JSON.stringify(selectedQuestion.logical_query, null, 2)}
            </div>
            <div style={{ marginTop: '1rem', display: 'flex', gap: '0.5rem' }}>
              <button className="btn">Sorguyu çalıştır</button>
              <button className="btn" style={{ background: 'var(--bg-card)', color: 'var(--text-secondary)' }}>
                Düzenle
              </button>
              <button className="btn" style={{ background: 'var(--error)' }}>Sil</button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
