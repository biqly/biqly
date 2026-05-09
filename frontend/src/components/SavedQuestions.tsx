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

// Demo saved questions - connects to API when backend is available
const demoQuestions: SavedQuestion[] = [
  {
    id: '1',
    name: 'Revenue by Country',
    description: 'Monthly revenue breakdown by country',
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
    name: 'Active Users This Week',
    description: 'Count of active users by day',
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
    name: 'Product Sales Top 10',
    description: 'Top 10 products by sales count',
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

  const filtered = questions.filter(
    (q) =>
      q.name.toLowerCase().includes(search.toLowerCase()) ||
      q.tags.some((t) => t.toLowerCase().includes(search.toLowerCase()))
  )

  return (
    <div>
      <div className="card">
        <h2>Saved Questions</h2>
        <div className="form-group">
          <input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search questions or tags..."
          />
        </div>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: '1fr 2fr', gap: '1.5rem' }}>
        <div className="card">
          {filtered.map((q) => (
            <div
              key={q.id}
              className="saved-question-item"
              style={{ borderRadius: '0.5rem', marginBottom: '0.5rem' }}
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
            </div>
          ))}
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
              <button className="btn">Run Query</button>
              <button className="btn" style={{ background: 'var(--bg-card)', color: 'var(--text-secondary)' }}>
                Edit
              </button>
              <button className="btn" style={{ background: 'var(--error)' }}>Delete</button>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
